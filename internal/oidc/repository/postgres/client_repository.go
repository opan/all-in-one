package postgres

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/all-in-one/internal/logging"
	"github.com/all-in-one/internal/oidc"
	"github.com/all-in-one/internal/oidc/model"
	"github.com/jmoiron/sqlx"
)

type ClientRepository struct {
	db *sqlx.DB
}

func NewClientRepository(db *sqlx.DB) *ClientRepository {
	return &ClientRepository{db: db}
}

func (r *ClientRepository) Create(ctx context.Context, c model.Client) error {
	logging.GetLoggerFromContext(ctx).Info().Str("entity", "OIDCClientRepo").Str("action", "Create").
		Str("client_id", c.ID).Msg("creating oidc client")
	_, err := r.db.NamedExecContext(ctx,
		`INSERT INTO oidc_clients (id, name, secret_hash, redirect_uris, post_logout_redirect_uris, brand_color, icon, created_at, created_by)
		VALUES (:id, :name, :secret_hash, :redirect_uris, :post_logout_redirect_uris, :brand_color, :icon, :created_at, :created_by)`, c)
	if isUniqueViolation(err) {
		return oidc.ErrClientExists
	}
	return err
}

func (r *ClientRepository) Get(ctx context.Context, id string) (model.Client, error) {
	var c model.Client
	err := r.db.GetContext(ctx, &c, "SELECT * FROM oidc_clients WHERE id = $1", id)
	if errors.Is(err, sql.ErrNoRows) {
		return model.Client{}, oidc.ErrClientNotFound
	}
	return c, err
}

func (r *ClientRepository) List(ctx context.Context) ([]model.Client, error) {
	clients := []model.Client{}
	err := r.db.SelectContext(ctx, &clients, "SELECT * FROM oidc_clients ORDER BY created_at")
	return clients, err
}

func (r *ClientRepository) Update(ctx context.Context, c model.Client) error {
	logging.GetLoggerFromContext(ctx).Info().Str("entity", "OIDCClientRepo").Str("action", "Update").
		Str("client_id", c.ID).Msg("updating oidc client")
	res, err := r.db.ExecContext(ctx,
		"UPDATE oidc_clients SET name = $1, brand_color = $2, icon = $3 WHERE id = $4 AND revoked_at IS NULL", c.Name, c.BrandColor, c.Icon, c.ID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return oidc.ErrClientNotFound
	}
	return nil
}

func (r *ClientRepository) Revoke(ctx context.Context, id string, at time.Time) error {
	logging.GetLoggerFromContext(ctx).Info().Str("entity", "OIDCClientRepo").Str("action", "Revoke").
		Str("client_id", id).Msg("revoking oidc client")
	res, err := r.db.ExecContext(ctx,
		"UPDATE oidc_clients SET revoked_at = $1 WHERE id = $2 AND revoked_at IS NULL", at.UTC(), id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return oidc.ErrClientNotFound
	}
	return nil
}

func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "unique constraint") || strings.Contains(msg, "duplicate key")
}
