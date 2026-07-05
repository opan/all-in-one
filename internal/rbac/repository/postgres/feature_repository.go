package postgres

import (
	"context"
	"database/sql"
	"time"

	httpHelper "github.com/all-in-one/internal/http"
	"github.com/all-in-one/internal/logging"
	"github.com/all-in-one/internal/rbac/model"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

type featureRepository struct {
	db *sqlx.DB
}

func newFeatureRepository(db *sqlx.DB) *featureRepository {
	return &featureRepository{db: db}
}

func (r *featureRepository) List(ctx context.Context) ([]model.Feature, error) {
	var features []model.Feature
	if err := r.db.SelectContext(ctx, &features, "SELECT * FROM features ORDER BY key"); err != nil {
		return nil, err
	}
	return features, nil
}

func (r *featureRepository) GetByKey(ctx context.Context, key string) (model.Feature, error) {
	var feature model.Feature
	if err := r.db.GetContext(ctx, &feature, "SELECT * FROM features WHERE key = $1", key); err != nil {
		if err == sql.ErrNoRows {
			return model.Feature{}, httpHelper.ErrNotFound
		}
		return model.Feature{}, err
	}
	return feature, nil
}

func (r *featureRepository) Upsert(ctx context.Context, feature model.Feature) error {
	log := logging.GetLoggerFromContext(ctx)
	log.Info().Str("entity", "FeatureRepo").Str("action", "Upsert").Str("key", feature.Key).Msg("syncing feature")

	if feature.ID == uuid.Nil {
		feature.ID = uuid.New()
	}
	now := time.Now().UTC()
	feature.CreatedAt = &now
	feature.UpdatedAt = &now

	_, err := r.db.NamedExecContext(ctx,
		`INSERT INTO features (id, key, name, description, admin_only, created_at, updated_at)
		VALUES (:id, :key, :name, :description, :admin_only, :created_at, :updated_at)
		ON CONFLICT (key) DO NOTHING`,
		feature)
	return err
}
