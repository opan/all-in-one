package postgres

import (
	"context"

	"github.com/all-in-one/internal/oidc/model"
	"github.com/jmoiron/sqlx"
)

type KeyRepository struct {
	db *sqlx.DB
}

func NewKeyRepository(db *sqlx.DB) *KeyRepository {
	return &KeyRepository{db: db}
}

func (r *KeyRepository) Create(ctx context.Context, k model.SigningKey) error {
	_, err := r.db.NamedExecContext(ctx,
		`INSERT INTO oidc_signing_keys (id, algorithm, private_key_encrypted, created_at)
		VALUES (:id, :algorithm, :private_key_encrypted, :created_at)`, k)
	return err
}

func (r *KeyRepository) ListActive(ctx context.Context) ([]model.SigningKey, error) {
	keys := []model.SigningKey{}
	err := r.db.SelectContext(ctx, &keys,
		"SELECT * FROM oidc_signing_keys WHERE retired_at IS NULL ORDER BY created_at DESC")
	return keys, err
}
