// Package example is a minimal Product plugin that shows the whole chain: manifest, explicit
// dependency on the audit plugin, HTTP route, lifecycle hooks. Replace it with real business code.
package example

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/0xunknownz/saas.starter/foundation/audit"
	"github.com/0xunknownz/saas.starter/foundation/plugin"
)

type Config struct {
	Greeting string `json:"greeting"`
}

// Deps lists exactly the plugins declared under dependencies in plugin.yaml.
type Deps struct {
	Audit audit.Writer
}

type Service struct {
	greeting string
	audit    audit.Writer
}

func NewService(cfg Config, deps Deps) (*Service, error) {
	g := cfg.Greeting
	if g == "" {
		g = "hello from saas-tic-tools"
	}
	return &Service{greeting: g, audit: deps.Audit}, nil
}

// Greet returns the greeting and records an audit entry.
func (s *Service) Greet(ctx context.Context) (string, error) {
	return s.greeting, s.audit.Write(ctx, audit.Entry{Actor: "anonymous", Action: "example.greet", Target: "example"})
}

// HTTPDeps names plugins that must be in the composition; they are not domain dependencies.
type HTTPDeps struct{}

func RegisterHTTP(r plugin.Router, svc *Service, deps HTTPDeps) error {
	r.Handle("GET /api/example/hello", http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		msg, err := svc.Greet(req.Context())
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"message": msg})
	}))
	return nil
}
