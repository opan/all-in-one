package memory

import (
	"github.com/all-in-one/internal/listing/pkg/model"
	"github.com/rs/zerolog"
)

const errTopicRepoNotImplemented = "TopicRepo not implemented in memory storage"

// storage implements the Storage interface from repository package
type storage struct {
	itemRepo  *itemRepository
	topicRepo *topicRepository
	log       zerolog.Logger
}

// NewStorage creates a new memory-based storage
func NewStorage() *storage {
	return &storage{
		itemRepo:  newItemRepository(),
		topicRepo: &topicRepository{},
	}
}

type QueryOption func(*QueryOptions)

type QueryOptions struct {
	tx string
}

func (s *storage) WithTx(tx string) QueryOption {
	return func(opts *QueryOptions) {
		// dummy implementation
		opts.tx = tx
	}
}

func (s *storage) Tx() (string, error) {
	// dummy implementation
	return "memory-tx", nil
}

// ItemRepo returns the item repository
func (s *storage) ItemRepo() *itemRepository {
	return s.itemRepo
}

func (s *storage) TopicRepo() *topicRepository {
	return s.topicRepo
}

// Close closes the storage connection (no-op for memory storage)
func (s *storage) Close() error {
	return nil
}

// InitializeSampleData adds sample data to the storage
func (s *storage) InitializeSampleData() int {

	sampleItems := []model.Item{
		{
			Title:       "Sample Task 1",
			Description: "This is a sample task for testing",
		},
		{
			Title:       "Sample Task 2",
			Description: "Another sample task with different content",
		},
		{
			Title:       "Sample Task 3",
			Description: "Third sample task for demonstration",
		},
	}

	for _, item := range sampleItems {
		_, _ = s.itemRepo.Create(item)
	}

	return len(sampleItems)
}
