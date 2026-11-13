package bootstrap

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"saas-tic-tools/apps/api/internal/config"
	"github.com/0xunknownz/saas.starter/foundation/lifecycle"
	apihttp "github.com/0xunknownz/saas.starter/foundation/http"
)

func get(h http.Handler, path string) int {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", path, nil))
	return rec.Code
}

func TestAppServesOnlyAfterStart(t *testing.T) {
	app, err := New(context.Background(), config.Config{Env: "development", SecretsKey: testSecretsKey(),
		StorageProvider: "local", StorageLocalRoot: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	if got := get(app.Handler, "/api/example/hello"); got != 503 {
		t.Fatalf("before start: %d, want 503", got)
	}
	if err := app.Lifecycle.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	if get(app.Handler, "/readyz") != 200 || get(app.Handler, "/api/example/hello") != 200 {
		t.Fatal("expected ready and serving after start")
	}
	if err := app.Lifecycle.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	if get(app.Handler, "/readyz") != 503 {
		t.Fatal("expected not ready after stop")
	}
}

func TestDatabaseUnreachableBlocksReadiness(t *testing.T) {
	app, err := New(context.Background(), config.Config{Env: "development", SecretsKey: testSecretsKey(),
		StorageProvider: "local", StorageLocalRoot: t.TempDir(), DatabaseURL: "postgres://u:p@127.0.0.1:1/db"})
	if err != nil {
		t.Fatal(err)
	}
	if err := app.Lifecycle.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := get(app.Handler, "/readyz"); got != 503 {
		t.Fatalf("readyz = %d, want 503 with database down", got)
	}
}

type recorder struct {
	mu   sync.Mutex
	done bool // set when Stop ran
}

func (r *recorder) Stop(context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.done = true
	return nil
}

func (r *recorder) stopped() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.done
}

// PLG-05: a request in flight when shutdown begins finishes before any component is stopped,
// and readiness is withdrawn immediately.
func TestShutdownDrainsInFlightBeforeStoppingComponents(t *testing.T) {
	lc := &lifecycle.Manager{}
	rec := &recorder{}
	lc.Add("component", rec)
	if err := lc.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	entered, release := make(chan struct{}), make(chan struct{})
	stoppedDuringRequest := make(chan bool, 1)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /slow", func(w http.ResponseWriter, r *http.Request) {
		close(entered)
		<-release
		stoppedDuringRequest <- rec.stopped()
		w.WriteHeader(200)
	})
	app := &App{Handler: apihttp.New(lc.Ready, mux), Lifecycle: lc}
	srv := &http.Server{Handler: app.Handler}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go srv.Serve(ln)
	base := "http://" + ln.Addr().String()

	status := make(chan int, 1)
	go func() {
		resp, err := http.Get(base + "/slow")
		if err != nil {
			status <- -1
			return
		}
		resp.Body.Close()
		status <- resp.StatusCode
	}()
	<-entered

	shutdownDone := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		shutdownDone <- app.Shutdown(ctx, srv)
	}()
	deadline := time.Now().Add(2 * time.Second)
	for lc.Ready(context.Background()) == nil {
		if time.Now().After(deadline) {
			t.Fatal("readiness not revoked at shutdown start")
		}
		time.Sleep(5 * time.Millisecond)
	}
	if rec.stopped() {
		t.Fatal("component stopped while a request was still in flight")
	}
	close(release)
	if err := <-shutdownDone; err != nil {
		t.Fatal(err)
	}
	if s := <-status; s != 200 {
		t.Fatalf("in-flight request got %d, want 200", s)
	}
	if <-stoppedDuringRequest || !rec.stopped() {
		t.Fatal("expected the request to finish first and the component to stop afterwards")
	}
}
