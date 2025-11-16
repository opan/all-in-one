package sqlite

import (
	"context"

	"github.com/all-in-one/internal/listing/pkg/model"
	"github.com/jmoiron/sqlx"
)

type userRepository struct {
	db *sqlx.DB
}

func newUserRepository(db *sqlx.DB) *userRepository {
	return &userRepository{db: db}
}

func (u *userRepository) FindByUsername(ctx context.Context, username string) (*model.User, error) {
	var user model.User
	if err := u.db.GetContext(ctx, &user, "SELECT * FROM users WHERE username = ?", username); err != nil {
		return nil, err
	}
	return &user, nil
}

func (u *userRepository) Create(ctx context.Context, user model.User) error {
	_, err := u.db.NamedExecContext(ctx, "INSERT INTO users (id, username, email, name, last_login, password_hash, salt) VALUES (:id, :username, :email, :name, :last_login, :password_hash, :salt)", user)
	if err != nil {
		return err
	}

	return nil
}
