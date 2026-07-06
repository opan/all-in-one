package db

import (
	"context"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTransferTestDB creates an in-memory SQLite database with just the
// tables transfer.go touches (not the full app schema — sufficient to
// exercise readAll/writeAll's column lists and FK ordering without
// depending on the migration runner's repo-root-relative source path,
// which doesn't resolve under `go test`'s package-directory working dir).
func newTransferTestDB(t *testing.T) *sqlx.DB {
	t.Helper()
	db, err := sqlx.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })

	_, err = db.Exec(`PRAGMA foreign_keys = ON`)
	require.NoError(t, err)

	_, err = db.Exec(`
		CREATE TABLE users (
			id TEXT PRIMARY KEY,
			username TEXT NOT NULL UNIQUE,
			email TEXT NOT NULL UNIQUE,
			name TEXT,
			password_hash TEXT NOT NULL,
			last_login TIMESTAMP,
			created_at TIMESTAMP,
			updated_at TIMESTAMP,
			totp_enabled INTEGER NOT NULL DEFAULT 0,
			totp_secret_encrypted TEXT,
			totp_verified_at TIMESTAMP,
			group_id TEXT REFERENCES groups(id) ON DELETE SET NULL
		);

		CREATE TABLE features (
			id TEXT PRIMARY KEY,
			key TEXT NOT NULL UNIQUE,
			name TEXT NOT NULL,
			description TEXT,
			admin_only INTEGER NOT NULL DEFAULT 0,
			created_at TIMESTAMP NOT NULL,
			updated_at TIMESTAMP NOT NULL
		);

		CREATE TABLE groups (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL UNIQUE,
			description TEXT,
			is_builtin INTEGER NOT NULL DEFAULT 0,
			created_at TIMESTAMP NOT NULL,
			updated_at TIMESTAMP NOT NULL
		);

		CREATE TABLE group_features (
			group_id TEXT NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
			feature_id TEXT NOT NULL REFERENCES features(id) ON DELETE CASCADE,
			created_at TIMESTAMP NOT NULL,
			PRIMARY KEY (group_id, feature_id)
		);

		CREATE TABLE user_feature_overrides (
			user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			feature_id TEXT NOT NULL REFERENCES features(id) ON DELETE CASCADE,
			allow INTEGER NOT NULL,
			created_at TIMESTAMP NOT NULL,
			updated_at TIMESTAMP NOT NULL,
			PRIMARY KEY (user_id, feature_id)
		);

		CREATE TABLE sessions (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			created_at TIMESTAMP,
			user_agent TEXT,
			access_token_expiry INTEGER,
			refresh_token_expiry INTEGER
		);

		CREATE TABLE topics (id INTEGER PRIMARY KEY, user_id TEXT NOT NULL, name TEXT NOT NULL, description TEXT, form_schema TEXT, created_at TIMESTAMP, updated_at TIMESTAMP);
		CREATE TABLE items (id INTEGER PRIMARY KEY, topic_id INTEGER, title TEXT NOT NULL, description TEXT, form_schema_values TEXT, created_at TIMESTAMP, updated_at TIMESTAMP);
		CREATE TABLE chat_sessions (id TEXT PRIMARY KEY, participant_hash TEXT, status TEXT, created_at TIMESTAMP, updated_at TIMESTAMP, created_by TEXT);
		CREATE TABLE chat_session_participants (session_id TEXT, user_id TEXT, joined_at TIMESTAMP, left_at TIMESTAMP);
		CREATE TABLE chat_messages (id TEXT PRIMARY KEY, chat_session_id TEXT, user_id TEXT, message TEXT, created_at TIMESTAMP, sent_at TIMESTAMP);
		CREATE TABLE chat_invites (id TEXT PRIMARY KEY, batch_id TEXT, inviter_id TEXT, invitee_id TEXT, session_id TEXT, status TEXT, created_at TIMESTAMP, updated_at TIMESTAMP);
		CREATE TABLE recovery_codes (id TEXT PRIMARY KEY, user_id TEXT, code_hash TEXT, used_at TIMESTAMP, created_at TIMESTAMP);
		CREATE TABLE totp_challenges (id TEXT PRIMARY KEY, user_id TEXT, created_at TIMESTAMP, expires_at TIMESTAMP, attempts INTEGER, user_agent TEXT);
		CREATE TABLE short_links (id TEXT PRIMARY KEY, code TEXT, target_url TEXT, created_at TIMESTAMP, expires_at TIMESTAMP, is_active INTEGER, click_count INTEGER, last_accessed_at TIMESTAMP);
		CREATE TABLE short_link_owners (code TEXT, user_id TEXT);
	`)
	require.NoError(t, err)
	return db
}

