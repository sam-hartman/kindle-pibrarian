package logger

import (
	"log"

	"go.uber.org/zap"
)

var logger *zap.Logger

func init() {
	var err error
	// Use development config with warn level for all modes
	// Production mode can be enabled via environment variable if needed
	config := zap.NewDevelopmentConfig()
	config.Level = zap.NewAtomicLevelAt(zap.WarnLevel)
	logger, err = config.Build()

	if err != nil {
		log.Fatalf("Failed to initialize zap logger: %v", err)
	}
}

func GetLogger() *zap.Logger {
	return logger
}
