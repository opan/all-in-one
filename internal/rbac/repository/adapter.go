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
// postgres) to the driver-agnostic Storage interface consumed by the rest of
// the rbac package.
type storeAdapter struct {
	featureRepo      FeatureRepository
	groupRepo        GroupRepository
	groupFeatureRepo GroupFeatureRepository
	overrideRepo     OverrideRepository
	userGroupRepo    UserGroupRepository
	trx              trxCreator
}

func (a *storeAdapter) FeatureRepo() FeatureRepository             { return a.featureRepo }
func (a *storeAdapter) GroupRepo() GroupRepository                 { return a.groupRepo }
func (a *storeAdapter) GroupFeatureRepo() GroupFeatureRepository   { return a.groupFeatureRepo }
func (a *storeAdapter) OverrideRepo() OverrideRepository           { return a.overrideRepo }
func (a *storeAdapter) UserGroupRepo() UserGroupRepository         { return a.userGroupRepo }

func (a *storeAdapter) CreateTrx(ctx context.Context) (query.QueryOptions, error) {
	return a.trx.CreateTrx(ctx)
}

// Close is a no-op: the underlying *sqlx.DB is owned and closed by the
// top-level internal/storage.Storage, shared across all app modules.
func (a *storeAdapter) Close() error {
	return nil
}
