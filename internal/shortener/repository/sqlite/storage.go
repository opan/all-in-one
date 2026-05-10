package sqlite

import (
	"context"

	"github.com/all-in-one/internal/config"
	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
	"github.com/rs/zerolog"
)

type storage struct {
	db            *sqlx.DB
	log           zerolog.Logger
	shortLinkRepo *shortLinkRepository
}

func NewStorage(ctx context.Context, config config.Config, log zerolog.Logger) (*storage, error) {
	db, err := sqlx.Open("sqlite3", config.Storage.SQLite.DBPath)
	if err != nil {
		return nil, err
	}

	return &storage{
		db:            db,
		log:           log,
		shortLinkRepo: newShortLinkRepository(db),
	}, nil
}

func (s *storage) ShortLinkRepo() *shortLinkRepository {
	return s.shortLinkRepo
}

func (s *storage) Close() error {
	return s.db.Close()
}
