package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/all-in-one/internal/config"
	"github.com/all-in-one/internal/query"
	"github.com/all-in-one/internal/rbac"
	"github.com/all-in-one/internal/rbac/model"
	"github.com/all-in-one/internal/rbac/repository"
	"github.com/all-in-one/internal/rbac/service"
	"github.com/all-in-one/internal/rbac/service/mocks"
	"github.com/all-in-one/internal/tester"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// fakeStorage wraps mocked sub-repositories behind repository.Storage, same
// shape as the one in internal/rbac/service/resolver_test.go (duplicated
// rather than shared, consistent with how this codebase already duplicates
// small test-only helpers per-package, e.g. shortener's repository vs.
// handler integration tests).
type fakeStorage struct {
	featureRepo      repository.FeatureRepository
	groupRepo        repository.GroupRepository
	groupFeatureRepo repository.GroupFeatureRepository
	overrideRepo     repository.OverrideRepository
	userGroupRepo    repository.UserGroupRepository
}

func (s *fakeStorage) FeatureRepo() repository.FeatureRepository           { return s.featureRepo }
func (s *fakeStorage) GroupRepo() repository.GroupRepository               { return s.groupRepo }
func (s *fakeStorage) GroupFeatureRepo() repository.GroupFeatureRepository { return s.groupFeatureRepo }
func (s *fakeStorage) OverrideRepo() repository.OverrideRepository         { return s.overrideRepo }
func (s *fakeStorage) UserGroupRepo() repository.UserGroupRepository       { return s.userGroupRepo }
func (s *fakeStorage) CreateTrx(ctx context.Context) (query.QueryOptions, error) {
	return nil, nil
}
func (s *fakeStorage) Close() error { return nil }

// newTestResolver builds a real *service.Resolver backed by mocked
// repositories, pre-wired so the user belongs (or not) to the admin group.
func newTestResolver(t *testing.T, userID uuid.UUID, isAdmin bool) *service.Resolver {
	t.Helper()

	adminGroupID := uuid.New()
	regularGroupID := uuid.New()

	groupRepo := mocks.NewMockGroupRepository(t)
	groupRepo.On("GetByName", mock.Anything, rbac.GroupAdmin).
		Return(model.Group{ID: adminGroupID, Name: rbac.GroupAdmin}, nil)
	groupRepo.On("GetByName", mock.Anything, rbac.GroupRegularUser).
		Return(model.Group{ID: regularGroupID, Name: rbac.GroupRegularUser}, nil)

	userGroupRepo := mocks.NewMockUserGroupRepository(t)
	if isAdmin {
		userGroupRepo.On("GetGroupID", mock.Anything, userID).Return(&adminGroupID, nil)
	} else {
		userGroupRepo.On("GetGroupID", mock.Anything, userID).Return(&regularGroupID, nil)
	}

	overrideRepo := mocks.NewMockOverrideRepository(t)
	groupFeatureRepo := mocks.NewMockGroupFeatureRepository(t)
	if !isAdmin {
		// Only reached by CanAccess (RequireFeature), never by IsAdmin
		// (RequireAdmin) — admin bypass short-circuits before the
		// override/grant lookup, and IsAdmin never performs it at all.
		// .Maybe() because this same helper backs tests that exercise
		// either resolver method.
		overrideRepo.On("GetByKey", mock.Anything, userID, rbac.FeatureListing).Return(nil, nil).Maybe()
		groupFeatureRepo.On("HasGrantByKey", mock.Anything, regularGroupID, rbac.FeatureListing).Return(isAdmin, nil).Maybe()
	}

	store := &fakeStorage{
		groupRepo:        groupRepo,
		userGroupRepo:    userGroupRepo,
		overrideRepo:     overrideRepo,
		groupFeatureRepo: groupFeatureRepo,
	}
	return service.NewResolver(store)
}

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}

func TestRequireFeature_Allow(t *testing.T) {
	userID := uuid.New()
	resolver := newTestResolver(t, userID, true) // admin -> bypasses the (unstubbed) grant check
	authz := NewAuthz(resolver, config.Config{RBAC: config.RBACConfig{DirectAuthIsAdmin: true}})

	req := httptest.NewRequest(http.MethodGet, "/topics", nil)
	req = req.WithContext(tester.ContextWithUser(userID.String(), "admin", "admin@example.com", uuid.New().String()))
	rr := httptest.NewRecorder()

	authz.RequireFeature(rbac.FeatureListing)(okHandler()).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestRequireFeature_Deny(t *testing.T) {
	userID := uuid.New()
	resolver := newTestResolver(t, userID, false) // regular user, feature not granted
	authz := NewAuthz(resolver, config.Config{})

	req := httptest.NewRequest(http.MethodGet, "/topics", nil)
	req = req.WithContext(tester.ContextWithUser(userID.String(), "user", "user@example.com", uuid.New().String()))
	rr := httptest.NewRecorder()

	authz.RequireFeature(rbac.FeatureListing)(okHandler()).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusForbidden, rr.Code)
}

