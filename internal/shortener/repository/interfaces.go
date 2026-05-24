package repository

import (
	"context"

	"github.com/all-in-one/internal/shortener/model"
)

type ShortLinkRepository interface {
	// Create inserts the link; when ownerID is non-empty, records ownership in the same transaction.
	Create(ctx context.Context, link model.ShortLink, ownerID string) (model.ShortLink, error)

	GetByCode(ctx context.Context, code string) (model.ShortLink, error)

	// GetByCodeOwned returns ErrNotFound when the link exists but is not owned by ownerID (no 403 leak).
	GetByCodeOwned(ctx context.Context, code, ownerID string) (model.ShortLink, error)

	ListByOwner(ctx context.Context, ownerID string, page, pageSize uint32) ([]model.ShortLink, uint32, error)

	Update(ctx context.Context, link model.ShortLink, ownerID string) (model.ShortLink, error)

	Delete(ctx context.Context, code, ownerID string) error

	// IncrementClick atomically updates click_count and last_accessed_at in a single statement — no select-then-update race.
	IncrementClick(ctx context.Context, code string, now string) error
}

type Storage interface {
	ShortLinkRepo() ShortLinkRepository
	Close() error
}
