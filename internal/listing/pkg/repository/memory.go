package repository

import "context"

// memoryStorageAdapter adapts memory.storage to repository.Storage interface
type memoryStorageAdapter struct {
	itemRepo  ItemRepository
	topicRepo TopicRepository
	storage   baseStorage
}

func (m *memoryStorageAdapter) ItemRepo() ItemRepository {
	return m.itemRepo
}

func (m *memoryStorageAdapter) TopicRepo() TopicRepository {
	return m.topicRepo
}

func (m *memoryStorageAdapter) Close() error {
	return m.storage.Close()
}

func (m *memoryStorageAdapter) InitializeSampleData(ctx context.Context) int {
	return m.storage.InitializeSampleData(ctx)
}
