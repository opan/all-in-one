package repository

import (
	"github.com/all-in-one/internal/authnz/repository/sqlite"
	"github.com/all-in-one/internal/config"
	"github.com/jmoiron/sqlx"
)

type baseStorage interface {
	Close() error
}

func NewRepo(db *sqlx.DB, config config.Config) (Storage, error) {
	switch config.Storage.Type {
	case "sqlite":
		storage := sqlite.NewStorage(db, config)
		return &sqliteStoreAdapter{
			userRepo:    storage.UserRepo(),
			sessionRepo: storage.SessionRepo(),
		}, nil
	default:
		panic("unsupported storage type: " + config.Storage.Type)
	}
}
