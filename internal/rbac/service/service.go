package service

import (
	"context"

	authnzRepo "github.com/all-in-one/internal/authnz/repository"
	"github.com/all-in-one/internal/config"
	"github.com/all-in-one/internal/rbac/repository"
	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog"
)

// Service wires the RBAC repository layer to the resolver and exposes the
// operations server.go needs at startup. The admin-only management API
// (RegisterAdminRoutes) is added on top of this in Phase 4.
type Service struct {
	Store    repository.Storage
	Resolver *Resolver

	config config.Config
}

func NewService(ctx context.Context, db *sqlx.DB, config config.Config, log zerolog.Logger) (*Service, error) {
	store, err := repository.NewRepo(db, config)
	if err != nil {
		log.Error().Err(err).Msg("Failed to initiate rbac repository")
		return nil, err
	}

	return &Service{
		Store:    store,
		Resolver: NewResolver(store),
		config:   config,
	}, nil
}

// Bootstrap runs the idempotent RBAC bootstrap (see bootstrap.go) using the
// configured admin username. Safe to call on every server start.
func (s *Service) Bootstrap(ctx context.Context, userRepo authnzRepo.UserRepository) error {
	return Bootstrap(ctx, s.Store, userRepo, s.config.RBAC.AdminUsername)
}
