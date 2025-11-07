package repository

import (
	"github.com/all-in-one/internal/listing/pkg/model"
	"github.com/all-in-one/internal/listing/pkg/repository/memory"
	"github.com/all-in-one/internal/listing/pkg/repository/sqlite"
)

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
