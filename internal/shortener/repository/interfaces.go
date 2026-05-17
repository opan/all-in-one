package repository

import (
	"context"

	"github.com/all-in-one/internal/shortener/model"
)

type ShortLinkRepository interface {
	// Create inserts the link and, when ownerID is non-empty, records ownership atomically.
	Create(ctx context.Context, link model.ShortLink, ownerID string) (model.ShortLink, error)

	// GetByCode finds a link by its short code regardless of ownership (used for redirects).
	GetByCode(ctx context.Context, code string) (model.ShortLink, error)

	// GetByCodeOwned finds a link only if the given user owns it; returns ErrNotFound otherwise.
	GetByCodeOwned(ctx context.Context, code, ownerID string) (model.ShortLink, error)

	ListByOwner(ctx context.Context, ownerID string, page, pageSize uint32) ([]model.ShortLink, uint32, error)

	// Update sets is_active and expires_at; returns ErrNotFound if code not found or not owned by ownerID.
	Update(ctx context.Context, link model.ShortLink, ownerID string) (model.ShortLink, error)

	// Delete removes the link; returns ErrNotFound if code not found or not owned by ownerID.
	Delete(ctx context.Context, code, ownerID string) error

	// IncrementClick atomically bumps click_count and sets last_accessed_at.
	// Returns ErrNotFound when no active, non-expired link matches the code.
	IncrementClick(ctx context.Context, code string, now string) error
}

type Storage interface {
	ShortLinkRepo() ShortLinkRepository
	Close() error
}
