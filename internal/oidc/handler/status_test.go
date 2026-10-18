package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/all-in-one/internal/config"
	httpHelper "github.com/all-in-one/internal/http"
	"github.com/all-in-one/internal/oidc"
	"github.com/all-in-one/internal/oidc/handler/mocks"
	"github.com/all-in-one/internal/oidc/model"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestGetUserStatus(t *testing.T) {
	const sub = "6f1c2a0e-8f4b-4f51-9a39-0d6f3d1e2b7a"
	cases := []struct {
		name      string
		basicAuth bool
		authErr   error
		status    model.UserStatus
		statusErr error
		wantCode  int
		wantData  map[string]any
	}{
		{name: "admin", basicAuth: true, status: model.UserStatus{Admin: true}, wantCode: http.StatusOK,
			wantData: map[string]any{"admin": true, "blocked": false}},
		{name: "not admin", basicAuth: true, wantCode: http.StatusOK,
			wantData: map[string]any{"admin": false, "blocked": false}},
		{name: "blocked", basicAuth: true, status: model.UserStatus{Blocked: true}, wantCode: http.StatusOK,
			wantData: map[string]any{"admin": false, "blocked": true}},
		{name: "no credentials", wantCode: http.StatusUnauthorized},
		{name: "wrong secret", basicAuth: true, authErr: oidc.ErrInvalidClientSecret, wantCode: http.StatusUnauthorized},
		{name: "unknown client", basicAuth: true, authErr: oidc.ErrClientNotFound, wantCode: http.StatusUnauthorized},
		{name: "revoked client", basicAuth: true, authErr: oidc.ErrClientRevoked, wantCode: http.StatusUnauthorized},
		{name: "client lookup fails", basicAuth: true, authErr: errors.New("db down"), wantCode: http.StatusInternalServerError},
		{name: "unknown user", basicAuth: true, statusErr: oidc.ErrUserNotFound, wantCode: http.StatusNotFound},
		{name: "status lookup fails", basicAuth: true, statusErr: errors.New("db down"), wantCode: http.StatusInternalServerError},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc := mocks.NewMockService(t)
			if tc.basicAuth {
				svc.EXPECT().AuthenticateClient(mock.Anything, "cashflow", "s3cret").Return(tc.authErr)
				if tc.authErr == nil {
					svc.EXPECT().UserStatus(mock.Anything, sub).Return(tc.status, tc.statusErr)
				}
			}
			h := NewHandler(svc, config.Config{})
			r := mux.NewRouter()
			h.RegisterAppRoutes(r)

			req := httptest.NewRequest(http.MethodGet, "/oidc/users/"+sub+"/status", nil)
			if tc.basicAuth {
				req.SetBasicAuth("cashflow", "s3cret")
			}
			rr := httptest.NewRecorder()
			r.ServeHTTP(rr, req)

			assert.Equal(t, tc.wantCode, rr.Code)
			assert.Equal(t, "no-store", rr.Header().Get("Cache-Control"))
			if tc.wantCode == http.StatusUnauthorized {
				assert.NotEmpty(t, rr.Header().Get("WWW-Authenticate"))
			}
			var resp httpHelper.Response
			require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
			assert.Equal(t, tc.wantCode == http.StatusOK, resp.Success)
			if tc.wantData != nil {
				assert.Equal(t, tc.wantData, resp.Data)
			}
		})
	}
}
