package logging

import (
	"context"
	"os"

	"github.com/all-in-one/internal/config"
	"github.com/rs/zerolog"
)

func New(cfg config.LoggingConfig) (zerolog.Logger, error) {

	// Setup logging
	log := zerolog.New(os.Stderr).With().Timestamp().Logger()
	level, err := zerolog.ParseLevel(cfg.Level)
	if err != nil {
		return log, err
	}
	zerolog.SetGlobalLevel(level)

	return log, nil
}

type contextKey string

const loggerKey contextKey = "logger"

func GetLoggerFromContext(ctx context.Context) *zerolog.Logger {
	if logger, ok := ctx.Value(loggerKey).(*zerolog.Logger); ok {
		return logger
	}
	// Fallback to a default logger if not found
	defaultLogger := zerolog.Nop()
	return &defaultLogger
}
