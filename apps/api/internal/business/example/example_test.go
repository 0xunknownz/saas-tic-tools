package example

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/0xunknownz/saas.starter/foundation/audit"
	"github.com/0xunknownz/saas.starter/foundation/db"
)

type spy struct{ entries []audit.Entry }

func (s *spy) Write(_ context.Context, e audit.Entry) error { s.entries = append(s.entries, e); return nil }

func (s *spy) WriteTx(ctx context.Context, _ db.DBTX, e audit.Entry) error { return s.Write(ctx, e) }

// Dependencies are injected explicitly, so a test double replaces the real audit plugin (PLG-03).
func TestHelloWritesAudit(t *testing.T) {
	a := &spy{}
	svc, err := NewService(Config{}, Deps{Audit: a})
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	if err := RegisterHTTP(mux, svc, HTTPDeps{}); err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/api/example/hello", nil))
	if rec.Code != 200 || len(a.entries) != 1 {
		t.Fatalf("status %d, audit entries %d", rec.Code, len(a.entries))
	}
}
