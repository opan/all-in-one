package postgres

import (
	"context"
	"time"

	"github.com/all-in-one/internal/logging"
	"github.com/all-in-one/internal/query"
	"github.com/all-in-one/internal/rbac/model"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

type groupFeatureRepository struct {
	db *sqlx.DB
}

func newGroupFeatureRepository(db *sqlx.DB) *groupFeatureRepository {
	return &groupFeatureRepository{db: db}
}

func (r *groupFeatureRepository) ListKeysByGroup(ctx context.Context, groupID uuid.UUID) ([]string, error) {
	var keys []string
	err := r.db.SelectContext(ctx, &keys, `
		SELECT features.key FROM group_features
		JOIN features ON features.id = group_features.feature_id
		WHERE group_features.group_id = $1
		ORDER BY features.key`, groupID.String())
	if err != nil {
		return nil, err
	}
	return keys, nil
}

func (r *groupFeatureRepository) HasGrantByKey(ctx context.Context, groupID uuid.UUID, key string) (bool, error) {
	var count int
	err := r.db.GetContext(ctx, &count, `
		SELECT COUNT(*) FROM group_features
		JOIN features ON features.id = group_features.feature_id
		WHERE group_features.group_id = $1 AND features.key = $2`, groupID.String(), key)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *groupFeatureRepository) Grant(ctx context.Context, groupID, featureID uuid.UUID, opts ...query.QueryOptions) error {
	log := logging.GetLoggerFromContext(ctx)
	log.Info().Str("entity", "GroupFeatureRepo").Str("action", "Grant").
		Str("group_id", groupID.String()).Str("feature_id", featureID.String()).Msg("grant feature to group")

	exec := getExecCtx(r.db, opts...)

	now := time.Now().UTC()
	gf := model.GroupFeature{GroupID: groupID, FeatureID: featureID, CreatedAt: &now}

	_, err := exec.NamedExecContext(ctx,
		`INSERT INTO group_features (group_id, feature_id, created_at) VALUES (:group_id, :feature_id, :created_at)
		ON CONFLICT (group_id, feature_id) DO NOTHING`,
		gf)
	return err
}

func (r *groupFeatureRepository) ReplaceGrants(ctx context.Context, groupID uuid.UUID, featureIDs []uuid.UUID, opts ...query.QueryOptions) error {
	log := logging.GetLoggerFromContext(ctx)
	log.Info().Str("entity", "GroupFeatureRepo").Str("action", "ReplaceGrants").
		Str("group_id", groupID.String()).Int("count", len(featureIDs)).Msg("replace group feature grants")

	exec := getExecCtx(r.db, opts...)

	if _, err := exec.ExecContext(ctx, "DELETE FROM group_features WHERE group_id = $1", groupID.String()); err != nil {
		return err
	}

	now := time.Now().UTC()
	for _, featureID := range featureIDs {
		gf := model.GroupFeature{GroupID: groupID, FeatureID: featureID, CreatedAt: &now}
		if _, err := exec.NamedExecContext(ctx,
			`INSERT INTO group_features (group_id, feature_id, created_at) VALUES (:group_id, :feature_id, :created_at)`,
			gf); err != nil {
			return err
		}
	}
	return nil
}
