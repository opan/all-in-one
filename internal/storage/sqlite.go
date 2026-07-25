package storage

import (
	"fmt"

	"github.com/all-in-one/internal/config"
	"github.com/golang-migrate/migrate/v4"
	sqlite3Migrate "github.com/golang-migrate/migrate/v4/database/sqlite3"
	"github.com/jmoiron/sqlx"
	"github.com/XSAM/otelsql"
	"go.opentelemetry.io/otel/attribute"
)

type sqliteStorage struct {
	db *sqlx.DB
}

func NewSQLite(config config.Config) (*sqliteStorage, error) {
	// otelsql wraps the database/sql driver so each query becomes a child span
	// of the request's server span. sqlx.NewDb adapts the *sql.DB back into a
	// *sqlx.DB without losing the otelsql wrapper.
	sqlDB, err := otelsql.Open("sqlite3", config.Storage.SQLite.DBPath+"?_foreign_keys=on",
		otelsql.WithAttributes(attribute.String("db.system", "sqlite")),
	)
	if err != nil {
		return nil, err
	}

	db := sqlx.NewDb(sqlDB, "sqlite3")

	// Export connection-pool metrics (open, idle, wait counts) via the global
	// MeterProvider. observability.Init runs before NewStorage in server.go so
	// the provider is already set when this line executes.
	otelsql.RegisterDBStatsMetrics(sqlDB,
		otelsql.WithAttributes(attribute.String("db.system", "sqlite")),
	)

	return &sqliteStorage{
		db: db,
	}, nil
}

func (s *sqliteStorage) DB() *sqlx.DB {
	return s.db
}

func (s *sqliteStorage) newMigrator() (*migrate.Migrate, error) {
	// NoTxWrap: migrations that rebuild a table (SQLite has no ALTER COLUMN /
	// DROP CONSTRAINT, so changing a UNIQUE/NOT NULL constraint means
	// CREATE+COPY+DROP+RENAME — see 09_relax_users_name_email_uniqueness)
	// need PRAGMA foreign_keys=OFF to actually take effect, and SQLite
	// silently no-ops that pragma inside a transaction. Trade-off: SQLite
	// migrations no longer auto-rollback a partially-failed file — recovery
	// falls back to golang-migrate's dirty-flag guard (blocks re-running
	// until manually resolved) instead of a clean automatic rollback.
	driver, err := sqlite3Migrate.WithInstance(s.db.DB, &sqlite3Migrate.Config{NoTxWrap: true})
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
