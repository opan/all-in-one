package postgres

import (
	"context"
	"database/sql"
	"time"

	"github.com/all-in-one/internal/logging"
	"github.com/all-in-one/internal/query"
	"github.com/all-in-one/internal/rbac/model"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

type overrideRepository struct {
	db *sqlx.DB
}

func newOverrideRepository(db *sqlx.DB) *overrideRepository {
	return &overrideRepository{db: db}
}

func (r *overrideRepository) ListByUser(ctx context.Context, userID uuid.UUID) ([]model.UserFeatureOverride, error) {
	var overrides []model.UserFeatureOverride
	err := r.db.SelectContext(ctx, &overrides, "SELECT * FROM user_feature_overrides WHERE user_id = $1", userID.String())
	if err != nil {
		return nil, err
	}
	return overrides, nil
}

// GetByKey returns nil (not an error) when no override row exists for the
// given user+feature — absence is the normal, most-common case and means
// "inherit from group" to the resolver, not a failure.
func (r *overrideRepository) GetByKey(ctx context.Context, userID uuid.UUID, key string) (*model.UserFeatureOverride, error) {
	var override model.UserFeatureOverride
	err := r.db.GetContext(ctx, &override, `
		SELECT user_feature_overrides.* FROM user_feature_overrides
		JOIN features ON features.id = user_feature_overrides.feature_id
		WHERE user_feature_overrides.user_id = $1 AND features.key = $2`, userID.String(), key)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &override, nil
}

func (r *overrideRepository) ReplaceForUser(ctx context.Context, userID uuid.UUID, overrides []model.UserFeatureOverride, opts ...query.QueryOptions) error {
	log := logging.GetLoggerFromContext(ctx)
	log.Info().Str("entity", "OverrideRepo").Str("action", "ReplaceForUser").
		Str("user_id", userID.String()).Int("count", len(overrides)).Msg("replace user feature overrides")

	exec := getExecCtx(r.db, opts...)

	if _, err := exec.ExecContext(ctx, "DELETE FROM user_feature_overrides WHERE user_id = $1", userID.String()); err != nil {
		return err
	}

	now := time.Now().UTC()
	for _, ov := range overrides {
		ov.UserID = userID
		ov.CreatedAt = &now
		ov.UpdatedAt = &now
		if _, err := exec.NamedExecContext(ctx,
			`INSERT INTO user_feature_overrides (user_id, feature_id, allow, created_at, updated_at)
			VALUES (:user_id, :feature_id, :allow, :created_at, :updated_at)`,
			ov); err != nil {
			return err
		}
	}
	return nil
}
