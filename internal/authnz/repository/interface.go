package repository

import (
	"context"

	"github.com/all-in-one/internal/authnz/model"
	"github.com/all-in-one/internal/query"
	"github.com/google/uuid"
)

type SessionRepository interface {
	Create(ctx context.Context, session model.Session, opts ...query.QueryOptions) error
	Delete(ctx context.Context, id uuid.UUID) error
	Get(ctx context.Context, id uuid.UUID) (*model.Session, error)
}

type UserRepository interface {
	GetAll(ctx context.Context) ([]model.User, error)
	FindByUsername(ctx context.Context, username string) (model.User, error)
	Find(ctx context.Context, id uuid.UUID) (model.User, error)
	Create(ctx context.Context, user model.User, opts ...query.QueryOptions) error
	Update(ctx context.Context, id uuid.UUID, user model.User, opts ...query.QueryOptions) error
}
