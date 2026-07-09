package rbac

import "github.com/all-in-one/internal/rbac/model"

// Feature keys — the canonical identifiers used throughout the RBAC system
// (DB rows, JWT-free per-request checks, frontend feature flags). New apps
// register by adding a constant + a Registry entry here; nothing else needs
// to change for the feature to become gateable (see ACCESS_MANAGEMENT_ADR.md
// ADR-007).
const (
	FeatureListing          = "listing"
	FeatureChat             = "chat"
	FeatureShortener        = "shortener"
	FeatureAccessManagement = "access-management"
)

// Built-in group names. These are seeded by Bootstrap and cannot be deleted
// or renamed through the management API (ACCESS_MANAGEMENT_ADR.md ADR-004).
const (
	GroupAdmin       = "admin"
	GroupRegularUser = "regular-user"
)

// Registry is the single source of truth for gateable app-features. Bootstrap
// syncs it into the `features` table on every server start: new entries are
// inserted (and, if non-admin-only, granted to the regular-user group);
// existing rows are left untouched to avoid clobbering prior admin edits.
var Registry = []model.Feature{
	{Key: FeatureListing, Name: "Listings", Description: "Item listing app with CRUD operations", AdminOnly: false},
	{Key: FeatureChat, Name: "Chats", Description: "Chat app with WebSocket support", AdminOnly: false},
	{Key: FeatureShortener, Name: "Shortener", Description: "URL shortener", AdminOnly: false},
	{Key: FeatureAccessManagement, Name: "Access Management", Description: "Manage groups, users, and feature access", AdminOnly: true},
}
