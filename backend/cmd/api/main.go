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
	"go.uber.org/zap"
)

func main() {
	defer func() { _ = logger.L().Sync() }()
	log := logger.Named("app.main")

	srv, err := app.Bootstrap()
	if err != nil {
		log.Fatal("bootstrap failed", zap.Error(err))
	}

	// Report a server failure back to main over a channel instead of calling
	// log.Fatal from the goroutine. log.Fatal calls os.Exit, which would skip
	// graceful shutdown (worker drain, pool close, SSE broker close) and the
	// deferred logger.Sync.
	srvErr := make(chan error, 1)
	go func() {
		if err := srv.Run(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			srvErr <- err
		}
	}()
	log.Info("server started")

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-srvErr:
		log.Error("server failed, draining", zap.Error(err))
	case s := <-sig:
		log.Info("shutdown signal received, draining", zap.String("signal", s.String()))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Error("graceful shutdown failed", zap.Error(err))
	}
	log.Info("shutdown complete")
}
