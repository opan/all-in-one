package repository

import (
	"context"
	"fmt"

	"github.com/all-in-one/internal/config"
	"github.com/all-in-one/internal/listing/pkg/repository/sqlite"
)

type baseStorage interface {
	Close() error
	InitializeSampleData(ctx context.Context) int
}

func NewStorage(ctx context.Context, config config.Config) (Storage, error) {
	switch config.Storage.Type {
	// case "memory":
	// 	memStorage := memory.NewStorage()
	// 	return &memoryStorageAdapter{
	// 		itemRepo:  memStorage.ItemRepo(),
	// 		topicRepo: memStorage.TopicRepo(),
	// 		storage:   memStorage,
	// 	}, nil
	case "sqlite":
		sqliteStorage, err := sqlite.NewStorage(ctx, config)
		if err != nil {
			return nil, err
		}
		return &sqliteStorageAdapter{
			itemRepo:    sqliteStorage.ItemRepo(),
			topicRepo:   sqliteStorage.TopicRepo(),
			sessionRepo: sqliteStorage.SessionRepo(),
			storage:     sqliteStorage,
		}, nil
	default:
		return nil, fmt.Errorf("unsupported storage type: %s", config.Storage.Type)
	}
}
