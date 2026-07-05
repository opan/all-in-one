package service

import (
	"context"
	"testing"

	"github.com/all-in-one/internal/query"
	"github.com/all-in-one/internal/rbac"
	"github.com/all-in-one/internal/rbac/model"
	"github.com/all-in-one/internal/rbac/repository"
	"github.com/all-in-one/internal/rbac/service/mocks"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// testStorage wraps mocked sub-repositories behind the repository.Storage
// interface, mirroring the MockStorage pattern used in authnz/handler tests.
type testStorage struct {
	featureRepo      repository.FeatureRepository
	groupRepo        repository.GroupRepository
	groupFeatureRepo repository.GroupFeatureRepository
	overrideRepo     repository.OverrideRepository
	userGroupRepo    repository.UserGroupRepository
}

func (s *testStorage) FeatureRepo() repository.FeatureRepository           { return s.featureRepo }
func (s *testStorage) GroupRepo() repository.GroupRepository               { return s.groupRepo }
func (s *testStorage) GroupFeatureRepo() repository.GroupFeatureRepository { return s.groupFeatureRepo }
func (s *testStorage) OverrideRepo() repository.OverrideRepository         { return s.overrideRepo }
func (s *testStorage) UserGroupRepo() repository.UserGroupRepository      { return s.userGroupRepo }
func (s *testStorage) CreateTrx(ctx context.Context) (query.QueryOptions, error) {
	return nil, nil
}
func (s *testStorage) Close() error { return nil }

// TestResolver_CanAccess exercises the full precedence matrix documented in
// docs/adr/ACCESS_MANAGEMENT_ADR.md ADR-002:
//
//	admin bypass > user override > group grant > default-deny
func TestResolver_CanAccess(t *testing.T) {
	type group string
	const (
		groupAdmin      group = "admin"
		groupCustom     group = "custom"
		groupUnassigned group = "" // NULL group_id
	)

	tests := []struct {
		name        string
		group       group
		featureKey  string
		override    *bool // nil = no override row
		groupGrants bool
		want        bool
	}{
		{
			name:       "admin bypass ignores a deny override",
			group:      groupAdmin,
			featureKey: rbac.FeatureListing,
			// Admin short-circuits before the override/grant lookup even runs
			// (see mock setup below) — this is what "bypass" means.
			want: true,
		},
		{
			name:       "admin can access an admin-only feature",
			group:      groupAdmin,
			featureKey: rbac.FeatureAccessManagement,
			want:       true,
		},
		{
			name:        "override allow beats a missing group grant",
			group:       groupCustom,
			featureKey:  rbac.FeatureListing,
			override:    boolPtr(true),
			groupGrants: false,
			want:        true,
		},
		{
			name:        "override deny beats an existing group grant",
			group:       groupCustom,
			featureKey:  rbac.FeatureListing,
			override:    boolPtr(false),
			groupGrants: true,
			want:        false,
		},
		{
			name:        "group grant allows access with no override",
			group:       groupCustom,
			featureKey:  rbac.FeatureListing,
			override:    nil,
			groupGrants: true,
			want:        true,
		},
		{
			name:        "no grant and no override denies access",
			group:       groupCustom,
			featureKey:  rbac.FeatureListing,
			override:    nil,
			groupGrants: false,
			want:        false,
		},
		{
			name:        "non-admin cannot access an admin-only feature that was never granted",
			group:       groupCustom,
			featureKey:  rbac.FeatureAccessManagement,
			override:    nil,
			groupGrants: false,
			want:        false,
		},
		{
			name:        "unassigned user (NULL group) falls back to regular-user grant",
			group:       groupUnassigned,
			featureKey:  rbac.FeatureListing,
			override:    nil,
			groupGrants: true,
			want:        true,
		},
		{
			name:        "unassigned user (NULL group) falls back to regular-user default-deny",
			group:       groupUnassigned,
			featureKey:  rbac.FeatureListing,
			override:    nil,
			groupGrants: false,
			want:        false,
		},
		{
			name:        "unknown feature key denies rather than errors",
			group:       groupCustom,
			featureKey:  "totally-made-up-feature",
			override:    nil,
			groupGrants: false,
			want:        false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			adminGroupID := uuid.New()
			regularGroupID := uuid.New()
			customGroupID := uuid.New()
			userID := uuid.New()

			var userGroupID *uuid.UUID
			switch tc.group {
			case groupAdmin:
				id := adminGroupID
				userGroupID = &id
			case groupCustom:
				id := customGroupID
				userGroupID = &id
			case groupUnassigned:
				userGroupID = nil
			}

			groupRepo := mocks.NewMockGroupRepository(t)
			groupRepo.On("GetByName", mock.Anything, rbac.GroupAdmin).
				Return(model.Group{ID: adminGroupID, Name: rbac.GroupAdmin}, nil)
			groupRepo.On("GetByName", mock.Anything, rbac.GroupRegularUser).
				Return(model.Group{ID: regularGroupID, Name: rbac.GroupRegularUser}, nil)

			userGroupRepo := mocks.NewMockUserGroupRepository(t)
			userGroupRepo.On("GetGroupID", mock.Anything, userID).Return(userGroupID, nil)

			overrideRepo := mocks.NewMockOverrideRepository(t)
			groupFeatureRepo := mocks.NewMockGroupFeatureRepository(t)

			// Admin bypass returns before the override/grant lookup is ever
			// reached, so deliberately do NOT stub those calls for the admin
			// case — if a future change accidentally checked overrides
			// before the admin bypass, the mock would panic on an
			// unexpected call, failing the test loudly.
			if tc.group != groupAdmin {
				var overridePtr *model.UserFeatureOverride
				if tc.override != nil {
					overridePtr = &model.UserFeatureOverride{Allow: *tc.override}
				}
				overrideRepo.On("GetByKey", mock.Anything, userID, tc.featureKey).Return(overridePtr, nil)

				if overridePtr == nil {
					groupFeatureRepo.On("HasGrantByKey", mock.Anything, mock.Anything, tc.featureKey).Return(tc.groupGrants, nil)
				}
			}

			store := &testStorage{
				groupRepo:        groupRepo,
				userGroupRepo:    userGroupRepo,
				overrideRepo:     overrideRepo,
				groupFeatureRepo: groupFeatureRepo,
			}

			resolver := NewResolver(store)
			got, err := resolver.CanAccess(context.Background(), userID, tc.featureKey)
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestResolver_IsAdmin(t *testing.T) {
	adminGroupID := uuid.New()
	regularGroupID := uuid.New()
	adminUser := uuid.New()
	regularUser := uuid.New()

	groupRepo := mocks.NewMockGroupRepository(t)
	groupRepo.On("GetByName", mock.Anything, rbac.GroupAdmin).
		Return(model.Group{ID: adminGroupID, Name: rbac.GroupAdmin}, nil)
	groupRepo.On("GetByName", mock.Anything, rbac.GroupRegularUser).
		Return(model.Group{ID: regularGroupID, Name: rbac.GroupRegularUser}, nil)

	userGroupRepo := mocks.NewMockUserGroupRepository(t)
	userGroupRepo.On("GetGroupID", mock.Anything, adminUser).Return(&adminGroupID, nil)
	userGroupRepo.On("GetGroupID", mock.Anything, regularUser).Return(nil, nil)

	store := &testStorage{groupRepo: groupRepo, userGroupRepo: userGroupRepo}
	resolver := NewResolver(store)

	isAdmin, err := resolver.IsAdmin(context.Background(), adminUser)
	require.NoError(t, err)
	assert.True(t, isAdmin)

	isAdmin, err = resolver.IsAdmin(context.Background(), regularUser)
	require.NoError(t, err)
	assert.False(t, isAdmin)
}

func TestResolver_EffectiveFeatures(t *testing.T) {
	adminGroupID := uuid.New()
	regularGroupID := uuid.New()
	userID := uuid.New()

	groupRepo := mocks.NewMockGroupRepository(t)
	groupRepo.On("GetByName", mock.Anything, rbac.GroupAdmin).
		Return(model.Group{ID: adminGroupID, Name: rbac.GroupAdmin}, nil)
	groupRepo.On("GetByName", mock.Anything, rbac.GroupRegularUser).
		Return(model.Group{ID: regularGroupID, Name: rbac.GroupRegularUser}, nil)
	groupRepo.On("Get", mock.Anything, regularGroupID).
		Return(model.Group{ID: regularGroupID, Name: rbac.GroupRegularUser}, nil)

	userGroupRepo := mocks.NewMockUserGroupRepository(t)
	userGroupRepo.On("GetGroupID", mock.Anything, userID).Return(nil, nil) // unassigned -> regular-user

	featureRepo := mocks.NewMockFeatureRepository(t)
	featureRepo.On("List", mock.Anything).Return([]model.Feature{
		{Key: rbac.FeatureListing, AdminOnly: false},
		{Key: rbac.FeatureChat, AdminOnly: false},
		{Key: rbac.FeatureAccessManagement, AdminOnly: true},
	}, nil)

	overrideRepo := mocks.NewMockOverrideRepository(t)
	overrideRepo.On("GetByKey", mock.Anything, userID, rbac.FeatureListing).Return(nil, nil)
	overrideRepo.On("GetByKey", mock.Anything, userID, rbac.FeatureChat).Return(nil, nil)

	groupFeatureRepo := mocks.NewMockGroupFeatureRepository(t)
	groupFeatureRepo.On("HasGrantByKey", mock.Anything, regularGroupID, rbac.FeatureListing).Return(true, nil)
	groupFeatureRepo.On("HasGrantByKey", mock.Anything, regularGroupID, rbac.FeatureChat).Return(false, nil)

	store := &testStorage{
		groupRepo:        groupRepo,
		userGroupRepo:    userGroupRepo,
		featureRepo:      featureRepo,
		overrideRepo:     overrideRepo,
		groupFeatureRepo: groupFeatureRepo,
	}
	resolver := NewResolver(store)

	isAdmin, groupID, groupName, keys, err := resolver.EffectiveFeatures(context.Background(), userID)
	require.NoError(t, err)
	assert.False(t, isAdmin)
	assert.Equal(t, regularGroupID, groupID)
	assert.Equal(t, rbac.GroupRegularUser, groupName)
	assert.Equal(t, []string{rbac.FeatureListing}, keys) // access-management excluded (admin-only); chat not granted
}

func boolPtr(b bool) *bool { return &b }
