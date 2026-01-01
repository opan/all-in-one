package sqlite

import (
	"context"
	"time"

	"github.com/all-in-one/internal/listing/pkg/model"
	"github.com/all-in-one/internal/listing/query"
	"github.com/all-in-one/internal/logging"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

type userRepository struct {
	db *sqlx.DB
}

func newUserRepository(db *sqlx.DB) *userRepository {
	return &userRepository{db: db}
}

func (u *userRepository) GetAll(ctx context.Context) ([]model.User, error) {
	log := logging.GetLoggerFromContext(ctx)
	log.Info().Str("entity", "UserRepo").Str("action", "GetAll").Msg("Fetching all users from database")

	var users []model.User
	if err := u.db.SelectContext(ctx, &users, "SELECT * FROM users"); err != nil {
		return nil, err
	}
	return users, nil
}

func (u *userRepository) FindByUsername(ctx context.Context, username string) (*model.User, error) {
	var user model.User
	if err := u.db.GetContext(ctx, &user, "SELECT * FROM users WHERE username = ?", username); err != nil {
		return nil, err
	}
	return &user, nil
}

func (u *userRepository) Find(ctx context.Context, id uuid.UUID) (*model.User, error) {
	var user model.User
	if err := u.db.GetContext(ctx, &user, "SELECT * FROM users WHERE id = ?", id.String()); err != nil {
		return nil, err
	}
	return &user, nil
}

func (u *userRepository) Create(ctx context.Context, user model.User, opts ...query.QueryOptions) error {
	exec := getExecCtx(u.db, opts...)

	now := time.Now().UTC()
	user.CreatedAt = &now
	user.UpdatedAt = &now

	_, err := exec.NamedExecContext(ctx, "INSERT INTO users (id, username, email, name, last_login, password_hash, created_at, updated_at) VALUES (:id, :username, :email, :name, :last_login, :password_hash, :created_at, :updated_at)", user)
	if err != nil {
		return err
	}

	return nil
}

func (u *userRepository) Update(ctx context.Context, id uuid.UUID, user model.User, opts ...query.QueryOptions) error {
	log := logging.GetLoggerFromContext(ctx)
	log.Info().Str("entity", "UserRepo").
		Str("action", "UpdatePassword").
		Str("user_id", id.String()).
		Msg("updating user password in database")

	exec := getExecCtx(u.db, opts...)

	now := time.Now().UTC()
	user.UpdatedAt = &now

	_, err := exec.NamedExecContext(ctx, `UPDATE users SET email = :email, username = :username,  password_hash = :password_hash
	, updated_at = :updated_at
	WHERE id = :id`, user)
	if err != nil {
		return err
	}

	return nil
}
