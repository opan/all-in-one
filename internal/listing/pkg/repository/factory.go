package repository

import (
	"fmt"

	"github.com/all-in-one/internal/listing/pkg/model"
	"github.com/all-in-one/internal/listing/pkg/repository/memory"
	"github.com/all-in-one/internal/listing/pkg/repository/sqlite"
)

// storageWrapper wraps the different storage implementations
type storageWrapper struct {
	storageType string
	memStorage  memory.Storage
	sqlStorage  sqlite.Storage
}

func (s *storageWrapper) Items() ItemRepository {
	if s.storageType == "memory" {
		return &itemRepositoryWrapper{
			storageType: "memory",
			memRepo:     s.memStorage.Items(),
		}
	}
	return &itemRepositoryWrapper{
		storageType: "sqlite",
		sqlRepo:     s.sqlStorage.Items(),
	}
}

func (s *storageWrapper) Close() error {
	if s.storageType == "memory" {
		return s.memStorage.Close()
	}
	return s.sqlStorage.Close()
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

func (r *itemRepositoryWrapper) InitializeSampleData() int {
	if r.storageType == "memory" {
		return r.memRepo.InitializeSampleData()
	}
	return r.sqlRepo.InitializeSampleData()
}

// NewStorage creates a new storage instance based on the storage type
func NewStorage(storageType, connectionString string) (Storage, error) {
	switch storageType {
	case "memory":
		memStorage := memory.NewStorage()
		return &storageWrapper{
			storageType: "memory",
			memStorage:  memStorage,
		}, nil
	case "sqlite":
		sqlStorage, err := sqlite.NewStorage(connectionString)
		if err != nil {
			return nil, err
		}
		return &storageWrapper{
			storageType: "sqlite",
			sqlStorage:  sqlStorage,
		}, nil
	default:
		return nil, fmt.Errorf("unsupported storage type: %s", storageType)
	}
}
