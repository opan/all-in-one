package service

import (
	"context"
	"fmt"
	"sync"

	"github.com/all-in-one/internal/rbac"
	"github.com/all-in-one/internal/rbac/repository"
	"github.com/google/uuid"
)

// Resolver implements the RBAC precedence rule (see
// docs/adr/ACCESS_MANAGEMENT_ADR.md ADR-002):
//
//	admin bypass > user override > group grant > default-deny
//
// It is safe for concurrent use — the built-in group IDs are resolved once
// (via sync.Once) and cached for the lifetime of the process.
type Resolver struct {
	store repository.Storage

	primeOnce      sync.Once
	primeErr       error
	adminGroupID   uuid.UUID
	regularGroupID uuid.UUID
}

func NewResolver(store repository.Storage) *Resolver {
	return &Resolver{store: store}
}

// primeBuiltinGroups resolves and caches the admin/regular-user group IDs on
// first use. Requires that Bootstrap has already created both groups —
// which is guaranteed in production because Bootstrap runs synchronously
// during server startup, before the HTTP server begins accepting requests.
func (r *Resolver) primeBuiltinGroups(ctx context.Context) error {
	r.primeOnce.Do(func() {
		admin, err := r.store.GroupRepo().GetByName(ctx, rbac.GroupAdmin)
		if err != nil {
			r.primeErr = fmt.Errorf("resolve admin group: %w", err)
			return
		}
		regular, err := r.store.GroupRepo().GetByName(ctx, rbac.GroupRegularUser)
		if err != nil {
			r.primeErr = fmt.Errorf("resolve regular-user group: %w", err)
			return
		}
		r.adminGroupID = admin.ID
		r.regularGroupID = regular.ID
	})
	return r.primeErr
}

// effectiveGroupID returns the user's assigned group, or the regular-user
// group if the user has no group assigned (NULL).
func (r *Resolver) effectiveGroupID(ctx context.Context, userID uuid.UUID) (uuid.UUID, error) {
	if err := r.primeBuiltinGroups(ctx); err != nil {
		return uuid.Nil, err
	}

	groupID, err := r.store.UserGroupRepo().GetGroupID(ctx, userID)
	if err != nil {
		return uuid.Nil, err
	}
	if groupID == nil {
		return r.regularGroupID, nil
	}
	return *groupID, nil
}

// IsAdmin reports whether the user belongs to the admin group.
func (r *Resolver) IsAdmin(ctx context.Context, userID uuid.UUID) (bool, error) {
	groupID, err := r.effectiveGroupID(ctx, userID)
	if err != nil {
		return false, err
	}
	return groupID == r.adminGroupID, nil
}

// CanAccess reports whether the user can access featureKey, applying the
// precedence rule: admin bypass > user override > group grant > default-deny.
func (r *Resolver) CanAccess(ctx context.Context, userID uuid.UUID, featureKey string) (bool, error) {
	groupID, err := r.effectiveGroupID(ctx, userID)
	if err != nil {
		return false, err
	}

	// 1. Admin bypass — unconditional, ignores overrides.
	if groupID == r.adminGroupID {
		return true, nil
	}

	// 2. Per-user override — tri-state, takes precedence over the group.
	override, err := r.store.OverrideRepo().GetByKey(ctx, userID, featureKey)
	if err != nil {
		return false, err
	}
	if override != nil {
		return override.Allow, nil
	}

	// 3. Group grant.
	granted, err := r.store.GroupFeatureRepo().HasGrantByKey(ctx, groupID, featureKey)
	if err != nil {
		return false, err
	}
	if granted {
		return true, nil
	}

	// 4. Default: deny.
	return false, nil
}

// EffectiveFeatures returns the user's admin status, group, and the set of
// non-admin-only feature keys they can currently access. Used to drive
// /users/me and frontend menu filtering (Phase 4/6).
func (r *Resolver) EffectiveFeatures(ctx context.Context, userID uuid.UUID) (isAdmin bool, groupID uuid.UUID, groupName string, featureKeys []string, err error) {
	gid, err := r.effectiveGroupID(ctx, userID)
	if err != nil {
		return false, uuid.Nil, "", nil, err
	}

	group, err := r.store.GroupRepo().Get(ctx, gid)
	if err != nil {
		return false, uuid.Nil, "", nil, err
	}

	isAdmin = gid == r.adminGroupID

	features, err := r.store.FeatureRepo().List(ctx)
	if err != nil {
		return false, uuid.Nil, "", nil, err
	}

	keys := make([]string, 0, len(features))
	for _, f := range features {
		if f.AdminOnly {
			continue
		}
		if isAdmin {
			keys = append(keys, f.Key)
			continue
		}
		ok, err := r.CanAccess(ctx, userID, f.Key)
		if err != nil {
			return false, uuid.Nil, "", nil, err
		}
		if ok {
			keys = append(keys, f.Key)
		}
	}

	return isAdmin, gid, group.Name, keys, nil
}
