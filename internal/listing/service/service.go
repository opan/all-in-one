package service

import (
	"context"
	"fmt"

	"github.com/all-in-one/internal/config"
	"github.com/all-in-one/internal/listing/pkg/handler"
	"github.com/all-in-one/internal/listing/pkg/repository"
	"github.com/gorilla/mux"

	"github.com/rs/zerolog"
)

// Service represents the listing service
type Service struct {
	Handler *handler.Handler
	Storage repository.Storage
}

func NewService(ctx context.Context, config config.Config, log zerolog.Logger) (*Service, error) {
	store, err := repository.NewStorage(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create storage: %w", err)
	}

	h := handler.NewHandler(store, config)

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
func (s *Service) InitializeSampleData(ctx context.Context) int {
	return s.Storage.InitializeSampleData(ctx)
}

// Close closes any resources used by the service
func (s *Service) Close() error {
	return s.Storage.Close()
}
