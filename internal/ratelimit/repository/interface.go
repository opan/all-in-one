package repository

import (
	"context"

	"github.com/all-in-one/internal/query"
	"github.com/all-in-one/internal/ratelimit/model"
)

type RuleRepository interface {
	List(ctx context.Context) ([]model.Rule, error)
	Get(ctx context.Context, targetKey string) (model.Rule, error)
	// Seed inserts the rule if no row for its TargetKey exists yet. It never
	// updates an existing row, so it cannot clobber a prior admin edit — the
	// service calls this once per Registry entry on every server start
	// (docs/adr/RATE_LIMITING_ADR.md ADR-003).
	Seed(ctx context.Context, rule model.Rule, opts ...query.QueryOptions) error
	Update(ctx context.Context, rule model.Rule, opts ...query.QueryOptions) error
	// ResetToDefault overwrites a rule's tunable fields (enabled/limit/window)
	// back to the given (registry-default) values, unconditionally.
	ResetToDefault(ctx context.Context, rule model.Rule, opts ...query.QueryOptions) error
}

type CounterRepository interface {
	// IncrAndGet atomically upserts the (target,bucket,day) bucket's count
	// by 1 and returns the new value, via a single race-safe
	// INSERT ... ON CONFLICT DO UPDATE ... RETURNING (never check-then-increment).
	IncrAndGet(ctx context.Context, targetKey, bucketKey, day string) (int, error)
	// DeleteForTargetDay clears a target's counters for one day (admin "reset").
	DeleteForTargetDay(ctx context.Context, targetKey, day string) error
	// DeleteOlderThan prunes counter rows for days strictly before day
	// (retention cleanup ticker) and returns the number of rows removed.
	DeleteOlderThan(ctx context.Context, day string) (int64, error)
}

type Storage interface {
	RuleRepo() RuleRepository
	CounterRepo() CounterRepository

	CreateTrx(ctx context.Context) (query.QueryOptions, error)
	Close() error
}
