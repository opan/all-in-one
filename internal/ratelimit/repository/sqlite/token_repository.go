package sqlite

import (
	"context"
	"database/sql"
	"time"

	httpHelper "github.com/all-in-one/internal/http"
	"github.com/all-in-one/internal/logging"
	"github.com/all-in-one/internal/query"
	"github.com/all-in-one/internal/ratelimit/model"
	"github.com/jmoiron/sqlx"
)

type tokenRepository struct {
	db *sqlx.DB
}

func newTokenRepository(db *sqlx.DB) *tokenRepository {
	return &tokenRepository{db: db}
}

func (r *tokenRepository) Create(ctx context.Context, t model.AppToken, opts ...query.QueryOptions) error {
	log := logging.GetLoggerFromContext(ctx)
	log.Info().Str("entity", "TokenRepo").Str("action", "Create").Str("app", t.App).Msg("creating app token")

	exec := getExecCtx(r.db, opts...)
	_, err := exec.NamedExecContext(ctx,
		`INSERT INTO app_tokens (id, app, name, token_hash, token_prefix, scope_prefix, created_at, created_by, last_used_at, revoked_at)
		VALUES (:id, :app, :name, :token_hash, :token_prefix, :scope_prefix, :created_at, :created_by, :last_used_at, :revoked_at)`,
		t)
	return err
}

func (r *tokenRepository) GetByHash(ctx context.Context, tokenHash string) (model.AppToken, error) {
	var t model.AppToken
	if err := r.db.GetContext(ctx, &t,
		"SELECT * FROM app_tokens WHERE token_hash = ? AND revoked_at IS NULL", tokenHash); err != nil {
		if err == sql.ErrNoRows {
			return model.AppToken{}, httpHelper.ErrNotFound
		}
		return model.AppToken{}, err
	}
	return t, nil
}

func (r *tokenRepository) List(ctx context.Context) ([]model.AppToken, error) {
	var tokens []model.AppToken
	if err := r.db.SelectContext(ctx, &tokens, "SELECT * FROM app_tokens ORDER BY created_at DESC"); err != nil {
		return nil, err
	}
	return tokens, nil
}

func (r *tokenRepository) Revoke(ctx context.Context, id string, opts ...query.QueryOptions) error {
	log := logging.GetLoggerFromContext(ctx)
	log.Info().Str("entity", "TokenRepo").Str("action", "Revoke").Str("id", id).Msg("revoking app token")

	exec := getExecCtx(r.db, opts...)
	_, err := exec.ExecContext(ctx,
		"UPDATE app_tokens SET revoked_at = ? WHERE id = ? AND revoked_at IS NULL",
		time.Now().UTC(), id)
	return err
}

func (r *tokenRepository) TouchLastUsed(ctx context.Context, id string, at time.Time) error {
	_, err := r.db.ExecContext(ctx,
		"UPDATE app_tokens SET last_used_at = ? WHERE id = ?", at.UTC(), id)
	return err
}
