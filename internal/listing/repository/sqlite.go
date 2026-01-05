package repository

// sqliteStorageAdapter adapts sqlite.storage to repository.Storage interface
type sqliteStorageAdapter struct {
	itemRepo  ItemRepository
	topicRepo TopicRepository
	storage   baseStorage
}

func (s *sqliteStorageAdapter) ItemRepo() ItemRepository {
	return s.itemRepo
}

func (s *sqliteStorageAdapter) TopicRepo() TopicRepository {
	return s.topicRepo
}

func (s *sqliteStorageAdapter) Close() error {
	return s.storage.Close()
}
