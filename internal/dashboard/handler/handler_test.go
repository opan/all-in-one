package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/all-in-one/internal/auth"
	"github.com/all-in-one/internal/dashboard/model"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeProvider struct {
	summary model.Summary
	err     error
}

func (f fakeProvider) Summary(_ context.Context, _ uuid.UUID) (model.Summary, error) {
	return f.summary, f.err
}

func TestHandler_Summary(t *testing.T) {
	valid := auth.UserClaims{UserID: uuid.NewString(), Username: "alice"}

	tests := []struct {
		name       string
		user       *auth.UserClaims
		provider   fakeProvider
		wantStatus int
		wantOK     bool
		check      func(t *testing.T, data model.Summary)
	}{
		{
			name:       "unauthenticated returns 401",
			user:       nil,
			wantStatus: http.StatusUnauthorized,
			wantOK:     false,
		},
		{
			name:       "invalid user id returns 400",
			user:       &auth.UserClaims{UserID: "not-a-uuid"},
			wantStatus: http.StatusBadRequest,
			wantOK:     false,
		},
		{
			name:       "provider error returns 500",
			user:       &valid,
			provider:   fakeProvider{err: errors.New("boom")},
			wantStatus: http.StatusInternalServerError,
			wantOK:     false,
		},
		{
			name:       "success returns only present sections",
			user:       &valid,
			provider:   fakeProvider{summary: model.Summary{Shortener: &model.ShortenerStats{Links: 7}}},
			wantStatus: http.StatusOK,
			wantOK:     true,
			check: func(t *testing.T, data model.Summary) {
				require.NotNil(t, data.Shortener)
				assert.Equal(t, 7, data.Shortener.Links)
				assert.Nil(t, data.Listing)
				assert.Nil(t, data.Chat)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewHandler(tt.provider, zerolog.Nop())

			req := httptest.NewRequest(http.MethodGet, "/api/v1/dashboard/summary", nil)
			if tt.user != nil {
				req = req.WithContext(context.WithValue(req.Context(), auth.UserContextKey, *tt.user))
			}
			rec := httptest.NewRecorder()

			h.Summary(rec, req)

			assert.Equal(t, tt.wantStatus, rec.Code)

			var resp struct {
				Success bool          `json:"success"`
				Data    model.Summary `json:"data"`
			}
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
			assert.Equal(t, tt.wantOK, resp.Success)
			if tt.check != nil {
				tt.check(t, resp.Data)
			}
		})
	}
}
