package storage

import (
	"github.com/all-in-one/internal/config"
	"github.com/jmoiron/sqlx"
)

type Storage interface {
	DB() *sqlx.DB
	MigrateUp() error
	MigrateDown(steps int) error
}

func NewStorage(config config.Config) (Storage, error) {
	switch config.Storage.Type {
	case "sqlite":
		return NewSQLite(config)
	case "postgres":
		return NewPostgres(config)
	default:
		panic("unsupported storage type: " + config.Storage.Type)
	}
}
