package storage

import (
	"github.com/all-in-one/internal/config"
	"github.com/jmoiron/sqlx"
)

type Storage interface {
	DB() *sqlx.DB
	Migrate()
}

func NewStorage(config config.Config) (Storage, error) {
	switch config.Storage.Type {
	case "sqlite":
		return NewSQLite(config)
	default:
		panic("unsupported storage type: " + config.Storage.Type)
	}
}
