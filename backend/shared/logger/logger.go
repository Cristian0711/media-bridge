package logger

import (
	"sync"

	"go.uber.org/zap"
)

var (
	once sync.Once
	l    *zap.Logger
)

func L() *zap.Logger {
	once.Do(func() {
		cfg := zap.NewProductionConfig()
		cfg.DisableStacktrace = true
		logger, err := cfg.Build()
		if err != nil {
			panic(err)
		}
		l = logger
	})
	return l
}

func Named(component string) *zap.Logger {
	return L().Named(component)
}
