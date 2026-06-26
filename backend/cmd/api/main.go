package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Cristian0711/media-bridge/backend/internal/app"
	"github.com/Cristian0711/media-bridge/backend/shared/logger"
	"github.com/Cristian0711/media-bridge/backend/shared/telemetry"
)

func appVersion() string {
	if v := os.Getenv("APP_VERSION"); v != "" {
		return v
	}
	return "dev"
}

func main() {
	logger.Init()
	ctx := context.Background()
	log := logger.Component("app.main")

	tpShutdown, err := telemetry.Init(ctx, "media-bridge-backend", appVersion())
	if err != nil {
		log.ErrorContext(ctx, "telemetry init failed", logger.Err(err))
		os.Exit(1)
	}

	srv, err := app.Bootstrap()
	if err != nil {
		log.ErrorContext(ctx, "bootstrap failed", logger.Err(err))
		os.Exit(1)
	}

	// Report a server failure back to main over a channel instead of exiting from
	// the goroutine, so a startup failure still runs graceful shutdown (worker
	// drain, pool close, SSE broker close).
	srvErr := make(chan error, 1)
	go func() {
		if err := srv.Run(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			srvErr <- err
		}
	}()
	log.InfoContext(ctx, "server started")

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-srvErr:
		log.ErrorContext(ctx, "server failed, draining", logger.Err(err))
	case s := <-sig:
		log.InfoContext(ctx, "shutdown signal received, draining", "signal", s.String())
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.ErrorContext(ctx, "graceful shutdown failed", logger.Err(err))
	}
	// Flush any buffered spans last, after workers have stopped producing them.
	if err := tpShutdown(shutdownCtx); err != nil {
		log.ErrorContext(ctx, "telemetry shutdown failed", logger.Err(err))
	}
	log.InfoContext(ctx, "shutdown complete")
}
