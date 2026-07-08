package handler

import (
	"bytes"
	"database/sql"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/all-in-one/internal/authnz/handler/mocks"
	"github.com/all-in-one/internal/authnz/model"
	"github.com/all-in-one/internal/config"
	"github.com/all-in-one/internal/tester"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func adminReq(t *testing.T, method, target, id string, body []byte) *http.Request {
	t.Helper()
	var r *http.Request
	if body != nil {
		r = httptest.NewRequest(method, target, bytes.NewReader(body))
	} else {
		r = httptest.NewRequest(method, target, nil)
	}
	r = r.WithContext(tester.ContextWithLogger())
	return mux.SetURLVars(r, map[string]string{"id": id})
}

func TestUpdateUserEmail(t *testing.T) {
	uid := uuid.New()

	tests := []struct {
		name       string
		id         string
		body       string
		setup      func(*mocks.MockUserRepository)
		wantStatus int
	}{
		{
			name: "success",
			id:   uid.String(),
			body: `{"email":"new@example.com"}`,
			setup: func(m *mocks.MockUserRepository) {
				m.On("Find", mock.Anything, uid).Return(model.User{ID: uid}, nil)
				m.On("UpdateEmail", mock.Anything, uid, "new@example.com").Return(nil)
			},
			wantStatus: http.StatusOK,
		},
		{
			name:       "invalid email",
			id:         uid.String(),
			body:       `{"email":"not-an-email"}`,
			setup:      func(m *mocks.MockUserRepository) {},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "invalid user id",
			id:         "not-a-uuid",
			body:       `{"email":"new@example.com"}`,
			setup:      func(m *mocks.MockUserRepository) {},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "user not found",
			id:   uid.String(),
			body: `{"email":"new@example.com"}`,
			setup: func(m *mocks.MockUserRepository) {
				m.On("Find", mock.Anything, uid).Return(model.User{}, sql.ErrNoRows)
			},
			wantStatus: http.StatusNotFound,
		},
		{
			name: "email already in use",
			id:   uid.String(),
			body: `{"email":"taken@example.com"}`,
			setup: func(m *mocks.MockUserRepository) {
				m.On("Find", mock.Anything, uid).Return(model.User{ID: uid}, nil)
				m.On("UpdateEmail", mock.Anything, uid, "taken@example.com").
					Return(errors.New("UNIQUE constraint failed: users.email"))
			},
			wantStatus: http.StatusConflict,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			userRepo := mocks.NewMockUserRepository(t)
			tt.setup(userRepo)
			storage := &MockStorage{userRepo: userRepo, sessionRepo: mocks.NewMockSessionRepository(t)}
			h := NewHandler(storage, config.Config{})

			rr := httptest.NewRecorder()
			h.UpdateUserEmail(rr, adminReq(t, http.MethodPatch, "/admin/users/"+tt.id, tt.id, []byte(tt.body)))

			assert.Equal(t, tt.wantStatus, rr.Code)
		})
	}
}

func TestBlockUser(t *testing.T) {
	uid := uuid.New()

	t.Run("blocks a non-admin and terminates sessions", func(t *testing.T) {
		userRepo := mocks.NewMockUserRepository(t)
		userRepo.On("Find", mock.Anything, uid).Return(model.User{ID: uid}, nil)
		userRepo.On("SetBlocked", mock.Anything, uid, true).Return(nil)
		sessionRepo := mocks.NewMockSessionRepository(t)
		sessionRepo.On("DeleteByUserID", mock.Anything, uid).Return(nil)
		resolver := mocks.NewMockAccessResolver(t)
		resolver.On("EffectiveFeatures", mock.Anything, uid).Return(false, uuid.Nil, "regular-user", []string{}, nil)

		h := NewHandler(&MockStorage{userRepo: userRepo, sessionRepo: sessionRepo}, config.Config{})
		h.SetAccessResolver(resolver)

		rr := httptest.NewRecorder()
		h.BlockUser(rr, adminReq(t, http.MethodPost, "/admin/users/"+uid.String()+"/block", uid.String(), nil))

		assert.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("refuses to block an admin", func(t *testing.T) {
		userRepo := mocks.NewMockUserRepository(t)
		userRepo.On("Find", mock.Anything, uid).Return(model.User{ID: uid}, nil)
		resolver := mocks.NewMockAccessResolver(t)
		resolver.On("EffectiveFeatures", mock.Anything, uid).Return(true, uuid.Nil, "admin", []string{}, nil)

		h := NewHandler(&MockStorage{userRepo: userRepo, sessionRepo: mocks.NewMockSessionRepository(t)}, config.Config{})
		h.SetAccessResolver(resolver)

		rr := httptest.NewRecorder()
		h.BlockUser(rr, adminReq(t, http.MethodPost, "/admin/users/"+uid.String()+"/block", uid.String(), nil))

		assert.Equal(t, http.StatusConflict, rr.Code)
	})

	t.Run("user not found", func(t *testing.T) {
		userRepo := mocks.NewMockUserRepository(t)
		userRepo.On("Find", mock.Anything, uid).Return(model.User{}, sql.ErrNoRows)

		h := NewHandler(&MockStorage{userRepo: userRepo, sessionRepo: mocks.NewMockSessionRepository(t)}, config.Config{})

		rr := httptest.NewRecorder()
		h.BlockUser(rr, adminReq(t, http.MethodPost, "/admin/users/"+uid.String()+"/block", uid.String(), nil))

		assert.Equal(t, http.StatusNotFound, rr.Code)
	})
}

func TestUnblockUser(t *testing.T) {
	uid := uuid.New()

	userRepo := mocks.NewMockUserRepository(t)
	userRepo.On("Find", mock.Anything, uid).Return(model.User{ID: uid, Blocked: true}, nil)
	userRepo.On("SetBlocked", mock.Anything, uid, false).Return(nil)

	h := NewHandler(&MockStorage{userRepo: userRepo, sessionRepo: mocks.NewMockSessionRepository(t)}, config.Config{})

	rr := httptest.NewRecorder()
	h.UnblockUser(rr, adminReq(t, http.MethodPost, "/admin/users/"+uid.String()+"/unblock", uid.String(), nil))

	require.Equal(t, http.StatusOK, rr.Code)
}
