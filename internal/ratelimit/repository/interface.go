package repository

import (
	"context"
	"time"

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
	// CreateExternal inserts a self-contained external rule (its own app,
	// name, scope, kind — no Registry entry to merge from). Returns
	// ratelimit.ErrExternalTargetExists if the target key is already taken.
	CreateExternal(ctx context.Context, rule model.Rule, opts ...query.QueryOptions) error
	// Delete removes an external rule by key. It is guarded at the SQL level
	// (WHERE is_external) so no code path can delete an internal (Registry)
	// row through it.
	Delete(ctx context.Context, targetKey string, opts ...query.QueryOptions) error
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

// TokenRepository stores service credentials (app tokens) for the external
// rate-limit API. GetByHash is the request hot path — one indexed lookup, no
// per-call logging (same reasoning as CounterRepository.IncrAndGet).
type TokenRepository interface {
	Create(ctx context.Context, t model.AppToken, opts ...query.QueryOptions) error
	// GetByHash looks a token up by its SHA-256 hash, excluding revoked tokens
	// at the SQL level so a revocation takes effect with no cache reasoning.
	// Returns httpHelper.ErrNotFound when no live token matches.
	GetByHash(ctx context.Context, tokenHash string) (model.AppToken, error)
	List(ctx context.Context) ([]model.AppToken, error)
	// Revoke soft-deletes a token (sets revoked_at); the audit row remains.
	Revoke(ctx context.Context, id string, opts ...query.QueryOptions) error
	// TouchLastUsed is best-effort last-seen tracking — its error must never
	// fail a check.
	TouchLastUsed(ctx context.Context, id string, at time.Time) error
}

type Storage interface {
	RuleRepo() RuleRepository
	CounterRepo() CounterRepository
	TokenRepo() TokenRepository

	CreateTrx(ctx context.Context) (query.QueryOptions, error)
	Close() error
}
