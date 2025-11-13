package sqlite

import (
	"context"

	"github.com/all-in-one/internal/config"
	"github.com/jmoiron/sqlx"
)

type sessionRepository struct {
	db *sqlx.DB
}

func NewSessionRepository(ctx context.Context, config config.Config) *sessionRepository {
	return &sessionRepository{
		db: sqlx.MustConnect("sqlite3", config.Session.Database.Source),
	}
}

func (r *sessionRepository) Create(ctx context.Context) error {
	return nil
}
