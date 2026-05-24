package service

import (
	"context"
	"fmt"

	"github.com/all-in-one/internal/config"
	"github.com/all-in-one/internal/shortener/handler"
	"github.com/all-in-one/internal/shortener/repository"
	"github.com/gorilla/mux"
	"github.com/rs/zerolog"
)

type Service struct {
	Handler *handler.Handler
	Storage repository.Storage
}

func NewService(ctx context.Context, config config.Config, log zerolog.Logger) (*Service, error) {
	store, err := repository.NewStorage(ctx, config, log)
	if err != nil {
		return nil, fmt.Errorf("failed to create shortener storage: %w", err)
	}

	h := handler.NewHandler(store, config, log)

	return &Service{
		Handler: h,
		Storage: store,
	}, nil
}

func (s *Service) RegisterPublicRoutes(router *mux.Router) {
	s.Handler.RegisterPublicRoutes(router)
}

func (s *Service) RegisterAuthenticatedRoutes(router *mux.Router) {
	s.Handler.RegisterAuthenticatedRoutes(router)
}

func (s *Service) Close() error {
	return s.Storage.Close()
}
