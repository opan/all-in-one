package repository

import (
	"context"

	"github.com/all-in-one/internal/listing/pkg/model"
	"github.com/all-in-one/internal/listing/query"
	"github.com/google/uuid"
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

	GetAll(ctx context.Context) ([]model.Topic, error)
	Get(ctx context.Context, id int) (model.Topic, error)
	Create(ctx context.Context, item model.Topic) (model.Topic, error)
	Update(ctx context.Context, id int, item model.Topic) (model.Topic, error)
	Delete(ctx context.Context, id int, opts ...query.QueryOptions) error
}

type SessionRepository interface {
	Create(ctx context.Context, session model.Session, opts ...query.QueryOptions) error
	Delete(ctx context.Context, id uuid.UUID) error
	Get(ctx context.Context, id uuid.UUID) (*model.Session, error)
}

type UserRepository interface {
	FindByUsername(ctx context.Context, username string) (*model.User, error)
	Find(ctx context.Context, id string) (*model.User, error)
	Create(ctx context.Context, user model.User) error
}

// Storage defines the main storage interface that aggregates all repositories
type Storage interface {

	// ItemRepo returns the item repository
	ItemRepo() ItemRepository
	TopicRepo() TopicRepository
	SessionRepo() SessionRepository
	UserRepo() UserRepository

	// Close closes the storage connection
	Close() error

	InitializeSampleData(ctx context.Context) int
}
