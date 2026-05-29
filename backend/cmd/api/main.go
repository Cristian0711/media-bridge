package main

import (
	"github.com/Cristian0711/media-bridge/backend/internal/app"
	"github.com/Cristian0711/media-bridge/backend/shared/logger"
	"go.uber.org/zap"
)

func main() {
	defer logger.L().Sync()
	log := logger.Named("app.main")

	srv, err := app.Bootstrap()
	if err != nil {
		log.Fatal("bootstrap failed", zap.Error(err))
	}
	if err := srv.Run(); err != nil {
		log.Fatal("server failed", zap.Error(err))
	}
}