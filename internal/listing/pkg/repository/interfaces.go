package repository

import (
	"context"

	"github.com/all-in-one/internal/listing/pkg/model"
	"github.com/all-in-one/internal/listing/query"
)

// ItemRepository defines the interface for item storage operations
type ItemRepository interface {
	// GetAll returns all listing items
	GetAll(ctx context.Context) ([]model.Item, error)

	// GetByTopicID returns all items for a specific topic
	GetByTopicID(ctx context.Context, topicID int) ([]model.Item, error)

	// Get returns a listing item by ID
	Get(ctx context.Context, id int) (model.Item, error)

	// Create adds a new listing item
	Create(ctx context.Context, item model.Item) (model.Item, error)

	// Update modifies an existing listing item
	Update(ctx context.Context, id int, item model.Item) (model.Item, error)

	// Delete removes a listing item
	Delete(ctx context.Context, id int) error
	DeleteByTopicID(ctx context.Context, topicID int, opts ...query.QueryOptions) error
}

type TopicRepository interface {
	CreateTrx(ctx context.Context) (query.QueryOptions, error)

	GetAll() ([]model.Topic, error)
	Get(id int) (model.Topic, error)
	Create(item model.Topic) (model.Topic, error)
	Update(id int, item model.Topic) (model.Topic, error)
	Delete(ctx context.Context, id int, opts ...query.QueryOptions) error
}

// Storage defines the main storage interface that aggregates all repositories
type Storage interface {

	// ItemRepo returns the item repository
	ItemRepo() ItemRepository
	TopicRepo() TopicRepository

	// Close closes the storage connection
	Close() error

	InitializeSampleData(ctx context.Context) int
}
