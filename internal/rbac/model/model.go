package model

import (
	"time"

	"github.com/google/uuid"
)

type Feature struct {
	ID          uuid.UUID  `json:"id" db:"id"`
	Key         string     `json:"key" db:"key"`
	Name        string     `json:"name" db:"name"`
	Description string     `json:"description,omitempty" db:"description"`
	AdminOnly   bool       `json:"admin_only" db:"admin_only"`
	CreatedAt   *time.Time `json:"created_at,omitempty" db:"created_at"`
	UpdatedAt   *time.Time `json:"updated_at,omitempty" db:"updated_at"`
}

type Group struct {
	ID          uuid.UUID  `json:"id" db:"id"`
	Name        string     `json:"name" db:"name"`
	Description string     `json:"description,omitempty" db:"description"`
	IsBuiltin   bool       `json:"is_builtin" db:"is_builtin"`
	CreatedAt   *time.Time `json:"created_at,omitempty" db:"created_at"`
	UpdatedAt   *time.Time `json:"updated_at,omitempty" db:"updated_at"`

	// FeatureKeys is populated by the service layer (not scanned from the
	// groups table itself) when a caller needs a group's granted features.
	FeatureKeys []string `json:"feature_keys,omitempty" db:"-"`
}

type GroupFeature struct {
	GroupID   uuid.UUID  `json:"group_id" db:"group_id"`
	FeatureID uuid.UUID  `json:"feature_id" db:"feature_id"`
	CreatedAt *time.Time `json:"created_at,omitempty" db:"created_at"`
}

// UserFeatureOverride is a tri-state per-user grant/revoke for one feature.
// A row's absence means "inherit from group" (see resolver precedence,
// ACCESS_MANAGEMENT_ADR.md ADR-002).
type UserFeatureOverride struct {
	UserID    uuid.UUID  `json:"user_id" db:"user_id"`
	FeatureID uuid.UUID  `json:"feature_id" db:"feature_id"`
	Allow     bool       `json:"allow" db:"allow"`
	CreatedAt *time.Time `json:"created_at,omitempty" db:"created_at"`
	UpdatedAt *time.Time `json:"updated_at,omitempty" db:"updated_at"`
}

// UserAccessRow is a denormalized read model joining a user to their group,
// used by the management API's Users tab. IsAdmin is computed by the service
// layer (GroupName == the admin group's name), not scanned from the DB.
type UserAccessRow struct {
	UserID    uuid.UUID  `json:"user_id" db:"user_id"`
	Username  string     `json:"username" db:"username"`
	Email     string     `json:"email" db:"email"`
	GroupID   *uuid.UUID `json:"group_id" db:"group_id"`
	GroupName *string    `json:"group_name" db:"group_name"`
	IsAdmin   bool       `json:"is_admin" db:"-"`
}

// FeatureOverrideView is the API-facing (feature-key-based) shape of a
// per-user override, used by both the read (list) and write (replace)
// sides of the management API's overrides endpoints.
type FeatureOverrideView struct {
	FeatureKey string `json:"feature_key"`
	Allow      bool   `json:"allow"`
}
