package repository

import (
	"context"

	"github.com/all-in-one/internal/query"
	"github.com/all-in-one/internal/rbac/model"
	"github.com/google/uuid"
)

type FeatureRepository interface {
	List(ctx context.Context) ([]model.Feature, error)
	GetByKey(ctx context.Context, key string) (model.Feature, error)
	// Upsert inserts the feature if no row with the same key exists yet. It
	// deliberately does not update admin_only/name/description on an existing
	// row (see ACCESS_MANAGEMENT_ADR.md ADR-007) — Bootstrap calls this once
	// per Registry entry on every server start.
	Upsert(ctx context.Context, feature model.Feature) error
}

type GroupRepository interface {
	List(ctx context.Context) ([]model.Group, error)
	Get(ctx context.Context, id uuid.UUID) (model.Group, error)
	GetByName(ctx context.Context, name string) (model.Group, error)
	Create(ctx context.Context, group model.Group, opts ...query.QueryOptions) error
	Update(ctx context.Context, group model.Group, opts ...query.QueryOptions) error
	Delete(ctx context.Context, id uuid.UUID, opts ...query.QueryOptions) error
	// EnsureBuiltin returns the group with the given name, creating it
	// (is_builtin=true) if absent. Idempotent — safe to call on every boot.
	EnsureBuiltin(ctx context.Context, name, description string) (model.Group, error)
}

type GroupFeatureRepository interface {
	ListKeysByGroup(ctx context.Context, groupID uuid.UUID) ([]string, error)
	HasGrantByKey(ctx context.Context, groupID uuid.UUID, key string) (bool, error)
	// Grant is insert-if-absent — safe to call repeatedly (Bootstrap relies
	// on this to re-grant new non-admin features to regular-user each boot).
	Grant(ctx context.Context, groupID, featureID uuid.UUID, opts ...query.QueryOptions) error
	// ReplaceGrants atomically replaces all of a group's grants with featureIDs.
	ReplaceGrants(ctx context.Context, groupID uuid.UUID, featureIDs []uuid.UUID, opts ...query.QueryOptions) error
}

type OverrideRepository interface {
	ListByUser(ctx context.Context, userID uuid.UUID) ([]model.UserFeatureOverride, error)
	GetByKey(ctx context.Context, userID uuid.UUID, key string) (*model.UserFeatureOverride, error)
	// ReplaceForUser atomically replaces all overrides for a user.
	ReplaceForUser(ctx context.Context, userID uuid.UUID, overrides []model.UserFeatureOverride, opts ...query.QueryOptions) error
}

type UserGroupRepository interface {
	GetGroupID(ctx context.Context, userID uuid.UUID) (*uuid.UUID, error)
	AssignGroup(ctx context.Context, userID uuid.UUID, groupID *uuid.UUID, opts ...query.QueryOptions) error
	CountByGroup(ctx context.Context, groupID uuid.UUID) (int, error)
	ListUsersWithGroup(ctx context.Context) ([]model.UserAccessRow, error)
}

type Storage interface {
	FeatureRepo() FeatureRepository
	GroupRepo() GroupRepository
	GroupFeatureRepo() GroupFeatureRepository
	OverrideRepo() OverrideRepository
	UserGroupRepo() UserGroupRepository

	CreateTrx(ctx context.Context) (query.QueryOptions, error)
	Close() error
}
