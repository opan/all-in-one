package service

import (
	"context"
	"fmt"

	"github.com/all-in-one/internal/chat/handler"
	"github.com/all-in-one/internal/chat/repository"
	"github.com/all-in-one/internal/chat/websocket"
	"github.com/all-in-one/internal/config"
	"github.com/gorilla/mux"
	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog"
)

// Service represents the chat service
type Service struct {
	Handler *handler.Handler
	Storage repository.Storage
	Hub     *websocket.Hub
}

// NewService creates a new chat service instance
func NewService(ctx context.Context, db *sqlx.DB, config config.Config, log zerolog.Logger) (*Service, error) {
	// Initialize storage
	store, err := repository.NewStorage(ctx, db, config, log)
	if err != nil {
		return nil, fmt.Errorf("failed to create storage: %w", err)
	}

	// Create WebSocket hub
	hub := websocket.NewHub(log)

	// Start hub in background
	go hub.Run()

	log.Info().Msg("WebSocket hub started")

	// Create handler
	h := handler.NewHandler(store, config, hub)

	return &Service{
		Handler: h,
		Storage: store,
		Hub:     hub,
	}, nil
}

// RegisterAuthenticatedRoutes registers authenticated routes to the given router
func (s *Service) RegisterAuthenticatedRoutes(router *mux.Router) {
	s.Handler.RegisterAuthenticatedRoutes(router)
}

// Close closes any resources used by the service
func (s *Service) Close() error {
	return s.Storage.Close()
}