// TestReadAllWriteAll_RBACTables proves the Phase 5 backfill: RBAC data
// (features, groups, group_features, user_feature_overrides, and
// users.group_id) survives a full readAll→writeAll round trip, inserted in
// an FK-safe order. Both sides are SQLite since no live PostgreSQL instance
// was available to test against (see .context/RBAC_PROGRESS.md) — direction
// "pg-to-sqlite" is passed so writeAll picks `?` placeholders and
// integer-bool encoding appropriate for the SQLite destination, exactly as
// it would for a real pg-to-sqlite transfer.
func TestReadAllWriteAll_RBACTables(t *testing.T) {
	src := newTransferTestDB(t)
	dst := newTransferTestDB(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)

	// Seed the source: two features (one admin_only), one built-in group
	// granting the non-admin feature, one user assigned to that group with
	// a feature override.
	_, err := src.Exec(`INSERT INTO features (id, key, name, description, admin_only, created_at, updated_at) VALUES
		('feat-listing', 'listing', 'Listings', 'desc', 0, ?, ?),
		('feat-admin',   'access-management', 'Access Management', NULL, 1, ?, ?)`,
		now, now, now, now)
	require.NoError(t, err)

	_, err = src.Exec(`INSERT INTO groups (id, name, description, is_builtin, created_at, updated_at) VALUES
		('grp-regular', 'regular-user', 'default', 1, ?, ?)`, now, now)
	require.NoError(t, err)

	_, err = src.Exec(`INSERT INTO users (id, username, email, password_hash, created_at, updated_at, group_id) VALUES
		('user-1', 'alice', 'alice@example.com', 'hash', ?, ?, 'grp-regular')`, now, now)
	require.NoError(t, err)

	_, err = src.Exec(`INSERT INTO group_features (group_id, feature_id, created_at) VALUES ('grp-regular', 'feat-listing', ?)`, now)
	require.NoError(t, err)

	_, err = src.Exec(`INSERT INTO user_feature_overrides (user_id, feature_id, allow, created_at, updated_at) VALUES ('user-1', 'feat-admin', 1, ?, ?)`, now, now)
	require.NoError(t, err)

	data, err := readAll(ctx, src)
	require.NoError(t, err)

	require.Len(t, data.features, 2)
	require.Len(t, data.groups, 1)
	require.Len(t, data.users, 1)
	require.Len(t, data.groupFeatures, 1)
	require.Len(t, data.userFeatureOverrides, 1)

	// The FK-sensitive assertion: this only succeeds if features+groups are
	// written before users, and group_features/user_feature_overrides after
	// users+features+groups. A wrong order fails here with a real FK error.
	total, err := writeAll(ctx, dst, data, "pg-to-sqlite", zerolog.Nop())
	require.NoError(t, err)
	assert.Equal(t, 6, total) // 2 features + 1 group + 1 user + 1 group_feature + 1 override

	var featureCount, groupCount, userCount, groupFeatureCount, overrideCount int
	require.NoError(t, dst.Get(&featureCount, "SELECT COUNT(*) FROM features"))
	require.NoError(t, dst.Get(&groupCount, "SELECT COUNT(*) FROM groups"))
	require.NoError(t, dst.Get(&userCount, "SELECT COUNT(*) FROM users"))
	require.NoError(t, dst.Get(&groupFeatureCount, "SELECT COUNT(*) FROM group_features"))
	require.NoError(t, dst.Get(&overrideCount, "SELECT COUNT(*) FROM user_feature_overrides"))
	assert.Equal(t, 2, featureCount)
	assert.Equal(t, 1, groupCount)
	assert.Equal(t, 1, userCount)
	assert.Equal(t, 1, groupFeatureCount)
	assert.Equal(t, 1, overrideCount)

	// users.group_id must survive the round trip, not be dropped or nulled.
	var groupID string
	require.NoError(t, dst.Get(&groupID, "SELECT group_id FROM users WHERE id = 'user-1'"))
	assert.Equal(t, "grp-regular", groupID)

	// Bool columns (admin_only, is_builtin, allow) must decode correctly on
	// both sides of the boolInt/boolForDst round trip.
	var adminOnly, isBuiltin, allow int
	require.NoError(t, dst.Get(&adminOnly, "SELECT admin_only FROM features WHERE id = 'feat-admin'"))
	require.NoError(t, dst.Get(&isBuiltin, "SELECT is_builtin FROM groups WHERE id = 'grp-regular'"))
	require.NoError(t, dst.Get(&allow, "SELECT allow FROM user_feature_overrides WHERE user_id = 'user-1'"))
	assert.Equal(t, 1, adminOnly)
	assert.Equal(t, 1, isBuiltin)
	assert.Equal(t, 1, allow)
}

// TestReadAllWriteAll_UserWithNullGroup proves an unassigned user (NULL
// group_id — resolves to regular-user by default, see the resolver)
// transfers cleanly rather than erroring or getting coerced to a zero value.
func TestReadAllWriteAll_UserWithNullGroup(t *testing.T) {
	src := newTransferTestDB(t)
	dst := newTransferTestDB(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)

	_, err := src.Exec(`INSERT INTO users (id, username, email, password_hash, created_at, updated_at, group_id) VALUES
		('user-2', 'bob', 'bob@example.com', 'hash', ?, ?, NULL)`, now, now)
	require.NoError(t, err)

	data, err := readAll(ctx, src)
	require.NoError(t, err)
	require.Len(t, data.users, 1)
	assert.False(t, data.users[0].GroupID.Valid)

	_, err = writeAll(ctx, dst, data, "pg-to-sqlite", zerolog.Nop())
	require.NoError(t, err)

	var groupID *string
	require.NoError(t, dst.Get(&groupID, "SELECT group_id FROM users WHERE id = 'user-2'"))
	assert.Nil(t, groupID)
}
