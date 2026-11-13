package testitems

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/0xunknownz/saas.starter/foundation/audit"
	"github.com/0xunknownz/saas.starter/foundation/db"
	"github.com/0xunknownz/saas.starter/foundation/identity"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type fakeReader struct {
	items []Item
	item  Item
	err   error
}

func (f fakeReader) List(context.Context, string, int) ([]Item, int, error) {
	return f.items, len(f.items), f.err
}
func (f fakeReader) Get(context.Context, string) (Item, error) { return f.item, f.err }
func (f fakeReader) Update(context.Context, string, string, UpdateInput) (Item, error) {
	return f.item, f.err
}

type fakeGuard struct { // Embed to satisfy unused PlatformGuard methods.
	identity.PlatformGuard
	allowed bool
}

func (g fakeGuard) RequirePlatform(next http.Handler) http.Handler { return next }
func (g fakeGuard) PlatformCan(context.Context, string) bool       { return g.allowed }
func (fakeGuard) GuardJSON(w http.ResponseWriter, req *http.Request, body any) bool {
	if body == nil {
		return true
	}
	if json.NewDecoder(req.Body).Decode(body) != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return false
	}
	return true
}
func testRouter(t *testing.T, allowed bool, reader itemStore) *http.ServeMux {
	t.Helper()
	svc := &Service{store: reader}
	mux := http.NewServeMux()
	if err := RegisterHTTP(mux, svc, HTTPDeps{Identity: fakeGuard{allowed: allowed}}); err != nil {
		t.Fatal(err)
	}
	return mux
}
func TestDictionaryListAndDetailAreReadOnly(t *testing.T) {
	item := Item{ID: "ti-001", Code: "WATER-PH", Name: "水质 pH 值", Category: "水质检测", Method: "电极法"}
	mux := testRouter(t, true, fakeReader{items: []Item{item}, item: item})
	for _, path := range []string{"/api/product-admin/v1/test-items", "/api/product-admin/v1/test-items/ti-001"} {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("GET %s: status %d", path, rec.Code)
		}
		var body map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/product-admin/v1/test-items", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST status %d", rec.Code)
	}
}
func TestDictionaryRequiresReadPermission(t *testing.T) {
	mux := testRouter(t, false, fakeReader{})
	for _, path := range []string{"/api/product-admin/v1/test-items", "/api/product-admin/v1/test-items/ti-001"} {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusForbidden {
			t.Fatalf("GET %s: status %d", path, rec.Code)
		}
	}
}

type recordingRouter struct{ patterns map[string]bool }

func (r recordingRouter) Handle(pattern string, _ http.Handler) { r.patterns[pattern] = true }

// The shipped OpenAPI contract must declare exactly the Product routes mounted by this plugin.
func TestDictionaryRoutesMatchOpenAPI(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "openapi", "product-testitems.json"))
	if err != nil {
		t.Fatal(err)
	}
	var contract struct {
		Paths map[string]map[string]struct {
			OperationID  string `json:"operationId"`
			Permission   string `json:"x-permission"`
			TenantScoped bool   `json:"x-tenant-scoped"`
		} `json:"paths"`
	}
	if err := json.Unmarshal(data, &contract); err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"GET /api/product-admin/v1/test-items":      "listProductTestItems",
		"GET /api/product-admin/v1/test-items/{id}": "getProductTestItem",
		"PUT /api/product-admin/v1/test-items/{id}": "updateProductTestItem",
	}
	operations := 0
	for path, methods := range contract.Paths {
		for method, operation := range methods {
			operations++
			pattern := strings.ToUpper(method) + " " + path
			permission := PermissionRead
			if method == "put" {
				permission = PermissionManage
			}
			if operation.OperationID != want[pattern] || operation.Permission != permission || operation.TenantScoped {
				t.Errorf("%s: operation=%q permission=%q tenantScoped=%t", pattern, operation.OperationID, operation.Permission, operation.TenantScoped)
			}
		}
	}
	if operations != len(want) {
		t.Fatalf("OpenAPI operations: got %d, want %d", operations, len(want))
	}
	router := recordingRouter{patterns: map[string]bool{}}
	if err := RegisterHTTP(router, &Service{}, HTTPDeps{Identity: fakeGuard{}}); err != nil {
		t.Fatal(err)
	}
	if len(router.patterns) != len(want) {
		t.Fatalf("mounted routes: got %d, want %d", len(router.patterns), len(want))
	}
	for pattern := range want {
		if !router.patterns[pattern] {
			t.Errorf("route not mounted: %s", pattern)
		}
	}
}

