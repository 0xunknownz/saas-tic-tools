// Command worker runs the same composition without an HTTP listener, for background plugins.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"saas-tic-tools/apps/api/internal/bootstrap"
	"saas-tic-tools/apps/api/internal/config"
	"github.com/0xunknownz/saas.starter/foundation/requestid"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("worker config", "error", err)
		os.Exit(1)
	}
	// Same context-aware handler as the API, so stored ids propagated into job contexts show up.
	slog.SetDefault(slog.New(requestid.NewHandler(slog.NewTextHandler(os.Stderr, nil))))
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	app, err := bootstrap.New(ctx, cfg)
	if err == nil {
		err = app.Lifecycle.Start(ctx)
	}
	if err != nil {
		slog.Error("worker start", "error", err)
		os.Exit(1)
	}
	slog.Info("worker running")
	<-ctx.Done()
	shutdown, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := app.Lifecycle.Stop(shutdown); err != nil {
		slog.Error("worker stop", "error", err)
		os.Exit(1)
	}
}
