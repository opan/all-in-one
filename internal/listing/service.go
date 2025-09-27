package listing

import (
	"github.com/all-in-one/internal/listing/pkg/handler"
	"github.com/all-in-one/internal/listing/pkg/repository"
	"github.com/gorilla/mux"
)

// Service represents the listing service
type Service struct {
	Handler *handler.Handler
	Storage repository.Storage
}

// NewMemoryService creates a new listing service with in-memory storage
func NewMemoryService() *Service {
	store, _ := repository.NewStorage("memory", "")
	h := handler.NewHandler(store)

	return &Service{
		Handler: h,
		Storage: store,
	}
}

// NewSQLiteService creates a new listing service with SQLite storage
func NewSQLiteService(dbPath string) (*Service, error) {
	store, err := repository.NewStorage("sqlite", dbPath)
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
	return s.Storage.Items().InitializeSampleData()
}

// Close closes any resources used by the service
func (s *Service) Close() error {
	return s.Storage.Close()
}
