package storage

import (
	"fmt"

	"github.com/all-in-one/internal/config"
	"github.com/golang-migrate/migrate/v4"
	sqlite3Migrate "github.com/golang-migrate/migrate/v4/database/sqlite3"
	"github.com/jmoiron/sqlx"
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

func (s *sqliteStorage) newMigrator() (*migrate.Migrate, error) {
	driver, err := sqlite3Migrate.WithInstance(s.db.DB, &sqlite3Migrate.Config{})
	if err != nil {
		return nil, fmt.Errorf("failed to create migration driver: %w", err)
	}

	m, err := migrate.NewWithDatabaseInstance(
		"file://db/migrations/sqlite3",
		"sqlite3",
		driver,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create migrate instance: %w", err)
	}

	return m, nil
}

func (s *sqliteStorage) MigrateUp() error {
	m, err := s.newMigrator()
	if err != nil {
		return err
	}

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("migration up failed: %w", err)
	}

	return nil
}

func (s *sqliteStorage) MigrateDown(steps int) error {
	m, err := s.newMigrator()
	if err != nil {
		return err
	}

	if steps == 0 {
		if err := m.Down(); err != nil && err != migrate.ErrNoChange {
			return fmt.Errorf("migration down failed: %w", err)
		}
		return nil
	}

	if err := m.Steps(-steps); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("migration down %d step(s) failed: %w", steps, err)
	}

	return nil
}
