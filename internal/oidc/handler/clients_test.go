package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/all-in-one/internal/config"
	"github.com/all-in-one/internal/oidc"
	"github.com/all-in-one/internal/oidc/handler/mocks"
	"github.com/all-in-one/internal/oidc/model"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func adminRouter(svc Service) *mux.Router {
	cfg := config.Config{}
	cfg.Auth.OIDC.Issuer = "https://auth.example.com"
	r := mux.NewRouter()
	NewHandler(svc, cfg).RegisterAdminRoutes(r)
	return r
}

func postJSON(r *mux.Router, path string, body any) *httptest.ResponseRecorder {
	raw, _ := json.Marshal(body)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, path, bytes.NewReader(raw)))
	return rr
}

func TestCreateClient_ReturnsSecretOnceWithIssuer(t *testing.T) {
	svc := mocks.NewMockService(t)
	in := model.CreateClientInput{ID: "cashflow", Name: "Cashflow", RedirectURIs: []string{"https://cf.example.com/cb"}}
	svc.EXPECT().CreateClient(mock.Anything, in, mock.Anything).
		Return(model.Client{ID: "cashflow", Name: "Cashflow", SecretHash: "stored-hash"}, "the-secret", nil)

	rr := postJSON(adminRouter(svc), "/oidc/clients", in)
	require.Equal(t, http.StatusCreated, rr.Code)
	body := rr.Body.String()
	assert.Contains(t, body, `"client_secret":"the-secret"`)
	assert.Contains(t, body, `"issuer":"https://auth.example.com"`, "the response says what issuer to configure")
	assert.NotContains(t, body, "stored-hash", "the hash is never serialized")
}

func TestCreateClient_ErrorMapping(t *testing.T) {
	cases := []struct {
		err    error
		status int
	}{
		{oidc.ErrInvalidClientID, http.StatusBadRequest},
		{oidc.ErrInvalidRedirectURI, http.StatusBadRequest},
		{oidc.ErrClientExists, http.StatusConflict},
	}
	for _, c := range cases {
		t.Run(c.err.Error(), func(t *testing.T) {
			svc := mocks.NewMockService(t)
			svc.EXPECT().CreateClient(mock.Anything, mock.Anything, mock.Anything).Return(model.Client{}, "", c.err)
			assert.Equal(t, c.status, postJSON(adminRouter(svc), "/oidc/clients", model.CreateClientInput{}).Code)
		})
	}
}

func TestListClients_NeverExposesHashes(t *testing.T) {
	svc := mocks.NewMockService(t)
	svc.EXPECT().ListClients(mock.Anything).Return([]model.Client{{ID: "cashflow", SecretHash: "stored-hash"}}, nil)
	rr := httptest.NewRecorder()
	adminRouter(svc).ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/oidc/clients", nil))
	require.Equal(t, http.StatusOK, rr.Code)
	assert.NotContains(t, rr.Body.String(), "stored-hash")
}

func TestRevokeClient(t *testing.T) {
	for _, c := range []struct {
		err    error
		status int
	}{{nil, http.StatusOK}, {oidc.ErrClientNotFound, http.StatusNotFound}} {
		svc := mocks.NewMockService(t)
		svc.EXPECT().RevokeClient(mock.Anything, "cashflow").Return(c.err)
		rr := httptest.NewRecorder()
		adminRouter(svc).ServeHTTP(rr, httptest.NewRequest(http.MethodDelete, "/oidc/clients/cashflow", nil))
		assert.Equal(t, c.status, rr.Code)
	}
}

func TestUpdateClient(t *testing.T) {
	patch := func(svc Service, body string) *httptest.ResponseRecorder {
		rr := httptest.NewRecorder()
		adminRouter(svc).ServeHTTP(rr, httptest.NewRequest(http.MethodPatch, "/oidc/clients/cashflow", bytes.NewBufferString(body)))
		return rr
	}
	t.Run("passes only the given fields", func(t *testing.T) {
		svc := mocks.NewMockService(t)
		color := "#0f766e"
		svc.EXPECT().UpdateClient(mock.Anything, "cashflow", model.UpdateClientInput{BrandColor: &color}).
			Return(model.Client{ID: "cashflow", Branding: model.Branding{BrandColor: color}}, nil)
		rr := patch(svc, `{"brand_color":"#0f766e"}`)
		require.Equal(t, http.StatusOK, rr.Code)
		assert.Contains(t, rr.Body.String(), `"brand_color":"#0f766e"`)
	})
	for _, c := range []struct {
		err    error
		status int
	}{{oidc.ErrInvalidBranding, http.StatusBadRequest}, {oidc.ErrClientNotFound, http.StatusNotFound}, {oidc.ErrClientRevoked, http.StatusNotFound}} {
		t.Run(c.err.Error(), func(t *testing.T) {
			svc := mocks.NewMockService(t)
			svc.EXPECT().UpdateClient(mock.Anything, "cashflow", mock.Anything).Return(model.Client{}, c.err)
			assert.Equal(t, c.status, patch(svc, `{"icon":"x"}`).Code)
		})
	}
	t.Run("bad json", func(t *testing.T) {
		assert.Equal(t, http.StatusBadRequest, patch(mocks.NewMockService(t), `{`).Code)
	})
}
