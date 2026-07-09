package service

import (
	"context"
	"errors"
	"testing"

	httpHelper "github.com/all-in-one/internal/http"
	"github.com/all-in-one/internal/rbac"
	"github.com/all-in-one/internal/rbac/model"
	"github.com/all-in-one/internal/rbac/service/mocks"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestService_CreateGroup(t *testing.T) {
	listingID := uuid.New()

	featureRepo := mocks.NewMockFeatureRepository(t)
	featureRepo.On("GetByKey", mock.Anything, rbac.FeatureListing).
		Return(model.Feature{ID: listingID, Key: rbac.FeatureListing}, nil)

	groupRepo := mocks.NewMockGroupRepository(t)
	groupRepo.On("Create", mock.Anything, mock.MatchedBy(func(g model.Group) bool {
		return g.Name == "listing-only"
	})).Return(nil)

	groupFeatureRepo := mocks.NewMockGroupFeatureRepository(t)
	groupFeatureRepo.On("ReplaceGrants", mock.Anything, mock.Anything, []uuid.UUID{listingID}).Return(nil)

	svc := &Service{Store: &testStorage{
		featureRepo:      featureRepo,
		groupRepo:        groupRepo,
		groupFeatureRepo: groupFeatureRepo,
	}}

	group, err := svc.CreateGroup(context.Background(), "listing-only", "listing only", []string{rbac.FeatureListing})
	require.NoError(t, err)
	assert.Equal(t, "listing-only", group.Name)
	assert.Equal(t, []string{rbac.FeatureListing}, group.FeatureKeys)
}

func TestService_UpdateGroup_RejectsBuiltinRename(t *testing.T) {
	id := uuid.New()
	groupRepo := mocks.NewMockGroupRepository(t)
	groupRepo.On("Get", mock.Anything, id).
		Return(model.Group{ID: id, Name: rbac.GroupAdmin, IsBuiltin: true}, nil)
	// Update must NOT be called — no .On() registered for it, so calling it
	// would panic the mock and fail the test.

	svc := &Service{Store: &testStorage{groupRepo: groupRepo}}

	_, err := svc.UpdateGroup(context.Background(), id, "renamed-admin", "desc")
	assert.ErrorIs(t, err, rbac.ErrBuiltinGroup)
}

func TestService_UpdateGroup_AllowsBuiltinDescriptionChange(t *testing.T) {
	id := uuid.New()
	groupRepo := mocks.NewMockGroupRepository(t)
	groupRepo.On("Get", mock.Anything, id).
		Return(model.Group{ID: id, Name: rbac.GroupAdmin, Description: "old", IsBuiltin: true}, nil)
	groupRepo.On("Update", mock.Anything, mock.MatchedBy(func(g model.Group) bool {
		return g.Name == rbac.GroupAdmin && g.Description == "new description"
	})).Return(nil)

	groupFeatureRepo := mocks.NewMockGroupFeatureRepository(t)
	groupFeatureRepo.On("ListKeysByGroup", mock.Anything, id).Return([]string{}, nil)

	svc := &Service{Store: &testStorage{groupRepo: groupRepo, groupFeatureRepo: groupFeatureRepo}}

	group, err := svc.UpdateGroup(context.Background(), id, rbac.GroupAdmin, "new description")
	require.NoError(t, err)
	assert.Equal(t, "new description", group.Description)
}

func TestService_DeleteGroup_RejectsBuiltin(t *testing.T) {
	id := uuid.New()
	groupRepo := mocks.NewMockGroupRepository(t)
	groupRepo.On("Get", mock.Anything, id).
		Return(model.Group{ID: id, Name: rbac.GroupRegularUser, IsBuiltin: true}, nil)
	// Delete must NOT be called.

	svc := &Service{Store: &testStorage{groupRepo: groupRepo}}

	err := svc.DeleteGroup(context.Background(), id)
	assert.ErrorIs(t, err, rbac.ErrBuiltinGroup)
}

func TestService_DeleteGroup_AllowsCustomGroup(t *testing.T) {
	id := uuid.New()
	groupRepo := mocks.NewMockGroupRepository(t)
	groupRepo.On("Get", mock.Anything, id).
		Return(model.Group{ID: id, Name: "custom-group", IsBuiltin: false}, nil)
	groupRepo.On("Delete", mock.Anything, id).Return(nil)

	svc := &Service{Store: &testStorage{groupRepo: groupRepo}}

	err := svc.DeleteGroup(context.Background(), id)
	require.NoError(t, err)
}

func TestService_SetGroupFeatures_RejectsAdminGroup(t *testing.T) {
	id := uuid.New()
	groupRepo := mocks.NewMockGroupRepository(t)
	groupRepo.On("Get", mock.Anything, id).
		Return(model.Group{ID: id, Name: rbac.GroupAdmin, IsBuiltin: true}, nil)
	// ReplaceGrants must NOT be called.

	svc := &Service{Store: &testStorage{groupRepo: groupRepo}}

	_, err := svc.SetGroupFeatures(context.Background(), id, []string{rbac.FeatureListing})
	assert.ErrorIs(t, err, rbac.ErrBuiltinGroup)
}

func TestService_SetGroupFeatures_AllowsRegularUserGroup(t *testing.T) {
	id := uuid.New()
	listingID := uuid.New()

	groupRepo := mocks.NewMockGroupRepository(t)
	groupRepo.On("Get", mock.Anything, id).
		Return(model.Group{ID: id, Name: rbac.GroupRegularUser, IsBuiltin: true}, nil)

	featureRepo := mocks.NewMockFeatureRepository(t)
	featureRepo.On("GetByKey", mock.Anything, rbac.FeatureListing).
		Return(model.Feature{ID: listingID, Key: rbac.FeatureListing}, nil)

	groupFeatureRepo := mocks.NewMockGroupFeatureRepository(t)
	groupFeatureRepo.On("ReplaceGrants", mock.Anything, id, []uuid.UUID{listingID}, mock.Anything).Return(nil)

	svc := &Service{Store: &testStorage{
		groupRepo:        groupRepo,
		featureRepo:      featureRepo,
		groupFeatureRepo: groupFeatureRepo,
	}}

	group, err := svc.SetGroupFeatures(context.Background(), id, []string{rbac.FeatureListing})
	require.NoError(t, err)
	assert.Equal(t, []string{rbac.FeatureListing}, group.FeatureKeys)
}

func TestService_AssignUserGroup_LastAdminGuard(t *testing.T) {
	adminGroupID := uuid.New()
	regularGroupID := uuid.New()
	userID := uuid.New()

	groupRepo := mocks.NewMockGroupRepository(t)
	groupRepo.On("Get", mock.Anything, regularGroupID).
		Return(model.Group{ID: regularGroupID, Name: rbac.GroupRegularUser}, nil)
	groupRepo.On("Get", mock.Anything, adminGroupID).
		Return(model.Group{ID: adminGroupID, Name: rbac.GroupAdmin}, nil)

	userGroupRepo := mocks.NewMockUserGroupRepository(t)
	userGroupRepo.On("GetGroupID", mock.Anything, userID).Return(&adminGroupID, nil)
	userGroupRepo.On("CountByGroup", mock.Anything, adminGroupID).Return(1, nil)
	// AssignGroup must NOT be called.

	svc := &Service{Store: &testStorage{groupRepo: groupRepo, userGroupRepo: userGroupRepo}}

	err := svc.AssignUserGroup(context.Background(), userID, &regularGroupID)
	assert.ErrorIs(t, err, rbac.ErrLastAdmin)
}

func TestService_AssignUserGroup_AllowsWhenAnotherAdminRemains(t *testing.T) {
	adminGroupID := uuid.New()
	regularGroupID := uuid.New()
	userID := uuid.New()

	groupRepo := mocks.NewMockGroupRepository(t)
	groupRepo.On("Get", mock.Anything, regularGroupID).
		Return(model.Group{ID: regularGroupID, Name: rbac.GroupRegularUser}, nil)
	groupRepo.On("Get", mock.Anything, adminGroupID).
		Return(model.Group{ID: adminGroupID, Name: rbac.GroupAdmin}, nil)

	userGroupRepo := mocks.NewMockUserGroupRepository(t)
	userGroupRepo.On("GetGroupID", mock.Anything, userID).Return(&adminGroupID, nil)
	userGroupRepo.On("CountByGroup", mock.Anything, adminGroupID).Return(2, nil)
	userGroupRepo.On("AssignGroup", mock.Anything, userID, &regularGroupID).Return(nil)

	svc := &Service{Store: &testStorage{groupRepo: groupRepo, userGroupRepo: userGroupRepo}}

	err := svc.AssignUserGroup(context.Background(), userID, &regularGroupID)
	require.NoError(t, err)
}

func TestService_AssignUserGroup_AllowsReassigningNonAdmin(t *testing.T) {
	regularGroupID := uuid.New()
	listingGroupID := uuid.New()
	userID := uuid.New()

	groupRepo := mocks.NewMockGroupRepository(t)
	groupRepo.On("Get", mock.Anything, listingGroupID).
		Return(model.Group{ID: listingGroupID, Name: "listing-only"}, nil)
	groupRepo.On("Get", mock.Anything, regularGroupID).
		Return(model.Group{ID: regularGroupID, Name: rbac.GroupRegularUser}, nil)

	userGroupRepo := mocks.NewMockUserGroupRepository(t)
	userGroupRepo.On("GetGroupID", mock.Anything, userID).Return(&regularGroupID, nil)
	userGroupRepo.On("AssignGroup", mock.Anything, userID, &listingGroupID).Return(nil)
	// CountByGroup must NOT be called — the user's current group isn't admin.

	svc := &Service{Store: &testStorage{groupRepo: groupRepo, userGroupRepo: userGroupRepo}}

	err := svc.AssignUserGroup(context.Background(), userID, &listingGroupID)
	require.NoError(t, err)
}

func TestService_AssignUserGroup_NoOpReassignmentToSameAdminGroupAllowed(t *testing.T) {
	adminGroupID := uuid.New()
	userID := uuid.New()

	groupRepo := mocks.NewMockGroupRepository(t)
	groupRepo.On("Get", mock.Anything, adminGroupID).
		Return(model.Group{ID: adminGroupID, Name: rbac.GroupAdmin}, nil)

	userGroupRepo := mocks.NewMockUserGroupRepository(t)
	userGroupRepo.On("GetGroupID", mock.Anything, userID).Return(&adminGroupID, nil)
	userGroupRepo.On("AssignGroup", mock.Anything, userID, &adminGroupID).Return(nil)
	// CountByGroup must NOT be called — not moving away from admin.

	svc := &Service{Store: &testStorage{groupRepo: groupRepo, userGroupRepo: userGroupRepo}}

	err := svc.AssignUserGroup(context.Background(), userID, &adminGroupID)
	require.NoError(t, err)
}

func TestService_ListUsers_ComputesIsAdmin(t *testing.T) {
	adminName := rbac.GroupAdmin
	regularName := rbac.GroupRegularUser

	userGroupRepo := mocks.NewMockUserGroupRepository(t)
	userGroupRepo.On("ListUsersWithGroup", mock.Anything).Return([]model.UserAccessRow{
		{Username: "admin", GroupName: &adminName},
		{Username: "user", GroupName: &regularName},
		{Username: "unassigned", GroupName: nil},
	}, nil)

	svc := &Service{Store: &testStorage{userGroupRepo: userGroupRepo}}

	rows, err := svc.ListUsers(context.Background())
	require.NoError(t, err)
	require.Len(t, rows, 3)
	assert.True(t, rows[0].IsAdmin)
	assert.False(t, rows[1].IsAdmin)
	assert.False(t, rows[2].IsAdmin)
}

func TestService_ListUserOverrides_ResolvesFeatureKeys(t *testing.T) {
	userID := uuid.New()
	listingID := uuid.New()
	chatID := uuid.New()

	overrideRepo := mocks.NewMockOverrideRepository(t)
	overrideRepo.On("ListByUser", mock.Anything, userID).Return([]model.UserFeatureOverride{
		{FeatureID: listingID, Allow: true},
		{FeatureID: chatID, Allow: false},
	}, nil)

	featureRepo := mocks.NewMockFeatureRepository(t)
	featureRepo.On("List", mock.Anything).Return([]model.Feature{
		{ID: listingID, Key: rbac.FeatureListing},
		{ID: chatID, Key: rbac.FeatureChat},
	}, nil)

	svc := &Service{Store: &testStorage{overrideRepo: overrideRepo, featureRepo: featureRepo}}

	views, err := svc.ListUserOverrides(context.Background(), userID)
	require.NoError(t, err)
	assert.ElementsMatch(t, []model.FeatureOverrideView{
		{FeatureKey: rbac.FeatureListing, Allow: true},
		{FeatureKey: rbac.FeatureChat, Allow: false},
	}, views)
}

func TestService_SetUserOverrides_ResolvesFeatureKeys(t *testing.T) {
	userID := uuid.New()
	listingID := uuid.New()

	featureRepo := mocks.NewMockFeatureRepository(t)
	featureRepo.On("GetByKey", mock.Anything, rbac.FeatureListing).
		Return(model.Feature{ID: listingID, Key: rbac.FeatureListing}, nil)

	overrideRepo := mocks.NewMockOverrideRepository(t)
	overrideRepo.On("ReplaceForUser", mock.Anything, userID, []model.UserFeatureOverride{
		{FeatureID: listingID, Allow: true},
	}, mock.Anything).Return(nil)

	svc := &Service{Store: &testStorage{featureRepo: featureRepo, overrideRepo: overrideRepo}}

	err := svc.SetUserOverrides(context.Background(), userID, []model.FeatureOverrideView{
		{FeatureKey: rbac.FeatureListing, Allow: true},
	})
	require.NoError(t, err)
}

func TestService_SetUserOverrides_UnknownFeatureKeyFailsClosed(t *testing.T) {
	userID := uuid.New()

	featureRepo := mocks.NewMockFeatureRepository(t)
	featureRepo.On("GetByKey", mock.Anything, "totally-made-up").Return(model.Feature{}, httpHelper.ErrNotFound)

	svc := &Service{Store: &testStorage{featureRepo: featureRepo}}

	err := svc.SetUserOverrides(context.Background(), userID, []model.FeatureOverrideView{
		{FeatureKey: "totally-made-up", Allow: true},
	})
	assert.True(t, errors.Is(err, httpHelper.ErrNotFound))
}
