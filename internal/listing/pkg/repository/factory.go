package repository

import (
	"context"
	"fmt"

	"github.com/all-in-one/internal/config"
	"github.com/all-in-one/internal/listing/pkg/model"
	"github.com/all-in-one/internal/listing/pkg/repository/memory"
	"github.com/all-in-one/internal/listing/pkg/repository/sqlite"
)

// storageWrapper wraps the different storage implementations
type storageWrapper struct {
	storageType   string
	memStorage    memory.Storage
	sqliteStorage sqlite.Storage
}

func (s *storageWrapper) ItemRepo() ItemRepository {
	if s.storageType == "memory" {
		return &itemRepositoryWrapper{
			storageType: "memory",
			memRepo:     s.memStorage.ItemRepo(),
		}
	}
	return &itemRepositoryWrapper{
		storageType: "sqlite",
		sqlRepo:     s.sqliteStorage.ItemRepo(),
	}
}

func (s *storageWrapper) TopicRepo() TopicRepository {
	if s.storageType == "memory" {
		return &topicRepositoryWrapper{
			storageType: "memory",
			memRepo:     s.memStorage.TopicRepo(),
		}
	}
	return &topicRepositoryWrapper{
		storageType: "sqlite",
		sqlRepo:     s.sqliteStorage.TopicRepo(),
	}
}

func (s *storageWrapper) Close() error {
	if s.storageType == "memory" {
		return s.memStorage.Close()
	}
	return s.sqliteStorage.Close()

}

// itemRepositoryWrapper wraps the different item repository implementations
type itemRepositoryWrapper struct {
	storageType string
	memRepo     memory.ItemRepository
	sqlRepo     sqlite.ItemRepository
}

func (r *itemRepositoryWrapper) GetAll() ([]model.Item, error) {
	if r.storageType == "memory" {
		return r.memRepo.GetAll()
	}
	return r.sqlRepo.GetAll()
}

func (r *itemRepositoryWrapper) Get(id int) (model.Item, error) {
	if r.storageType == "memory" {
		return r.memRepo.Get(id)
	}
	return r.sqlRepo.Get(id)
}

func (r *itemRepositoryWrapper) Create(item model.Item) (model.Item, error) {
	if r.storageType == "memory" {
		return r.memRepo.Create(item)
	}
	return r.sqlRepo.Create(item)
}

func (r *itemRepositoryWrapper) Update(id int, item model.Item) (model.Item, error) {
	if r.storageType == "memory" {
		return r.memRepo.Update(id, item)
	}
	return r.sqlRepo.Update(id, item)
}

func (r *itemRepositoryWrapper) Delete(id int) error {
	if r.storageType == "memory" {
		return r.memRepo.Delete(id)
	}
	return r.sqlRepo.Delete(id)
}

// NewStorage creates a new storage instance based on the storage type
func NewStorage(ctx context.Context, config config.Config) (Storage, error) {
	switch config.Storage.Type {
	case "memory":
		memStorage := memory.NewStorage()
		return &storageWrapper{
			storageType: "memory",
			memStorage:  memStorage,
		}, nil
	case "sqlite":
		sqliteStorage, err := sqlite.NewStorage(ctx, config)
		if err != nil {
			return nil, err
		}
		return &storageWrapper{
			storageType:   "sqlite",
			sqliteStorage: sqliteStorage,
		}, nil
	default:
		return nil, fmt.Errorf("unsupported storage type: %s", config.Storage.Type)
	}
}
