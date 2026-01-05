package storage

import (
	"github.com/all-in-one/internal/config"
	"github.com/jmoiron/sqlx"
)

func NewStorage(config config.Config) (*sqlx.DB, error) {
	switch config.Storage.Type {
	case "sqlite":
		return NewSQLite(config)
	default:
		panic("unsupported storage type: " + config.Storage.Type)
	}
}
