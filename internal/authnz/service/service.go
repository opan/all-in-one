package service

import (
	"context"

	"github.com/all-in-one/internal/authnz/handler"
	"github.com/all-in-one/internal/authnz/repository"
	"github.com/all-in-one/internal/config"
	"github.com/gorilla/mux"
	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog"
)

type Service struct {
	Handler *handler.Handler
	Store   repository.Storage
}

func NewService(ctx context.Context, db *sqlx.DB, config config.Config, log zerolog.Logger) (*Service, error) {
	store, err := repository.NewRepo(db, config)
	if err != nil {
		log.Error().Err(err).Msg("Failed to initiate repository")
		return nil, err
	}

	handler := handler.NewHandler(store, config)

	return &Service{
		Store:   store,
		Handler: handler,
	}, nil
}

func (s *Service) RegisterPublicRoutes(router *mux.Router) {
	s.Handler.RegisterPublicRoutes(router)
}

func (s *Service) RegisterAuthenticatedRoutes(router *mux.Router) {
	s.Handler.RegisterAuthenticatedRoutes(router)
}