type editSpy struct {
	fakeReader
	calls int
	input UpdateInput
}

func (s *editSpy) Update(_ context.Context, actor, id string, in UpdateInput) (Item, error) {
	s.calls++
	s.input = in
	if actor != "account:admin" || id != "ti-001" {
		return Item{}, errors.New("wrong edit target")
	}
	return Item{ID: id, Name: in.Name, Revision: in.ExpectedRevision + 1}, nil
}
func TestUpdateValidatesBeforeWriting(t *testing.T) {
	spy := &editSpy{}
	svc := &Service{store: spy}
	_, err := svc.Update(context.Background(), "account:admin", "ti-001", UpdateInput{Name: " ", Category: "水质", Method: "电极法", ExpectedRevision: 1})
	if !errors.Is(err, ErrInvalidInput) || spy.calls != 0 {
		t.Fatalf("invalid edit: err=%v calls=%d", err, spy.calls)
	}
	item, err := svc.Update(context.Background(), "account:admin", "ti-001", UpdateInput{Name: " 水质 pH ", Category: "水质", Method: "电极法", ExpectedRevision: 1})
	if err != nil || spy.input.Name != "水质 pH" || item.Revision != 2 {
		t.Fatalf("valid edit: item=%+v err=%v input=%+v", item, err, spy.input)
	}
}

type updateRow struct {
	values []any
	err    error
}

func (r updateRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	for i, value := range r.values {
		switch target := dest[i].(type) {
		case *string:
			*target = value.(string)
		case *int64:
			*target = value.(int64)
		case *bool:
			*target = value.(bool)
		}
	}
	return nil
}

type updateTx struct{ noRows bool }

func (tx updateTx) QueryRow(_ context.Context, sql string, args ...any) pgx.Row {
	if tx.noRows {
		return updateRow{err: pgx.ErrNoRows}
	}
	return updateRow{values: []any{args[0].(string), "WATER-PH", args[2].(string), args[3].(string), args[4].(string), args[5].(string), args[1].(int64) + 1}}
}
func (tx updateTx) Query(context.Context, string, ...any) (pgx.Rows, error) { return nil, nil }
func (tx updateTx) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}

type updateDB struct {
	committed bool
	opts      db.TxOptions
}

func (d *updateDB) InTx(ctx context.Context, opts db.TxOptions, fn func(context.Context, db.DBTX) error) error {
	d.opts = opts
	err := fn(ctx, updateTx{})
	d.committed = err == nil
	return err
}

type updateAudit struct {
	entry audit.Entry
	err   error
}

func (a *updateAudit) Write(context.Context, audit.Entry) error { return nil }
func (a *updateAudit) WriteTx(_ context.Context, _ db.DBTX, e audit.Entry) error {
	a.entry = e
	return a.err
}
func TestUpdateAuditIsAtomicWithBusinessWrite(t *testing.T) {
	database := &updateDB{}
	writer := &updateAudit{}
	store := databaseStore{db: database, audit: writer}
	input := UpdateInput{Name: "水质 pH", Category: "水质", Method: "电极法", ExpectedRevision: 1}
	item, err := store.Update(context.Background(), "account:admin", "ti-001", input)
	if err != nil || !database.committed || database.opts.ReadOnly || item.Revision != 2 || writer.entry.Action != "product.testitem.update" || writer.entry.Actor != "account:admin" {
		t.Fatalf("update/audit: item=%+v err=%v committed=%t audit=%+v", item, err, database.committed, writer.entry)
	}
	writer.err = errors.New("audit down")
	_, err = store.Update(context.Background(), "account:admin", "ti-001", input)
	if !errors.Is(err, writer.err) || database.committed {
		t.Fatalf("audit failure must abort update: err=%v committed=%t", err, database.committed)
	}
}

func TestUpdateRequiresManagePermission(t *testing.T) {
	mux := testRouter(t, false, fakeReader{})
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPut, "/api/product-admin/v1/test-items/ti-001", strings.NewReader(`{"name":"x","category":"y","method":"z","description":"","expectedRevision":1}`)))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("PUT status %d", rec.Code)
	}
}
func TestUpdateRejectsMalformedBody(t *testing.T) {
	mux := testRouter(t, true, fakeReader{})
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPut, "/api/product-admin/v1/test-items/ti-001", strings.NewReader(`{"name":`)))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("PUT malformed body status %d", rec.Code)
	}
}

func TestDictionaryDetailNotFound(t *testing.T) {
	mux := testRouter(t, true, fakeReader{err: ErrNotFound})
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/product-admin/v1/test-items/missing", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status %d", rec.Code)
	}
}
