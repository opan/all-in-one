package memory

import (
	"context"
	"sync"
	"time"

	httpHelper "github.com/all-in-one/internal/http"
	"github.com/all-in-one/internal/listing/model"
	"github.com/all-in-one/internal/query"
)

type itemRepository struct {
	items  map[int]model.Item
	lastID int
	mutex  sync.RWMutex
}

func newItemRepository() *itemRepository {
	return &itemRepository{
		items:  make(map[int]model.Item),
		lastID: 0,
	}
}

func (r *itemRepository) GetAll(ctx context.Context) ([]model.Item, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	items := make([]model.Item, 0, len(r.items))
	for _, item := range r.items {
		items = append(items, item)
	}

	return items, nil
}

func (r *itemRepository) GetByTopicID(ctx context.Context, topicID int) ([]model.Item, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	items := make([]model.Item, 0)
	for _, item := range r.items {
		if item.TopicID == topicID {
			items = append(items, item)
		}
	}

	return items, nil
}

func (r *itemRepository) Get(ctx context.Context, id int) (model.Item, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	item, exists := r.items[id]
	if !exists {
		return model.Item{}, httpHelper.ErrNotFound
	}

	return item, nil
}

func (r *itemRepository) Create(ctx context.Context, item model.Item) (model.Item, error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	r.lastID++
	item.ID = r.lastID
	item.CreatedAt = time.Now()
	item.UpdatedAt = time.Now()

	r.items[item.ID] = item

	return item, nil
}

func (r *itemRepository) Update(ctx context.Context, id int, item model.Item) (model.Item, error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	existingItem, exists := r.items[id]
	if !exists {
		return model.Item{}, httpHelper.ErrNotFound
	}

	item.ID = id
	item.CreatedAt = existingItem.CreatedAt
	item.UpdatedAt = time.Now()

	r.items[id] = item

	return item, nil
}

func (r *itemRepository) Delete(ctx context.Context, id int) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	_, exists := r.items[id]
	if !exists {
		return httpHelper.ErrNotFound
	}

	delete(r.items, id)
	return nil
}

func (r *itemRepository) DeleteByTopicID(ctx context.Context, topicID int, opts ...query.QueryOptions) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	for id, item := range r.items {
		if item.TopicID == topicID {
			delete(r.items, id)
		}
	}

	return nil
}
