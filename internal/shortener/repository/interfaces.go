package repository

import (
	"context"

	"github.com/all-in-one/internal/shortener/model"
)

type ShortLinkRepository interface {
	Create(ctx context.Context, link model.ShortLink) (model.ShortLink, error)
	GetByCode(ctx context.Context, code string) (model.ShortLink, error)
	ListByOwner(ctx context.Context, ownerID int64, page, pageSize uint32) ([]model.ShortLink, uint32, error)
	Update(ctx context.Context, link model.ShortLink) (model.ShortLink, error)
	Delete(ctx context.Context, code string, ownerID int64) error

	// IncrementClick atomically bumps click_count and sets last_accessed_at.
	// Returns ErrNotFound when no active, non-expired link matches the code.
	IncrementClick(ctx context.Context, code string, now string) error
}

type Storage interface {
	ShortLinkRepo() ShortLinkRepository
	Close() error
}
