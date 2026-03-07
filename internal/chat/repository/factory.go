package repository

import (
	"context"
	"fmt"

	"github.com/all-in-one/internal/config"
	"github.com/rs/zerolog"
)

// NewStorage creates a new Storage instance based on the configuration
func NewStorage(ctx context.Context, config config.Config, log zerolog.Logger) (Storage, error) {
	dbType := config.Storage.Type
	if dbType == "" {
		dbType = "sqlite"
	}

	switch dbType {
	case "sqlite":
		return NewSQLiteStorage(ctx, config, log)
	case "postgres":
		// TODO: Implement PostgreSQL storage when needed
		return nil, fmt.Errorf("postgres storage not yet implemented")
	default:
		return nil, fmt.Errorf("unsupported database type: %s", dbType)
	}
}
