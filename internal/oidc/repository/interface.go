package repository

import (
	"context"
	"time"

	"github.com/all-in-one/internal/oidc/model"
)

type ClientRepository interface {
	// Create returns oidc.ErrClientExists if the client id is taken.
	Create(ctx context.Context, c model.Client) error
	// Get returns a client including revoked ones (callers check RevokedAt);
	// oidc.ErrClientNotFound if no such id.
	Get(ctx context.Context, id string) (model.Client, error)
	List(ctx context.Context) ([]model.Client, error)
	// Revoke sets revoked_at; oidc.ErrClientNotFound if no live client matches.
	Revoke(ctx context.Context, id string, at time.Time) error
}

type KeyRepository interface {
	Create(ctx context.Context, k model.SigningKey) error
	// ListActive returns keys that are not retired, newest first.
	ListActive(ctx context.Context) ([]model.SigningKey, error)
}

type Storage interface {
	ClientRepo() ClientRepository
	KeyRepo() KeyRepository
}
