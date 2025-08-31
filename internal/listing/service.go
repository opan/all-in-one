package listing

import (
	"github.com/all-in-one/internal/listing/pkg/handler"
	"github.com/all-in-one/internal/listing/pkg/model"
	"github.com/all-in-one/internal/listing/pkg/storage"
	"github.com/gorilla/mux"
)

// Service represents the listing service
type Service struct {
	Handler *handler.Handler
	Storage model.Storage
}

// NewMemoryService creates a new listing service with in-memory storage
func NewMemoryService() *Service {
	store := storage.NewMemoryStorage()
	h := handler.NewHandler(store)

	return &Service{
		Handler: h,
		Storage: store,
	}
}

// NewSQLiteService creates a new listing service with SQLite storage
func NewSQLiteService(dbPath string) (*Service, error) {
	store, err := storage.NewSQLiteStorage(dbPath)
	if err != nil {
		return nil, err
	}

	h := handler.NewHandler(store)

	return &Service{
		Handler: h,
		Storage: store,
	}, nil
}

// RegisterRoutes registers the listing routes to the given router
func (s *Service) RegisterRoutes(router *mux.Router) {
	s.Handler.RegisterRoutes(router)
}

// InitializeSampleData adds sample data to the storage
func (s *Service) InitializeSampleData() int {
	return s.Storage.InitializeSampleData()
}

// Close closes any resources used by the service
func (s *Service) Close() error {
	// Check if the storage is SQLite and needs to be closed
	if sqliteStore, ok := s.Storage.(*storage.SQLiteStorage); ok {
		return sqliteStore.Close()
	}
	return nil
}
