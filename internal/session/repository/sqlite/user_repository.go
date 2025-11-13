package sqlite

import (
	"context"

	"github.com/all-in-one/internal/config"
	"github.com/jmoiron/sqlx"
)

type userRepository struct {
	db *sqlx.DB
}

func NewUserRepository(ctx context.Context, config config.Config) *userRepository {
	return &userRepository{
		db: sqlx.MustConnect("sqlite3", config.Session.Database.Source),
	}
}
