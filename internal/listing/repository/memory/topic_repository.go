package memory

import (
	"context"
	"fmt"
	"sync"
	"time"

	httpHelper "github.com/all-in-one/internal/http"
	"github.com/all-in-one/internal/listing/model"
	"github.com/all-in-one/internal/query"
)

type queryOptions struct {
	inTransaction bool
}

func (q *queryOptions) Commit() error {
	return nil
}

func (q *queryOptions) Rollback() error {
	return nil
}

type topicRepository struct {
	topics map[int]model.Topic
	lastID int
	mutex  sync.RWMutex
}

func newTopicRepository() *topicRepository {
	return &topicRepository{
		topics: make(map[int]model.Topic),
		lastID: 0,
	}
}

func (t *topicRepository) CreateTrx(ctx context.Context) (query.QueryOptions, error) {
	return &queryOptions{inTransaction: true}, nil
}

func (t *topicRepository) GetAll(ctx context.Context) ([]model.Topic, error) {
	t.mutex.RLock()
	defer t.mutex.RUnlock()

	topics := make([]model.Topic, 0, len(t.topics))
	for _, topic := range t.topics {
		topics = append(topics, topic)
	}

	return topics, nil
}

func (t *topicRepository) Get(ctx context.Context, id int) (model.Topic, error) {
	t.mutex.RLock()
	defer t.mutex.RUnlock()

	topic, exists := t.topics[id]
	if !exists {
		return model.Topic{}, fmt.Errorf("topic with id %d not found", id)
	}

	return topic, nil
}

func (t *topicRepository) Create(ctx context.Context, topic model.Topic) (model.Topic, error) {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	t.lastID++
	topic.ID = t.lastID
	topic.CreatedAt = time.Now()
	topic.UpdatedAt = time.Now()

	t.topics[topic.ID] = topic

	return topic, nil
}

func (t *topicRepository) Update(ctx context.Context, id int, topic model.Topic) (model.Topic, error) {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	existingTopic, exists := t.topics[id]
	if !exists {
		return model.Topic{}, httpHelper.ErrNotFound
	}

	topic.ID = id
	topic.CreatedAt = existingTopic.CreatedAt
	topic.UpdatedAt = time.Now()

	t.topics[id] = topic

	return topic, nil
}

func (t *topicRepository) Delete(ctx context.Context, id int, opts ...query.QueryOptions) error {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	_, exists := t.topics[id]
	if !exists {
		return fmt.Errorf("topic with id %d not found", id)
	}

	delete(t.topics, id)
	return nil
}
