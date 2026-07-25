package postgres

import (
	"context"
	"time"

	"github.com/all-in-one/internal/authnz/model"
	"github.com/all-in-one/internal/logging"
	"github.com/all-in-one/internal/query"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

type userRepository struct {
	db *sqlx.DB
}

func newUserRepository(db *sqlx.DB) *userRepository {
	return &userRepository{db: db}
}

// selectColumns reads email via COALESCE so a NULL row (an account with no
// email — allowed since migration 09 dropped NOT NULL to let multiple
// no-email accounts coexist under the still-active UNIQUE constraint) scans
// into model.User.Email as "" like every other caller already expects,
// instead of erroring on NULL-into-string.
const selectColumns = `id, username, COALESCE(email, '') AS email, name, last_login, password_hash,
	totp_enabled, totp_secret_encrypted, totp_verified_at, group_id, blocked, created_at, updated_at`

func (u *userRepository) GetAll(ctx context.Context) ([]model.User, error) {
	log := logging.GetLoggerFromContext(ctx)
	log.Info().Str("entity", "UserRepo").Str("action", "GetAll").Msg("Fetching all users from database")

	var users []model.User
	if err := u.db.SelectContext(ctx, &users, "SELECT "+selectColumns+" FROM users"); err != nil {
		return nil, err
	}
	return users, nil
}

func (u *userRepository) FindByUsername(ctx context.Context, username string) (model.User, error) {
	var user model.User
	if err := u.db.GetContext(ctx, &user, "SELECT "+selectColumns+" FROM users WHERE username = $1", username); err != nil {
		return model.User{}, err
	}
	return user, nil
}

func (u *userRepository) Find(ctx context.Context, id uuid.UUID) (model.User, error) {
	var user model.User
	if err := u.db.GetContext(ctx, &user, "SELECT "+selectColumns+" FROM users WHERE id = $1", id.String()); err != nil {
		return model.User{}, err
	}
	return user, nil
}

func (u *userRepository) Create(ctx context.Context, user model.User, opts ...query.QueryOptions) error {
	exec := getExecCtx(u.db, opts...)

	now := time.Now().UTC()
	user.CreatedAt = &now
	user.UpdatedAt = &now

	// NULLIF(:email, '') stores NULL for an omitted email instead of '' — the
	// UNIQUE constraint on users.email allows any number of NULLs but not
	// duplicate ''s, which self-service sign-up (optional email) would
	// otherwise collide on for every user after the first.
	_, err := exec.NamedExecContext(ctx, "INSERT INTO users (id, username, email, name, last_login, password_hash, created_at, updated_at) VALUES (:id, :username, NULLIF(:email, ''), :name, :last_login, :password_hash, :created_at, :updated_at)", user)
	return err
}

func (u *userRepository) Update(ctx context.Context, id uuid.UUID, user model.User, opts ...query.QueryOptions) error {
	log := logging.GetLoggerFromContext(ctx)
	log.Info().Str("entity", "UserRepo").Str("action", "Update").Str("user_id", id.String()).Msg("updating user in database")

	exec := getExecCtx(u.db, opts...)

	now := time.Now().UTC()
	user.UpdatedAt = &now

	_, err := exec.NamedExecContext(ctx, `UPDATE users SET email = NULLIF(:email, ''), username = :username, password_hash = :password_hash,
		totp_enabled = :totp_enabled, totp_secret_encrypted = :totp_secret_encrypted, totp_verified_at = :totp_verified_at,
		updated_at = :updated_at
		WHERE id = :id`, user)
	return err
}

func (u *userRepository) UpdateEmail(ctx context.Context, id uuid.UUID, email string) error {
	now := time.Now().UTC()
	_, err := u.db.ExecContext(ctx, "UPDATE users SET email = $1, updated_at = $2 WHERE id = $3", email, now, id.String())
	return err
}

func (u *userRepository) SetBlocked(ctx context.Context, id uuid.UUID, blocked bool) error {
	now := time.Now().UTC()
	_, err := u.db.ExecContext(ctx, "UPDATE users SET blocked = $1, updated_at = $2 WHERE id = $3", blocked, now, id.String())
	return err
}
