package repository

import (
	"context"
	"time"

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
	ClearPasswordChangeRequired(ctx context.Context, id uuid.UUID) error
}

type UserRepository interface {
	GetAll(ctx context.Context) ([]model.User, error)
	FindByUsername(ctx context.Context, username string) (model.User, error)
	Find(ctx context.Context, id uuid.UUID) (model.User, error)
	Create(ctx context.Context, user model.User, opts ...query.QueryOptions) error
	Update(ctx context.Context, id uuid.UUID, user model.User, opts ...query.QueryOptions) error
	// UpdateEmail and SetBlocked are narrow admin-management updates that touch a
	// single column, deliberately separate from the full-row Update to avoid
	// clobbering unrelated fields.
	UpdateEmail(ctx context.Context, id uuid.UUID, email string) error
	SetBlocked(ctx context.Context, id uuid.UUID, blocked bool) error

	FindByVerifiedEmail(ctx context.Context, email string) (model.User, error)
	SetVerifiedEmail(ctx context.Context, id uuid.UUID, email string, verifiedAt time.Time) error
	SetPasswordHash(ctx context.Context, id uuid.UUID, hash string) error
	SetEmailPrompted(ctx context.Context, id uuid.UUID, at time.Time) error
}

// PasswordResetRepository stores temporary passwords (RFC-003 §6.4).
type PasswordResetRepository interface {
	Create(ctx context.Context, reset model.PasswordReset) error
	Active(ctx context.Context, userID uuid.UUID) ([]model.PasswordReset, error)
	MarkUsed(ctx context.Context, id uuid.UUID) (bool, error)
	RecordFailedAttempt(ctx context.Context, id uuid.UUID, maxAttempts int) error
	VoidActive(ctx context.Context, userID uuid.UUID) error
	CountSince(ctx context.Context, userID uuid.UUID, t time.Time) (int, error)
}

// EmailVerificationRepository stores pending email addresses (RFC-003 §6.2).
type EmailVerificationRepository interface {
	Create(ctx context.Context, v model.EmailVerification) error
	FindUsable(ctx context.Context, tokenHash string) (model.EmailVerification, error)
	Pending(ctx context.Context, userID uuid.UUID) (*model.EmailVerification, error)
	MarkUsed(ctx context.Context, id uuid.UUID) (bool, error)
	CountSince(ctx context.Context, userID uuid.UUID, t time.Time) (int, error)
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
	PasswordResetRepo() PasswordResetRepository
	EmailVerificationRepo() EmailVerificationRepository

	Close() error
}
