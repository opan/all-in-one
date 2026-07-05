package service

import (
	"context"
	"testing"

	authnzMocks "github.com/all-in-one/internal/authnz/handler/mocks"
	authnzModel "github.com/all-in-one/internal/authnz/model"
	"github.com/all-in-one/internal/config"
	"github.com/all-in-one/internal/rbac"
	"github.com/all-in-one/internal/rbac/model"
	"github.com/all-in-one/internal/rbac/repository"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func newBootstrapTestDB(t *testing.T) *sqlx.DB {
	t.Helper()
	db, err := sqlx.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })

	_, err = db.Exec(`PRAGMA foreign_keys = ON`)
	require.NoError(t, err)

	_, err = db.Exec(`
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

		CREATE TABLE users (
			id TEXT PRIMARY KEY,
			username TEXT NOT NULL UNIQUE,
			email TEXT NOT NULL UNIQUE,
			group_id TEXT REFERENCES groups(id) ON DELETE SET NULL
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
	`)
	require.NoError(t, err)
	return db
}

func newBootstrapTestStore(t *testing.T, db *sqlx.DB) repository.Storage {
	t.Helper()
	store, err := repository.NewRepo(db, config.Config{Storage: config.StorageConfig{Type: "sqlite"}})
	require.NoError(t, err)
	return store
}