func TestRequireAdmin_Allow(t *testing.T) {
	userID := uuid.New()
	resolver := newTestResolver(t, userID, true)
	authz := NewAuthz(resolver, config.Config{})

	req := httptest.NewRequest(http.MethodGet, "/access/users", nil)
	req = req.WithContext(tester.ContextWithUser(userID.String(), "admin", "admin@example.com", uuid.New().String()))
	rr := httptest.NewRecorder()

	authz.RequireAdmin(okHandler()).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestRequireAdmin_Deny(t *testing.T) {
	userID := uuid.New()
	resolver := newTestResolver(t, userID, false)
	authz := NewAuthz(resolver, config.Config{})

	req := httptest.NewRequest(http.MethodGet, "/access/users", nil)
	req = req.WithContext(tester.ContextWithUser(userID.String(), "user", "user@example.com", uuid.New().String()))
	rr := httptest.NewRecorder()

	authz.RequireAdmin(okHandler()).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusForbidden, rr.Code)
}

func TestDirectAuth_AllowedWhenConfiguredAsAdmin(t *testing.T) {
	// A nil resolver proves the direct-auth branch short-circuits before any
	// resolver call — matching the production wiring where DirectAuthIsAdmin
	// defaults to true precisely because that bypass already forgoes auth.
	authz := NewAuthz(nil, config.Config{RBAC: config.RBACConfig{DirectAuthIsAdmin: true}})

	req := httptest.NewRequest(http.MethodGet, "/topics", nil)
	req = req.WithContext(tester.ContextWithUser("someuser", "someuser", "someuser@example.com", "direct-auth"))
	rr := httptest.NewRecorder()

	authz.RequireFeature(rbac.FeatureListing)(okHandler()).ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)

	rr = httptest.NewRecorder()
	authz.RequireAdmin(okHandler()).ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestDirectAuth_DeniedWhenNotConfiguredAsAdmin(t *testing.T) {
	authz := NewAuthz(nil, config.Config{RBAC: config.RBACConfig{DirectAuthIsAdmin: false}})

	req := httptest.NewRequest(http.MethodGet, "/topics", nil)
	req = req.WithContext(tester.ContextWithUser("someuser", "someuser", "someuser@example.com", "direct-auth"))
	rr := httptest.NewRecorder()

	authz.RequireFeature(rbac.FeatureListing)(okHandler()).ServeHTTP(rr, req)
	assert.Equal(t, http.StatusForbidden, rr.Code)

	rr = httptest.NewRecorder()
	authz.RequireAdmin(okHandler()).ServeHTTP(rr, req)
	assert.Equal(t, http.StatusForbidden, rr.Code)
}

func TestRequireFeature_MissingContext_Unauthorized(t *testing.T) {
	authz := NewAuthz(nil, config.Config{})

	req := httptest.NewRequest(http.MethodGet, "/topics", nil)
	rr := httptest.NewRecorder()

	authz.RequireFeature(rbac.FeatureListing)(okHandler()).ServeHTTP(rr, req)
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestRequireAdmin_MissingContext_Unauthorized(t *testing.T) {
	authz := NewAuthz(nil, config.Config{})

	req := httptest.NewRequest(http.MethodGet, "/access/users", nil)
	rr := httptest.NewRecorder()

	authz.RequireAdmin(okHandler()).ServeHTTP(rr, req)
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestRequireFeature_InvalidUserID_Unauthorized(t *testing.T) {
	authz := NewAuthz(nil, config.Config{})

	req := httptest.NewRequest(http.MethodGet, "/topics", nil)
	// A non-UUID, non-"direct-auth" user id should never occur in practice,
	// but must fail closed (401) rather than panic if it ever does.
	req = req.WithContext(tester.ContextWithUser("not-a-uuid", "user", "user@example.com", uuid.New().String()))
	rr := httptest.NewRecorder()

	authz.RequireFeature(rbac.FeatureListing)(okHandler()).ServeHTTP(rr, req)
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}
