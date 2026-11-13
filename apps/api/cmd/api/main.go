package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"saas-tic-tools/apps/api/internal/bootstrap"
	"saas-tic-tools/apps/api/internal/config"
	"github.com/0xunknownz/saas.starter/foundation/requestid"
)

func main() {
	if err := run(); err != nil {
		slog.Error("api exited", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	// Install the context-aware slog handler before anything logs: records with a request context
	// gain request_id, records without one keep their shape. The handler wraps an explicit text
	// handler: wrapping slog's built-in default handler would recurse through the standard log
	// package's writer and deadlock (log/slog.SetDefault only skips that redirect for the exact
	// defaultHandler type).
	slog.SetDefault(slog.New(requestid.NewHandler(slog.NewTextHandler(os.Stderr, nil))))
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	app, err := bootstrap.New(ctx, cfg)
	if err != nil {
		return err
	}
	if err := app.Lifecycle.Start(ctx); err != nil {
		return err
	}
	srv := &http.Server{Addr: cfg.HTTPAddr, Handler: app.Handler, ReadHeaderTimeout: 10 * time.Second}
	errc := make(chan error, 1)
	go func() { errc <- srv.ListenAndServe() }()
	slog.Info("api listening", "addr", cfg.HTTPAddr, "env", cfg.Env)

	select {
	case err := <-errc:
		_ = app.Lifecycle.Stop(context.Background())
		return err
	case <-ctx.Done():
	}
	shutdown, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	return app.Shutdown(shutdown, srv)
}
