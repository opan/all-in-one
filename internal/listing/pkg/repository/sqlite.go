package repository

import "context"

// sqliteStorageAdapter adapts sqlite.storage to repository.Storage interface
type sqliteStorageAdapter struct {
	itemRepo    ItemRepository
	topicRepo   TopicRepository
	sessionRepo SessionRepository
	userRepo    UserRepository
	storage     baseStorage
}

func (s *sqliteStorageAdapter) ItemRepo() ItemRepository {
	return s.itemRepo
}

func (s *sqliteStorageAdapter) TopicRepo() TopicRepository {
	return s.topicRepo
}

func (s *sqliteStorageAdapter) SessionRepo() SessionRepository {
	return s.sessionRepo
}

func (s *sqliteStorageAdapter) UserRepo() UserRepository {
	return s.userRepo
}

func (s *sqliteStorageAdapter) Close() error {
	return s.storage.Close()
}

func (s *sqliteStorageAdapter) InitializeSampleData(ctx context.Context) int {
	return s.storage.InitializeSampleData(ctx)
}
