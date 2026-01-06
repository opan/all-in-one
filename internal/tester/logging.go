package tester

import (
	"context"

	"github.com/all-in-one/internal/logging"
	"github.com/rs/zerolog"
)

func ContextWithLogger() context.Context {
	logger := zerolog.New(nil).Level(zerolog.Disabled)
	return WithLogger(context.Background(), logger)
}

// helper function for testing
func WithLogger(ctx context.Context, logger zerolog.Logger) context.Context {
	return context.WithValue(ctx, logging.LoggerKey, &logger)
}
