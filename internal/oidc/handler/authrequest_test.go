package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/all-in-one/internal/auth"
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

func newRouter(svc Service, loggedInAs string) *mux.Router {
	h := NewHandler(svc, config.Config{})
	r := mux.NewRouter()
	h.RegisterPublicRoutes(r)
	authed := r.NewRoute().Subrouter()
	authed.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			if loggedInAs != "" {
				req = req.WithContext(context.WithValue(req.Context(), auth.UserContextKey,
					auth.UserClaims{UserID: loggedInAs, Username: "budi"}))
			}
			next.ServeHTTP(w, req)
		})
	})
	h.RegisterAuthenticatedRoutes(authed)
	return r
}

func do(t *testing.T, r *mux.Router, method, path string, cookies ...*http.Cookie) (int, httpHelper.Response) {
	t.Helper()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, nil)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	r.ServeHTTP(rr, req)
	var resp httpHelper.Response
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	return rr.Code, resp
}

func TestGetAuthRequest(t *testing.T) {
	cases := []struct {
		name   string
		info   model.AuthRequestInfo
		err    error
		status int
	}{
		{"found", model.AuthRequestInfo{ID: "r1", ClientID: "cashflow", ClientName: "Cashflow", Signup: true}, nil, http.StatusOK},
		{"expired", model.AuthRequestInfo{}, oidc.ErrAuthRequestNotFound, http.StatusNotFound},
		{"revoked client", model.AuthRequestInfo{}, oidc.ErrClientRevoked, http.StatusNotFound},
		{"unexpected", model.AuthRequestInfo{}, errors.New("boom"), http.StatusInternalServerError},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			svc := mocks.NewMockService(t)
			svc.EXPECT().AuthRequestInfo(mock.Anything, "r1").Return(c.info, c.err)
			status, resp := do(t, newRouter(svc, ""), http.MethodGet, "/oidc/auth-requests/r1")
			assert.Equal(t, c.status, status)
			if c.err == nil {
				data := resp.Data.(map[string]any)
				assert.Equal(t, "Cashflow", data["client_name"])
				assert.Equal(t, true, data["signup"])
			}
		})
	}
}

func TestCompleteAuthRequest(t *testing.T) {
	t.Run("not logged in", func(t *testing.T) {
		status, _ := do(t, newRouter(mocks.NewMockService(t), ""), http.MethodPost, "/oidc/auth-requests/r1/complete")
		assert.Equal(t, http.StatusUnauthorized, status)
	})

	t.Run("completes with the browser cookie and returns the next URL", func(t *testing.T) {
		browserID := strings.Repeat("A", 43) // 32 bytes, base64url
		svc := mocks.NewMockService(t)
		svc.EXPECT().CompleteAuthRequest(mock.Anything, "r1", "user-1", browserID).Return("http://aio/api/v1/oauth2/authorize/callback?id=r1", nil)
		svc.EXPECT().AuthRequestInfo(mock.Anything, "r1").Return(model.AuthRequestInfo{ClientID: "cashflow"}, nil)
		status, resp := do(t, newRouter(svc, "user-1"), http.MethodPost, "/oidc/auth-requests/r1/complete",
			&http.Cookie{Name: oidc.BrowserCookie, Value: browserID})
		require.Equal(t, http.StatusOK, status)
		assert.Equal(t, "http://aio/api/v1/oauth2/authorize/callback?id=r1", resp.Data.(map[string]any)["redirect_url"])
	})

	errs := []struct {
		name   string
		err    error
		status int
	}{
		{"started in another browser", oidc.ErrOtherBrowser, http.StatusForbidden},
		{"blocked user", oidc.ErrUserBlocked, http.StatusForbidden},
		{"demo account", oidc.ErrDemoAccount, http.StatusForbidden},
		{"expired request", oidc.ErrAuthRequestNotFound, http.StatusNotFound},
		{"revoked client", oidc.ErrClientRevoked, http.StatusNotFound},
	}
	for _, c := range errs {
		t.Run(c.name, func(t *testing.T) {
			svc := mocks.NewMockService(t)
			svc.EXPECT().CompleteAuthRequest(mock.Anything, "r1", "user-1", "").Return("", c.err)
			status, _ := do(t, newRouter(svc, "user-1"), http.MethodPost, "/oidc/auth-requests/r1/complete")
			assert.Equal(t, c.status, status)
		})
	}
}

func TestCompleteAuthRequest_ErrorsInTheRequestedLanguage(t *testing.T) {
	cases := []struct {
		acceptLanguage, want string
	}{
		{"id", "proses masuk ini dimulai di browser lain"},
		{"id-ID,id;q=0.9,en;q=0.8", "proses masuk ini dimulai di browser lain"},
		{"en", "this login was started in a different browser"},
		{"fr-FR", "this login was started in a different browser"},
		{"", "this login was started in a different browser"},
	}
	for _, c := range cases {
		t.Run(c.acceptLanguage, func(t *testing.T) {
			svc := mocks.NewMockService(t)
			svc.EXPECT().CompleteAuthRequest(mock.Anything, "r1", "user-1", "").Return("", oidc.ErrOtherBrowser)
			rr := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/oidc/auth-requests/r1/complete", nil)
			if c.acceptLanguage != "" {
				req.Header.Set("Accept-Language", c.acceptLanguage)
			}
			newRouter(svc, "user-1").ServeHTTP(rr, req)
			assert.Equal(t, http.StatusForbidden, rr.Code)
			assert.Contains(t, rr.Body.String(), c.want)
		})
	}
}

func TestAuthRequestErrors_EveryReasonInEveryLanguage(t *testing.T) {
	for reason := range authRequestErrors["en"] {
		assert.NotEmpty(t, authRequestErrors["id"][reason], "missing Indonesian message for %q", reason)
	}
	assert.Len(t, authRequestErrors["id"], len(authRequestErrors["en"]))
}
