// Package testitems owns the sample inspection test-item dictionary.
package testitems

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/0xunknownz/saas.starter/foundation/audit"
	"github.com/0xunknownz/saas.starter/foundation/db"
	"github.com/0xunknownz/saas.starter/foundation/identity"
	"github.com/jackc/pgx/v5"
)

type Config struct{}

type Deps struct {
	Audit    audit.Writer
	Db       db.Transactor
	Identity identity.Directory // HTTP authorization is provided through HTTPDeps.
}

type Item struct {
	ID          string `json:"id"`
	Code        string `json:"code"`
	Name        string `json:"name"`
	Category    string `json:"category"`
	Method      string `json:"method"`
	Description string `json:"description"`
	Revision    int64  `json:"revision"`
}

type UpdateInput struct {
	Name             string `json:"name"`
	Category         string `json:"category"`
	Method           string `json:"method"`
	Description      string `json:"description"`
	ExpectedRevision int64  `json:"expectedRevision"`
}

func (in UpdateInput) validate() (UpdateInput, error) {
	in.Name, in.Category, in.Method = strings.TrimSpace(in.Name), strings.TrimSpace(in.Category), strings.TrimSpace(in.Method)
	in.Description = strings.TrimSpace(in.Description)
	for _, field := range []struct {
		name, value string
		max         int
	}{
		{"name", in.Name, 120}, {"category", in.Category, 80}, {"method", in.Method, 120},
	} {
		if n := utf8.RuneCountInString(field.value); n == 0 || n > field.max {
			return in, fmt.Errorf("%w: %s must contain 1-%d characters", ErrInvalidInput, field.name, field.max)
		}
	}
	if utf8.RuneCountInString(in.Description) > 1000 {
		return in, fmt.Errorf("%w: description must contain at most 1000 characters", ErrInvalidInput)
	}
	if in.ExpectedRevision < 1 {
		return in, fmt.Errorf("%w: expectedRevision must be positive", ErrInvalidInput)
	}
	return in, nil
}

type itemStore interface {
	List(context.Context, string, int) ([]Item, int, error)
	Get(context.Context, string) (Item, error)
	Update(context.Context, string, string, UpdateInput) (Item, error)
}

type Service struct{ store itemStore }

func NewService(_ Config, deps Deps) (*Service, error) {
	return &Service{store: databaseStore{db: deps.Db, audit: deps.Audit}}, nil
}

type databaseStore struct {
	db    db.Transactor
	audit audit.Writer
}

func (d databaseStore) List(ctx context.Context, q string, limit int) (items []Item, total int, err error) {
	err = d.db.InTx(ctx, db.TxOptions{ReadOnly: true}, func(ctx context.Context, tx db.DBTX) error {
		if e := tx.QueryRow(ctx, `SELECT count(*) FROM product_test_items
			WHERE ($1 = '' OR code ILIKE '%' || $1 || '%' OR name ILIKE '%' || $1 || '%' OR category ILIKE '%' || $1 || '%')`, q).Scan(&total); e != nil {
			return e
		}
		rows, e := tx.Query(ctx, `SELECT id, code, name, category, method, description, revision FROM product_test_items
			WHERE ($1 = '' OR code ILIKE '%' || $1 || '%' OR name ILIKE '%' || $1 || '%' OR category ILIKE '%' || $1 || '%')
			ORDER BY code LIMIT $2`, q, limit)
		if e != nil {
			return e
		}
		defer rows.Close()
		items = make([]Item, 0)
		for rows.Next() {
			var item Item
			if e := rows.Scan(&item.ID, &item.Code, &item.Name, &item.Category, &item.Method, &item.Description, &item.Revision); e != nil {
				return e
			}
			items = append(items, item)
		}
		return rows.Err()
	})
	return
}

func (d databaseStore) Get(ctx context.Context, id string) (item Item, err error) {
	err = d.db.InTx(ctx, db.TxOptions{ReadOnly: true}, func(ctx context.Context, tx db.DBTX) error {
		e := tx.QueryRow(ctx, `SELECT id, code, name, category, method, description, revision
			FROM product_test_items WHERE id = $1`, id).Scan(
			&item.ID, &item.Code, &item.Name, &item.Category, &item.Method, &item.Description, &item.Revision,
		)
		if errors.Is(e, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return e
	})
	return
}

func (d databaseStore) Update(ctx context.Context, actor, id string, in UpdateInput) (item Item, err error) {
	err = d.db.InTx(ctx, db.TxOptions{}, func(ctx context.Context, tx db.DBTX) error {
		e := tx.QueryRow(ctx, `UPDATE product_test_items
			SET name = $3, category = $4, method = $5, description = $6, revision = revision + 1
			WHERE id = $1 AND revision = $2
			RETURNING id, code, name, category, method, description, revision`,
			id, in.ExpectedRevision, in.Name, in.Category, in.Method, in.Description,
		).Scan(&item.ID, &item.Code, &item.Name, &item.Category, &item.Method, &item.Description, &item.Revision)
		if errors.Is(e, pgx.ErrNoRows) {
			var exists bool
			if e = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM product_test_items WHERE id = $1)`, id).Scan(&exists); e != nil {
				return e
			}
			if !exists {
				return ErrNotFound
			}
			return ErrConflict
		}
		if e != nil {
			return e
		}
		return d.audit.WriteTx(ctx, tx, audit.Entry{Realm: "platform", Actor: actor, Action: "product.testitem.update",
			Target: "test-item:" + id, Metadata: map[string]any{"revision": item.Revision}})
	})
	return
}

var (
	ErrNotFound     = errors.New("test item not found")
	ErrConflict     = errors.New("test item has changed; reload before editing")
	ErrInvalidInput = errors.New("invalid test item")
)

const (
	PermissionRead   = "platform.product.testitems.read"
	PermissionManage = "platform.product.testitems.manage"
)

func (s *Service) List(ctx context.Context, q string, limit int) ([]Item, int, error) {
	return s.store.List(ctx, q, limit)
}
func (s *Service) Get(ctx context.Context, id string) (Item, error) { return s.store.Get(ctx, id) }
func (s *Service) Update(ctx context.Context, actor, id string, in UpdateInput) (Item, error) {
	in, err := in.validate()
	if err != nil {
		return Item{}, err
	}
	return s.store.Update(ctx, actor, id, in)
}

// HTTPDeps provides the same full platform-session and permission guard as Foundation Admin APIs.
type HTTPDeps struct{ Identity identity.PlatformGuard }
