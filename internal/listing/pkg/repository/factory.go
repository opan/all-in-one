package repository

import (
	"context"
	"fmt"

	"github.com/all-in-one/internal/config"
	"github.com/all-in-one/internal/listing/pkg/repository/memory"
	"github.com/all-in-one/internal/listing/pkg/repository/sqlite"
)

// baseStorage defines the common interface for underlying storage implementations
type baseStorage interface {
	Close() error
	InitializeSampleData() int
}

// memoryStorageAdapter adapts memory.storage to repository.Storage interface
type memoryStorageAdapter struct {
	itemRepo  ItemRepository
	topicRepo TopicRepository
	storage   baseStorage
}

func (m *memoryStorageAdapter) ItemRepo() ItemRepository {
	return m.itemRepo
}

func (m *memoryStorageAdapter) TopicRepo() TopicRepository {
	return m.topicRepo
}

func (m *memoryStorageAdapter) Close() error {
	return m.storage.Close()
}

func (m *memoryStorageAdapter) InitializeSampleData() int {
	return m.storage.InitializeSampleData()
}

// sqliteStorageAdapter adapts sqlite.storage to repository.Storage interface
type sqliteStorageAdapter struct {
	itemRepo  ItemRepository
	topicRepo TopicRepository
	storage   baseStorage
}

func (s *sqliteStorageAdapter) ItemRepo() ItemRepository {
	return s.itemRepo
}

func (s *sqliteStorageAdapter) TopicRepo() TopicRepository {
	return s.topicRepo
}

func (s *sqliteStorageAdapter) Close() error {
	return s.storage.Close()
}

func (s *sqliteStorageAdapter) InitializeSampleData() int {
	return s.storage.InitializeSampleData()
}

// NewStorage creates a new storage instance based on the storage type
func NewStorage(ctx context.Context, config config.Config) (Storage, error) {
	switch config.Storage.Type {
	case "memory":
		memStorage := memory.NewStorage()
		return &memoryStorageAdapter{
			itemRepo:  memStorage.ItemRepo(),
			topicRepo: memStorage.TopicRepo(),
			storage:   memStorage,
		}, nil
	case "sqlite":
		sqliteStorage, err := sqlite.NewStorage(ctx, config)
		if err != nil {
			return nil, err
		}
		return &sqliteStorageAdapter{
			itemRepo:  sqliteStorage.ItemRepo(),
			topicRepo: sqliteStorage.TopicRepo(),
			storage:   sqliteStorage,
		}, nil
	default:
		return nil, fmt.Errorf("unsupported storage type: %s", config.Storage.Type)
	}
}
