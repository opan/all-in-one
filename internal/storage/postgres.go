package storage

import (
	"fmt"

	"github.com/XSAM/otelsql"
	"github.com/all-in-one/internal/config"
	"github.com/golang-migrate/migrate/v4"
	postgresMigrate "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"go.opentelemetry.io/otel/attribute"
)

type postgresStorage struct {
	db *sqlx.DB
}

func NewPostgres(config config.Config) (*postgresStorage, error) {
	sqlDB, err := otelsql.Open("postgres", config.Storage.Postgres.DSN(),
		otelsql.WithAttributes(attribute.String("db.system", "postgresql")),
	)
	if err != nil {
		return nil, err
	}

	db := sqlx.NewDb(sqlDB, "postgres")

	otelsql.RegisterDBStatsMetrics(sqlDB,
		otelsql.WithAttributes(attribute.String("db.system", "postgresql")),
	)

	return &postgresStorage{db: db}, nil
}

func (s *postgresStorage) DB() *sqlx.DB { return s.db }

func (s *postgresStorage) newMigrator() (*migrate.Migrate, error) {
	driver, err := postgresMigrate.WithInstance(s.db.DB, &postgresMigrate.Config{})
	if err != nil {
		return nil, fmt.Errorf("failed to create migration driver: %w", err)
	}

	m, err := migrate.NewWithDatabaseInstance(
		"file://db/migrations/postgres",
		"postgres",
		driver,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create migrate instance: %w", err)
	}

	return m, nil
}

func (s *postgresStorage) MigrateUp() error {
	m, err := s.newMigrator()
	if err != nil {
		return err
	}

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("migration up failed: %w", err)
	}

	return nil
}

func (s *postgresStorage) MigrateDown(steps int) error {
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
