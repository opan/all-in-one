package storage

import (
	"github.com/all-in-one/internal/config"
	"github.com/golang-migrate/migrate/v4"
	sqlite3Migrate "github.com/golang-migrate/migrate/v4/database/sqlite3"
	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog/log"
)

type sqliteStorage struct {
	db *sqlx.DB
}

func NewSQLite(config config.Config) (*sqliteStorage, error) {
	db, err := sqlx.Open("sqlite3", config.Storage.SQLite.DBPath)
	if err != nil {
		return nil, err
	}

	return &sqliteStorage{
		db: db,
	}, nil
}

func (s *sqliteStorage) DB() *sqlx.DB {
	return s.db
}

func (s *sqliteStorage) Migrate() {
	driver, err := sqlite3Migrate.WithInstance(s.db.DB, &sqlite3Migrate.Config{})
	if err != nil {
		log.Error().Err(err).Msg("Failed to create migration driver")
		s.db.Close()
	}

	m, err := migrate.NewWithDatabaseInstance(
		"file://db/migrations/sqlite3",
		"sqlite3",
		driver,
	)
	if err != nil {
		log.Error().Err(err).Msg("Failed to create migrate instance")
		s.db.Close()
	}

	m.Up()
}
