package storage

import (
	"github.com/all-in-one/internal/config"
	"github.com/jmoiron/sqlx"
)

func NewSQLite(config config.Config) (*sqlx.DB, error) {
	db, err := sqlx.Open("sqlite3", config.Storage.SQLite.DBPath)
	if err != nil {
		return nil, err
	}

	return db, nil
}
