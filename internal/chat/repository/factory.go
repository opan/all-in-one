package repository

import (
	"context"
	"fmt"

	"github.com/all-in-one/internal/config"
	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog"
)

// NewStorage creates a new Storage instance based on the configuration
func NewStorage(ctx context.Context, db *sqlx.DB, config config.Config, log zerolog.Logger) (Storage, error) {
	switch config.Storage.Type {
	case "sqlite":
		return NewSQLiteStorage(db, log), nil
	case "postgres":
		return NewPostgresStorage(db, log), nil
	default:
		return nil, fmt.Errorf("unsupported database type: %s", config.Storage.Type)
	}
}
