// Command migrate applies the deployment migration plan written by "go tool saasctl build".
//
// The generated Product does not vendor the saasctl CLI, so the test-environment image builds this
// small runner instead. It applies the same ordered plan, uses the same saasctl_migrations history
// table, the same file checksums and the same database advisory lock as "go tool saasctl db migrate"; the
// two are interoperable and parallel runs cannot apply a migration twice.
package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"hash/fnv"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

const table = "saasctl_migrations"

const createTable = "CREATE TABLE IF NOT EXISTS " + table + " (" +
	"module_id text NOT NULL, migration_version text NOT NULL, checksum text NOT NULL, " +
	"applied_at timestamptz NOT NULL DEFAULT now(), execution_duration_ms bigint NOT NULL, " +
	"PRIMARY KEY (module_id, migration_version))"

var lockKey = func() int64 {
	h := fnv.New64a()
	h.Write([]byte("saasctl:migration-lock"))
	return int64(h.Sum64())
}()

type plan struct {
	SchemaVersion int         `json:"schemaVersion"`
	Product       string      `json:"product"`
	Digest        string      `json:"digest"`
	Migrations    []migration `json:"migrations"`
}

type migration struct {
	Module        string `json:"module"`
	Version       string `json:"version"`
	Name          string `json:"name"`
	Checksum      string `json:"checksum"`
	NoTransaction bool   `json:"noTransaction"`
	SQL           string `json:"sql"`
}

func (m migration) id() string { return m.Module + "@" + m.Version }

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "error: MIGRATE_FAILED:", err)
		os.Exit(1)
	}
}

func run() error {
	planPath := flag.String("plan", env("PLAN_PATH", "/app/migrations/plan.json"), "migration plan written by go tool saasctl build")
	lockTimeout := flag.Duration("lock-timeout", envDuration("MIGRATION_LOCK_TIMEOUT", 30*time.Second), "how long to wait for the database migration lock")
	flag.Parse()

	url := os.Getenv("DATABASE_URL")
	if strings.TrimSpace(url) == "" {
		return errors.New("DATABASE_URL is required")
	}
	p, err := loadPlan(*planPath)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()
	c, err := pgx.Connect(ctx, url)
	if err != nil {
		return fmt.Errorf("cannot connect to the database: %s", redact(err))
	}
	defer c.Close(context.Background())

	if err := lock(ctx, c, *lockTimeout); err != nil {
		return err
	}
	defer func() { _, _ = c.Exec(context.Background(), "SELECT pg_advisory_unlock($1)", lockKey) }()

	if _, err := c.Exec(ctx, createTable); err != nil {
		return fmt.Errorf("creating %s: %s", table, redact(err))
	}
	hist, err := history(ctx, c)
	if err != nil {
		return fmt.Errorf("reading migration history: %s", redact(err))
	}
	pending, err := planPending(p.Migrations, hist)
	if err != nil {
		return err
	}
	if len(pending) == 0 {
		fmt.Println("database is up to date (plan", short(p.Digest)+")")
		return nil
	}
	fmt.Printf("%d pending migration(s) from plan %s\n", len(pending), short(p.Digest))
	for _, m := range pending {
		fmt.Printf("applying %s %s\n", m.id(), m.Name)
		if err := apply(ctx, c, m); err != nil {
			return fmt.Errorf("migration %s (%s) failed: %s; later migrations were not run", m.id(), m.Name, redact(err))
		}
	}
	fmt.Printf("applied %d migration(s)\n", len(pending))
	return nil
}

func loadPlan(path string) (plan, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return plan{}, fmt.Errorf("read migration plan: %w (build the release with go tool saasctl build first)", err)
	}
	var p plan
	if err := json.Unmarshal(b, &p); err != nil {
		return plan{}, fmt.Errorf("parse migration plan: %w", err)
	}
	if p.SchemaVersion != 1 || p.Product == "" || p.Digest == "" {
		return plan{}, errors.New("migration plan schema, product or digest is invalid; rebuild with go tool saasctl build")
	}
	h := sha256.New()
	seen := make(map[string]bool)
	for _, m := range p.Migrations {
		if m.Module == "" || m.Version == "" || m.SQL == "" {
			return plan{}, fmt.Errorf("migration plan entry %q is incomplete", m.id())
		}
		sum := sha256.Sum256([]byte(m.SQL))
		if hex.EncodeToString(sum[:]) != m.Checksum {
			return plan{}, fmt.Errorf("migration %s does not match its checksum; the plan file is corrupt", m.id())
		}
		first, _, _ := strings.Cut(m.SQL, "\n")
		if m.NoTransaction != (strings.TrimSpace(first) == "-- saasctl:no-transaction") {
			return plan{}, fmt.Errorf("migration %s transaction mode does not match its SQL marker", m.id())
		}
		if seen[m.id()] {
			return plan{}, fmt.Errorf("duplicate migration %s", m.id())
		}
		seen[m.id()] = true
		fmt.Fprintf(h, "%s@%s:%s\n", m.Module, m.Version, m.Checksum)
	}
	if hex.EncodeToString(h.Sum(nil)) != p.Digest {
		return plan{}, errors.New("migration plan digest mismatch")
	}
	return p, nil
}

