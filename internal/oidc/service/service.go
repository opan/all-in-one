package service

import (
	"github.com/all-in-one/internal/config"
	"github.com/all-in-one/internal/oidc/repository"
	"github.com/rs/zerolog"
)

// Service is aio's OpenID Connect provider (RFC-001, Option A): the client
// registry, signing keys, and the storage the zitadel/oidc provider runs on.
type Service struct {
	store  repository.Storage
	config config.Config
	log    zerolog.Logger
}
