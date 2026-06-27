package repository

import (
	"fmt"

	"github.com/all-in-one/internal/config"
	"github.com/all-in-one/internal/listing/repository/postgres"
	"github.com/all-in-one/internal/listing/repository/sqlite"
	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog"
)

type baseStorage interface {
	Close() error
}

func NewStorage(db *sqlx.DB, config config.Config, log zerolog.Logger) (Storage, error) {
	switch config.Storage.Type {
	case "sqlite":
		s := sqlite.NewFromDB(db, log)
		return &sqliteStorageAdapter{
			itemRepo:  s.ItemRepo(),
			topicRepo: s.TopicRepo(),
			storage:   s,
		}, nil
	case "postgres":
		s := postgres.NewFromDB(db, log)
		return &sqliteStorageAdapter{
			itemRepo:  s.ItemRepo(),
			topicRepo: s.TopicRepo(),
			storage:   s,
		}, nil
	default:
		return nil, fmt.Errorf("unsupported storage type: %s", config.Storage.Type)
	}
}
