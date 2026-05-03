package repository

import (
	"context"

	"github.com/all-in-one/internal/authnz/model"
	"github.com/all-in-one/internal/query"
	"github.com/google/uuid"
)

type SessionRepository interface {
	CreateTrx(ctx context.Context) (query.QueryOptions, error)
	Create(ctx context.Context, session model.Session, opts ...query.QueryOptions) error
	Delete(ctx context.Context, id uuid.UUID) error
	DeleteByUserID(ctx context.Context, userID uuid.UUID) error
	DeleteByUserIDExcept(ctx context.Context, userID uuid.UUID, exceptSessionID uuid.UUID) error
	Get(ctx context.Context, id uuid.UUID) (*model.Session, error)
}

type UserRepository interface {
	GetAll(ctx context.Context) ([]model.User, error)
	FindByUsername(ctx context.Context, username string) (model.User, error)
	Find(ctx context.Context, id uuid.UUID) (model.User, error)
	Create(ctx context.Context, user model.User, opts ...query.QueryOptions) error
	Update(ctx context.Context, id uuid.UUID, user model.User, opts ...query.QueryOptions) error
}

type TOTPRepository interface {
	CreateChallenge(ctx context.Context, challenge model.TOTPChallenge) error
	GetChallenge(ctx context.Context, id uuid.UUID) (*model.TOTPChallenge, error)
	IncrementChallengeAttempts(ctx context.Context, id uuid.UUID) error
	DeleteChallenge(ctx context.Context, id uuid.UUID) error
	DeleteExpiredChallenges(ctx context.Context) error

	CreateRecoveryCodes(ctx context.Context, codes []model.RecoveryCode) error
	GetUnusedRecoveryCodes(ctx context.Context, userID uuid.UUID) ([]model.RecoveryCode, error)
	MarkRecoveryCodeUsed(ctx context.Context, id uuid.UUID) error
	DeleteRecoveryCodesByUserID(ctx context.Context, userID uuid.UUID) error
}

type Storage interface {
	SessionRepo() SessionRepository
	UserRepo() UserRepository
	TOTPRepo() TOTPRepository

	Close() error
}
