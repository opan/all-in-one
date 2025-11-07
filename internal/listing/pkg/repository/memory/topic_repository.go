package memory

import (
	"fmt"

	"github.com/all-in-one/internal/listing/pkg/model"
)

// topicRepository is a placeholder for topic repository (not implemented yet)
type topicRepository struct{}

func (t *topicRepository) GetAll() ([]model.Topic, error) {
	return nil, fmt.Errorf(errTopicRepoNotImplemented)
}

func (t *topicRepository) Get(id int) (model.Topic, error) {
	return model.Topic{}, fmt.Errorf(errTopicRepoNotImplemented)
}

func (t *topicRepository) Create(item model.Topic) (model.Topic, error) {
	return model.Topic{}, fmt.Errorf(errTopicRepoNotImplemented)
}

func (t *topicRepository) Update(id int, item model.Topic) (model.Topic, error) {
	return model.Topic{}, fmt.Errorf(errTopicRepoNotImplemented)
}

func (t *topicRepository) Delete(id int) error {
	return fmt.Errorf(errTopicRepoNotImplemented)
}
