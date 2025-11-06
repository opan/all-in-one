package repository

import (
	"github.com/all-in-one/internal/listing/pkg/model"
	"github.com/all-in-one/internal/listing/pkg/repository/memory"
	"github.com/all-in-one/internal/listing/pkg/repository/sqlite"
)

// topicRepositoryWrapper wraps the different topic repository implementations
type topicRepositoryWrapper struct {
	storageType string
	memRepo     memory.TopicRepository
	sqlRepo     sqlite.TopicRepository
}

func (r *topicRepositoryWrapper) GetAll() ([]model.Topic, error) {
	if r.storageType == "memory" {
		return r.memRepo.GetAll()
	}
	return r.sqlRepo.GetAll()
}

func (r *topicRepositoryWrapper) Get(id int) (model.Topic, error) {
	if r.storageType == "memory" {
		return r.memRepo.Get(id)
	}
	return r.sqlRepo.Get(id)
}

func (r *topicRepositoryWrapper) Create(topic model.Topic) (model.Topic, error) {
	if r.storageType == "memory" {
		return r.memRepo.Create(topic)
	}
	return r.sqlRepo.Create(topic)
}

func (r *topicRepositoryWrapper) Update(id int, topic model.Topic) (model.Topic, error) {
	if r.storageType == "memory" {
		return r.memRepo.Update(id, topic)
	}
	return r.sqlRepo.Update(id, topic)
}

func (r *topicRepositoryWrapper) Delete(id int) error {
	if r.storageType == "memory" {
		return r.memRepo.Delete(id)
	}
	return r.sqlRepo.Delete(id)
}
