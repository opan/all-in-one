package repository

import (
	"github.com/all-in-one/internal/authnz/repository/postgres"
	"github.com/all-in-one/internal/authnz/repository/recoverystore"
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
		s := sqlite.NewStorage(db, config)
		return &sqliteStoreAdapter{
			userRepo:    s.UserRepo(),
			sessionRepo: s.SessionRepo(),
			totpRepo:    s.TOTPRepo(),
			resetRepo:   recoverystore.NewPasswordResets(db),
			emailRepo:   recoverystore.NewEmailVerifications(db),
		}, nil
	case "postgres":
		s := postgres.NewStorage(db, config)
		return &sqliteStoreAdapter{
			userRepo:    s.UserRepo(),
			sessionRepo: s.SessionRepo(),
			totpRepo:    s.TOTPRepo(),
			resetRepo:   recoverystore.NewPasswordResets(db),
			emailRepo:   recoverystore.NewEmailVerifications(db),
		}, nil
	default:
		panic("unsupported storage type: " + config.Storage.Type)
	}
}
