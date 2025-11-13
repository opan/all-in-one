package memory

import (
	"context"

	"github.com/all-in-one/internal/listing/pkg/model"
	"github.com/rs/zerolog"
)

type storage struct {
	itemRepo  *itemRepository
	topicRepo *topicRepository
	log       zerolog.Logger
}

func NewStorage() *storage {
	return &storage{
		itemRepo:  newItemRepository(),
		topicRepo: newTopicRepository(),
	}
}

func (s *storage) ItemRepo() *itemRepository {
	return s.itemRepo
}

func (s *storage) TopicRepo() *topicRepository {
	return s.topicRepo
}

func (s *storage) Close() error {
	return nil
}

func (s *storage) InitializeSampleData(ctx context.Context) int {
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
		_, _ = s.itemRepo.Create(ctx, item)
	}

	return len(sampleItems)
}
