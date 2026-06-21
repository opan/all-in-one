package repository

import (
	"context"
	"fmt"

	"github.com/all-in-one/internal/config"
	"github.com/all-in-one/internal/shortener/repository/postgres"
	"github.com/all-in-one/internal/shortener/repository/sqlite"
	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog"
)

type storageAdapter struct {
	shortLinkRepo ShortLinkRepository
}

func (a *storageAdapter) ShortLinkRepo() ShortLinkRepository { return a.shortLinkRepo }
func (a *storageAdapter) Close() error                       { return nil }

// NewStorageFromDB creates a sqlite-backed Storage from an existing DB connection.
// Used by integration tests.
func NewStorageFromDB(db *sqlx.DB) Storage {
	s := sqlite.NewFromDB(db)
	return &storageAdapter{shortLinkRepo: s.ShortLinkRepo()}
}

func NewStorage(ctx context.Context, db *sqlx.DB, config config.Config, log zerolog.Logger) (Storage, error) {
	switch config.Storage.Type {
	case "sqlite":
		s := sqlite.NewFromDB(db)
		return &storageAdapter{shortLinkRepo: s.ShortLinkRepo()}, nil
	case "postgres":
		s := postgres.NewFromDB(db)
		return &storageAdapter{shortLinkRepo: s.ShortLinkRepo()}, nil
	default:
		return nil, fmt.Errorf("unsupported storage type: %s", config.Storage.Type)
	}
}