type applied struct{ checksum string }

func history(ctx context.Context, c *pgx.Conn) (map[string]applied, error) {
	rows, err := c.Query(ctx, "SELECT module_id, migration_version, checksum FROM "+table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	h := map[string]applied{}
	for rows.Next() {
		var m, v, s string
		if err := rows.Scan(&m, &v, &s); err != nil {
			return nil, err
		}
		h[m+"@"+v] = applied{s}
	}
	return h, rows.Err()
}

func planPending(ms []migration, h map[string]applied) ([]migration, error) {
	inSet := map[string]bool{}
	mods := map[string]bool{}
	latest := map[string]string{}
	for _, m := range ms {
		inSet[m.id()] = true
		mods[m.Module] = true
		if a, ok := h[m.id()]; ok && a.checksum == m.Checksum && m.Version > latest[m.Module] {
			latest[m.Module] = m.Version
		}
	}
	for id, a := range h {
		mod, _, _ := strings.Cut(id, "@")
		if mods[mod] && !inSet[id] {
			return nil, fmt.Errorf("migration %s was applied but its file is gone; restore it", id)
		}
		if inSet[id] {
			for _, m := range ms {
				if m.id() == id && m.Checksum != a.checksum {
					return nil, fmt.Errorf("migration %s was changed after it was applied; restore the original file and add a new migration instead", id)
				}
			}
		}
	}
	var pending []migration
	for _, m := range ms {
		if a, ok := h[m.id()]; ok {
			if a.checksum != m.Checksum {
				return nil, fmt.Errorf("migration %s was changed after it was applied; restore the original file and add a new migration instead", m.id())
			}
			continue
		}
		if v := latest[m.Module]; v > m.Version {
			return nil, fmt.Errorf("migration %s is older than the applied %s@%s; give it a newer version", m.id(), m.Module, v)
		}
		pending = append(pending, m)
	}
	return pending, nil
}

func lock(ctx context.Context, c *pgx.Conn, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		var got bool
		if err := c.QueryRow(ctx, "SELECT pg_try_advisory_lock($1)", lockKey).Scan(&got); err != nil {
			return fmt.Errorf("migration lock connection failed: %s", redact(err))
		}
		if got {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("another migration runner held the lock for %s; deployment stopped", timeout)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
}

const record = "INSERT INTO " + table + " (module_id, migration_version, checksum, execution_duration_ms) VALUES ($1,$2,$3,$4)"

func apply(ctx context.Context, c *pgx.Conn, m migration) error {
	start := time.Now()
	ms := func() int64 { return time.Since(start).Milliseconds() }
	if m.NoTransaction {
		if _, err := c.PgConn().Exec(ctx, m.SQL).ReadAll(); err != nil {
			return err
		}
		_, err := c.Exec(ctx, record, m.Module, m.Version, m.Checksum, ms())
		return err
	}
	tx, err := c.Begin(ctx)
	if err != nil {
		return err
	}
	if _, err := c.PgConn().Exec(ctx, m.SQL).ReadAll(); err != nil {
		_ = tx.Rollback(context.Background())
		return err
	}
	if c.PgConn().TxStatus() != 'T' {
		return errors.New("the migration ended its own transaction (COMMIT/ROLLBACK); history was not recorded")
	}
	if _, err := tx.Exec(ctx, record, m.Module, m.Version, m.Checksum, ms()); err != nil {
		_ = tx.Rollback(context.Background())
		return err
	}
	return tx.Commit(ctx)
}

func redact(err error) string {
	var pe *pgconn.PgError
	if errors.As(err, &pe) {
		return pe.Message
	}
	var ce *pgconn.ConnectError
	if errors.As(err, &ce) {
		return ce.Unwrap().Error()
	}
	return "connection failed"
}

func env(name, def string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return def
}

func envDuration(name string, def time.Duration) time.Duration {
	v := os.Getenv(name)
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: %s is not a duration (%q); using %s\n", name, v, def)
		return def
	}
	return d
}

func short(d string) string {
	if len(d) > 12 {
		return d[:12]
	}
	return d
}

// keep strconv imported for older toolchains that parse -lock-timeout themselves; harmless.
var _ = strconv.Itoa
