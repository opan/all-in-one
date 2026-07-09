package postgres

import (
	"context"
	"time"

	"github.com/all-in-one/internal/logging"
	"github.com/jmoiron/sqlx"
)

type counterRepository struct {
	db *sqlx.DB
}

func newCounterRepository(db *sqlx.DB) *counterRepository {
	return &counterRepository{db: db}
}

// IncrAndGet is on the request hot path (evaluated per matching request),
// so unlike the rest of this package it deliberately does not log per call
// — logging every increment would itself become a flood amplifier under the
// exact traffic this repository is meant to help contain.
func (r *counterRepository) IncrAndGet(ctx context.Context, targetKey, bucketKey, day string) (int, error) {
	now := time.Now().UTC()
	var count int
	err := r.db.QueryRowxContext(ctx,
		`INSERT INTO rate_limit_counters (target_key, bucket_key, day, count, updated_at)
		VALUES ($1, $2, $3, 1, $4)
		ON CONFLICT (target_key, bucket_key, day) DO UPDATE SET count = rate_limit_counters.count + 1, updated_at = $4
		RETURNING count`,
		targetKey, bucketKey, day, now).Scan(&count)
	return count, err
}

func (r *counterRepository) DeleteForTargetDay(ctx context.Context, targetKey, day string) error {
	log := logging.GetLoggerFromContext(ctx)
	log.Info().Str("entity", "CounterRepo").Str("action", "DeleteForTargetDay").
		Str("target_key", targetKey).Str("day", day).Msg("resetting rate limit counters")

	_, err := r.db.ExecContext(ctx, "DELETE FROM rate_limit_counters WHERE target_key = $1 AND day = $2", targetKey, day)
	return err
}

func (r *counterRepository) DeleteOlderThan(ctx context.Context, day string) (int64, error) {
	res, err := r.db.ExecContext(ctx, "DELETE FROM rate_limit_counters WHERE day < $1", day)
	if err != nil {
		return 0, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, err
	}

	log := logging.GetLoggerFromContext(ctx)
	log.Info().Str("entity", "CounterRepo").Str("action", "DeleteOlderThan").
		Str("before_day", day).Int64("rows_deleted", n).Msg("pruned expired rate limit counters")

	return n, nil
}
