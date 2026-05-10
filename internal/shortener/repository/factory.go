package repository

import (
	"context"
	"fmt"

	"github.com/all-in-one/internal/config"
	"github.com/all-in-one/internal/shortener/repository/sqlite"
	"github.com/rs/zerolog"
)

type sqliteStorageAdapter struct {
	shortLinkRepo ShortLinkRepository
	base          interface{ Close() error }
}

func (a *sqliteStorageAdapter) ShortLinkRepo() ShortLinkRepository {
	return a.shortLinkRepo
}

func (a *sqliteStorageAdapter) Close() error {
	return a.base.Close()
}

func NewStorage(ctx context.Context, config config.Config, log zerolog.Logger) (Storage, error) {
	switch config.Storage.Type {
	case "sqlite":
		s, err := sqlite.NewStorage(ctx, config, log)
		if err != nil {
			return nil, err
		}
		return &sqliteStorageAdapter{
			shortLinkRepo: s.ShortLinkRepo(),
			base:          s,
		}, nil
	default:
		return nil, fmt.Errorf("unsupported storage type: %s", config.Storage.Type)
	}
}
