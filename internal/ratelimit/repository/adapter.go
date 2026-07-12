package repository

import (
	"context"

	"github.com/all-in-one/internal/query"
)

// trxCreator is satisfied structurally by both the sqlite and postgres
// concrete storage types (each expose a CreateTrx method with this shape).
type trxCreator interface {
	CreateTrx(ctx context.Context) (query.QueryOptions, error)
}

// storeAdapter adapts a driver-specific concrete storage (sqlite or
// postgres) to the driver-agnostic Storage interface consumed by the rest
// of the ratelimit package.
type storeAdapter struct {
	ruleRepo    RuleRepository
	counterRepo CounterRepository
	trx         trxCreator
}

func (a *storeAdapter) RuleRepo() RuleRepository       { return a.ruleRepo }
func (a *storeAdapter) CounterRepo() CounterRepository { return a.counterRepo }

func (a *storeAdapter) CreateTrx(ctx context.Context) (query.QueryOptions, error) {
	return a.trx.CreateTrx(ctx)
}

// Close is a no-op: the underlying *sqlx.DB is owned and closed by the
// top-level internal/storage.Storage, shared across all app modules.
func (a *storeAdapter) Close() error {
	return nil
}
