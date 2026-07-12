package postgres

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

type ruleRepository struct {
	db *sqlx.DB
}

func newRuleRepository(db *sqlx.DB) *ruleRepository {
	return &ruleRepository{db: db}
}

func (r *ruleRepository) List(ctx context.Context) ([]model.Rule, error) {
	var rules []model.Rule
	if err := r.db.SelectContext(ctx, &rules, "SELECT * FROM rate_limit_rules ORDER BY target_key"); err != nil {
		return nil, err
	}
	return rules, nil
}

func (r *ruleRepository) Get(ctx context.Context, targetKey string) (model.Rule, error) {
	var rule model.Rule
	if err := r.db.GetContext(ctx, &rule, "SELECT * FROM rate_limit_rules WHERE target_key = $1", targetKey); err != nil {
		if err == sql.ErrNoRows {
			return model.Rule{}, httpHelper.ErrNotFound
		}
		return model.Rule{}, err
	}
	return rule, nil
}

func (r *ruleRepository) Seed(ctx context.Context, rule model.Rule, opts ...query.QueryOptions) error {
	log := logging.GetLoggerFromContext(ctx)
	log.Info().Str("entity", "RuleRepo").Str("action", "Seed").Str("target_key", rule.TargetKey).Msg("seeding rate limit rule")

	exec := getExecCtx(r.db, opts...)
	rule.UpdatedAt = time.Now().UTC()

	_, err := exec.NamedExecContext(ctx,
		`INSERT INTO rate_limit_rules (target_key, enabled, limit_count, window_value, window_unit, updated_at, updated_by)
		VALUES (:target_key, :enabled, :limit_count, :window_value, :window_unit, :updated_at, :updated_by)
		ON CONFLICT (target_key) DO NOTHING`,
		rule)
	return err
}

func (r *ruleRepository) Update(ctx context.Context, rule model.Rule, opts ...query.QueryOptions) error {
	log := logging.GetLoggerFromContext(ctx)
	log.Info().Str("entity", "RuleRepo").Str("action", "Update").Str("target_key", rule.TargetKey).Msg("update rate limit rule")
	return r.updateRow(ctx, rule, opts...)
}

func (r *ruleRepository) ResetToDefault(ctx context.Context, rule model.Rule, opts ...query.QueryOptions) error {
	log := logging.GetLoggerFromContext(ctx)
	log.Info().Str("entity", "RuleRepo").Str("action", "ResetToDefault").Str("target_key", rule.TargetKey).Msg("reset rate limit rule to default")
	rule.UpdatedBy = nil
	return r.updateRow(ctx, rule, opts...)
}

func (r *ruleRepository) updateRow(ctx context.Context, rule model.Rule, opts ...query.QueryOptions) error {
	exec := getExecCtx(r.db, opts...)
	rule.UpdatedAt = time.Now().UTC()

	_, err := exec.NamedExecContext(ctx,
		`UPDATE rate_limit_rules SET enabled = :enabled, limit_count = :limit_count, window_value = :window_value,
		window_unit = :window_unit, updated_at = :updated_at, updated_by = :updated_by WHERE target_key = :target_key`,
		rule)
	return err
}
