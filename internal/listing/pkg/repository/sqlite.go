package repository

import (
	"context"

	"github.com/all-in-one/internal/listing/pkg/repository/sqlite"
)

// sqliteTxAdapter adapts the sqlite storage's concrete transaction type to the interface
type sqliteTxAdapter struct {
	storage *sqlite.Storage
}

func (a *sqliteTxAdapter) CreateTx(ctx context.Context) (QueryOptions, error) {
	// The sqlite storage returns *sqlite.queryOptions which implements QueryOptions
	// Go will automatically convert it to the interface type when we return it
	return a.storage.CreateTx(ctx)
}

// sqliteStorageAdapter adapts sqlite.storage to repository.Storage interface
type sqliteStorageAdapter struct {
	itemRepo  ItemRepository
	topicRepo TopicRepository
	storage   baseStorage
	txCreator *sqliteTxAdapter
}

func (s *sqliteStorageAdapter) CreateTx(ctx context.Context) (QueryOptions, error) {
	return s.txCreator.CreateTx(ctx)
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

func (s *sqliteStorageAdapter) InitializeSampleData() int {
	return s.storage.InitializeSampleData()
}
