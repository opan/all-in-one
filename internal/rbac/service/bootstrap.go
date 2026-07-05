package service

import (
	"context"
	"fmt"

	authnzRepo "github.com/all-in-one/internal/authnz/repository"
	"github.com/all-in-one/internal/logging"
	"github.com/all-in-one/internal/rbac"
	"github.com/all-in-one/internal/rbac/repository"
)

// Bootstrap idempotently synchronizes the code-defined feature registry into
// the database, ensures the built-in admin/regular-user groups exist and
// regular-user is granted every non-admin feature, and guarantees at least
// one admin exists. Safe — and intended — to call on every server start (see
// docs/adr/ACCESS_MANAGEMENT_ADR.md ADR-004, ADR-005, ADR-007).
func Bootstrap(ctx context.Context, store repository.Storage, userRepo authnzRepo.UserRepository, adminUsername string) error {
	log := logging.GetLoggerFromContext(ctx)

	// 1. Sync the code-defined feature registry into the DB. Existing rows
	// are left untouched (insert-if-absent by key) so a prior admin edit to
	// admin_only/name/description is never silently overwritten.
	for _, feature := range rbac.Registry {
		if err := store.FeatureRepo().Upsert(ctx, feature); err != nil {
			return fmt.Errorf("sync feature %q: %w", feature.Key, err)
		}
	}

	// 2. Ensure the built-in groups exist.
	adminGroup, err := store.GroupRepo().EnsureBuiltin(ctx, rbac.GroupAdmin, "Full access to every app-feature and Access Management")
	if err != nil {
		return fmt.Errorf("ensure admin group: %w", err)
	}
	regularGroup, err := store.GroupRepo().EnsureBuiltin(ctx, rbac.GroupRegularUser, "Default group; granted every non-admin-only feature")
	if err != nil {
		return fmt.Errorf("ensure regular-user group: %w", err)
	}

	// 3. Grant regular-user every non-admin feature (allow-all-by-default,
	// realized as data — ADR-005). Re-synced every boot so a newly added
	// non-admin feature is automatically granted without a manual migration.
	features, err := store.FeatureRepo().List(ctx)
	if err != nil {
		return fmt.Errorf("list features: %w", err)
	}
	for _, feature := range features {
		if feature.AdminOnly {
			continue
		}
		if err := store.GroupFeatureRepo().Grant(ctx, regularGroup.ID, feature.ID); err != nil {
			return fmt.Errorf("grant %q to regular-user: %w", feature.Key, err)
		}
	}

	// 4. Ensure at least one admin exists. Only acts when the admin group is
	// empty (fresh install, or recovery from a hypothetical lockout) — never
	// overrides a later manual admin assignment.
	adminCount, err := store.UserGroupRepo().CountByGroup(ctx, adminGroup.ID)
	if err != nil {
		return fmt.Errorf("count admins: %w", err)
	}
	if adminCount == 0 {
		user, err := userRepo.FindByUsername(ctx, adminUsername)
		if err != nil {
			log.Warn().Err(err).Str("username", adminUsername).
				Msg("rbac bootstrap: configured admin username not found; no admin assigned yet")
		} else {
			adminGroupID := adminGroup.ID
			if err := store.UserGroupRepo().AssignGroup(ctx, user.ID, &adminGroupID); err != nil {
				return fmt.Errorf("assign initial admin: %w", err)
			}
			log.Info().Str("username", user.Username).Msg("rbac bootstrap: assigned initial admin")
		}
	}

	log.Info().
		Int("features", len(features)).
		Str("admin_group_id", adminGroup.ID.String()).
		Str("regular_group_id", regularGroup.ID.String()).
		Msg("rbac bootstrap complete")

	return nil
}