func TestBootstrap_FreshInstall(t *testing.T) {
	db := newBootstrapTestDB(t)
	store := newBootstrapTestStore(t, db)
	ctx := context.Background()

	adminUserID := uuid.New()
	_, err := db.Exec(`INSERT INTO users (id, username, email) VALUES (?, ?, ?)`, adminUserID.String(), "admin", "admin@example.com")
	require.NoError(t, err)

	mockUserRepo := authnzMocks.NewMockUserRepository(t)
	mockUserRepo.On("FindByUsername", mock.Anything, "admin").
		Return(authnzModel.User{ID: adminUserID, Username: "admin"}, nil)

	require.NoError(t, Bootstrap(ctx, store, mockUserRepo, "admin"))

	adminGroup, err := store.GroupRepo().GetByName(ctx, rbac.GroupAdmin)
	require.NoError(t, err)
	assert.True(t, adminGroup.IsBuiltin)

	regularGroup, err := store.GroupRepo().GetByName(ctx, rbac.GroupRegularUser)
	require.NoError(t, err)
	assert.True(t, regularGroup.IsBuiltin)

	// regular-user is granted every non-admin-only feature; admin group gets
	// no grants (its access comes from resolver bypass, not group_features).
	regularKeys, err := store.GroupFeatureRepo().ListKeysByGroup(ctx, regularGroup.ID)
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{rbac.FeatureListing, rbac.FeatureChat, rbac.FeatureShortener}, regularKeys)

	adminKeys, err := store.GroupFeatureRepo().ListKeysByGroup(ctx, adminGroup.ID)
	require.NoError(t, err)
	assert.Empty(t, adminKeys)

	// The configured admin username was assigned to the admin group.
	groupID, err := store.UserGroupRepo().GetGroupID(ctx, adminUserID)
	require.NoError(t, err)
	require.NotNil(t, groupID)
	assert.Equal(t, adminGroup.ID, *groupID)

	count, err := store.UserGroupRepo().CountByGroup(ctx, adminGroup.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

func TestBootstrap_SecondRunIsNoOp(t *testing.T) {
	db := newBootstrapTestDB(t)
	store := newBootstrapTestStore(t, db)
	ctx := context.Background()

	adminUserID := uuid.New()
	_, err := db.Exec(`INSERT INTO users (id, username, email) VALUES (?, ?, ?)`, adminUserID.String(), "admin", "admin@example.com")
	require.NoError(t, err)

	mockUserRepo := authnzMocks.NewMockUserRepository(t)
	mockUserRepo.On("FindByUsername", mock.Anything, "admin").
		Return(authnzModel.User{ID: adminUserID, Username: "admin"}, nil)

	require.NoError(t, Bootstrap(ctx, store, mockUserRepo, "admin"))

	regularGroup, err := store.GroupRepo().GetByName(ctx, rbac.GroupRegularUser)
	require.NoError(t, err)
	keysAfterFirst, err := store.GroupFeatureRepo().ListKeysByGroup(ctx, regularGroup.ID)
	require.NoError(t, err)

	adminGroup, err := store.GroupRepo().GetByName(ctx, rbac.GroupAdmin)
	require.NoError(t, err)

	// Second run must not duplicate groups/grants, nor reassign/duplicate
	// the admin membership (the admin group is no longer empty).
	require.NoError(t, Bootstrap(ctx, store, mockUserRepo, "admin"))

	groups, err := store.GroupRepo().List(ctx)
	require.NoError(t, err)
	assert.Len(t, groups, 2, "bootstrap must not create duplicate built-in groups")

	keysAfterSecond, err := store.GroupFeatureRepo().ListKeysByGroup(ctx, regularGroup.ID)
	require.NoError(t, err)
	assert.ElementsMatch(t, keysAfterFirst, keysAfterSecond)

	count, err := store.UserGroupRepo().CountByGroup(ctx, adminGroup.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, count, "admin membership must not be duplicated or reassigned")
}

// TestBootstrap_NewNonAdminFeatureAutoGrantedOnNextRun proves the mechanism
// that makes ADR-005 hold as new apps register features over time: any
// non-admin-only row present in the features table — regardless of whether
// it arrived via the Registry sync step or some other means — is granted to
// regular-user the next time Bootstrap runs.
func TestBootstrap_NewNonAdminFeatureAutoGrantedOnNextRun(t *testing.T) {
	db := newBootstrapTestDB(t)
	store := newBootstrapTestStore(t, db)
	ctx := context.Background()

	adminUserID := uuid.New()
	_, err := db.Exec(`INSERT INTO users (id, username, email) VALUES (?, ?, ?)`, adminUserID.String(), "admin", "admin@example.com")
	require.NoError(t, err)

	mockUserRepo := authnzMocks.NewMockUserRepository(t)
	mockUserRepo.On("FindByUsername", mock.Anything, "admin").
		Return(authnzModel.User{ID: adminUserID, Username: "admin"}, nil)

	require.NoError(t, Bootstrap(ctx, store, mockUserRepo, "admin"))

	// Simulate a newly shipped, non-admin feature appearing between boots.
	newFeature := model.Feature{ID: uuid.New(), Key: "new-app", Name: "New App", AdminOnly: false}
	require.NoError(t, store.FeatureRepo().Upsert(ctx, newFeature))

	require.NoError(t, Bootstrap(ctx, store, mockUserRepo, "admin"))

	regularGroup, err := store.GroupRepo().GetByName(ctx, rbac.GroupRegularUser)
	require.NoError(t, err)
	keys, err := store.GroupFeatureRepo().ListKeysByGroup(ctx, regularGroup.ID)
	require.NoError(t, err)
	assert.Contains(t, keys, "new-app")
}

func TestBootstrap_AdminUsernameNotFound_DoesNotError(t *testing.T) {
	db := newBootstrapTestDB(t)
	store := newBootstrapTestStore(t, db)
	ctx := context.Background()

	mockUserRepo := authnzMocks.NewMockUserRepository(t)
	mockUserRepo.On("FindByUsername", mock.Anything, "admin").
		Return(authnzModel.User{}, assert.AnError)

	// A missing configured admin user is logged, not fatal — the operator
	// can assign an admin manually once the account exists.
	require.NoError(t, Bootstrap(ctx, store, mockUserRepo, "admin"))

	adminGroup, err := store.GroupRepo().GetByName(ctx, rbac.GroupAdmin)
	require.NoError(t, err)
	count, err := store.UserGroupRepo().CountByGroup(ctx, adminGroup.ID)
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}
