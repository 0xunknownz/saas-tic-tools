package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadPlanRejectsTampering(t *testing.T) {
	m := migration{Module: "foundation.audit", Version: "20261105000000", Name: "audit", SQL: "CREATE TABLE example(id int);"}
	sum := sha256.Sum256([]byte(m.SQL))
	m.Checksum = hex.EncodeToString(sum[:])
	h := sha256.New()
	fmt.Fprintf(h, "%s@%s:%s\n", m.Module, m.Version, m.Checksum)
	original := plan{SchemaVersion: 1, Product: "example", Digest: hex.EncodeToString(h.Sum(nil)), Migrations: []migration{m}}
	path := filepath.Join(t.TempDir(), "plan.json")
	write := func(p plan) {
		t.Helper()
		b, err := json.Marshal(p)
		if err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(path, b, 0600); err != nil {
			t.Fatal(err)
		}
	}
	write(original)
	if _, err := loadPlan(path); err != nil {
		t.Fatalf("valid: %v", err)
	}
	for _, tc := range []struct {
		name   string
		change func(*plan)
		want   string
	}{
		{"sql", func(p *plan) { p.Migrations[0].SQL += " SELECT 1;" }, "checksum"},
		{"transaction", func(p *plan) { p.Migrations[0].NoTransaction = true }, "transaction mode"},
		{"digest", func(p *plan) { p.Digest = strings.Repeat("0", 64) }, "digest mismatch"},
		{"removed", func(p *plan) { p.Migrations = nil }, "digest mismatch"},
		{"duplicate", func(p *plan) { p.Migrations = append(p.Migrations, p.Migrations[0]) }, "duplicate"},
		{"schema", func(p *plan) { p.SchemaVersion = 2 }, "schema"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := original
			p.Migrations = append([]migration(nil), original.Migrations...)
			tc.change(&p)
			write(p)
			if _, err := loadPlan(path); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("got %v, want %s", err, tc.want)
			}
		})
	}
}
