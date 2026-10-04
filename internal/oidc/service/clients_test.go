package service

import (
	"context"
	"testing"
	"time"

	"github.com/all-in-one/internal/oidc"
	"github.com/all-in-one/internal/oidc/model"
	"github.com/all-in-one/internal/oidc/service/mocks"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func newClientTestService(t *testing.T, clients *mocks.MockClientRepository) *Service {
	t.Helper()
	store := mocks.NewMockStorage(t)
	store.EXPECT().ClientRepo().Return(clients).Maybe()
	return &Service{store: store, log: zerolog.Nop()}
}

func validInput() CreateClientInput {
	return CreateClientInput{
		ID: "cashflow", Name: "Cashflow",
		RedirectURIs:           []string{"https://cashflow.example.com/auth/callback"},
		PostLogoutRedirectURIs: []string{"https://cashflow.example.com/"},
	}
}

func TestCreateClient_StoresHashAndReturnsSecretOnce(t *testing.T) {
	repo := mocks.NewMockClientRepository(t)
	var stored model.Client
	repo.EXPECT().Create(mock.Anything, mock.MatchedBy(func(c model.Client) bool { stored = c; return true })).Return(nil)

	svc := newClientTestService(t, repo)
	c, secret, err := svc.CreateClient(context.Background(), validInput(), "admin")
	require.NoError(t, err)

	assert.Len(t, secret, 43, "32 random bytes, base64url without padding")
	assert.Equal(t, hashSecret(secret), stored.SecretHash)
	assert.NotContains(t, stored.SecretHash, secret, "the plaintext secret is never stored")
	assert.Equal(t, "cashflow", c.ID)
	require.NotNil(t, stored.CreatedBy)
	assert.Equal(t, "admin", *stored.CreatedBy)
}

func TestCreateClient_Validation(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*CreateClientInput)
		want   error
	}{
		{"uppercase id", func(in *CreateClientInput) { in.ID = "CashFlow" }, oidc.ErrInvalidClientID},
		{"one-char id", func(in *CreateClientInput) { in.ID = "c" }, oidc.ErrInvalidClientID},
		{"id with space", func(in *CreateClientInput) { in.ID = "cash flow" }, oidc.ErrInvalidClientID},
		{"empty name", func(in *CreateClientInput) { in.Name = "  " }, oidc.ErrInvalidClientName},
		{"no redirect uris", func(in *CreateClientInput) { in.RedirectURIs = nil }, oidc.ErrInvalidRedirectURI},
		{"relative redirect", func(in *CreateClientInput) { in.RedirectURIs = []string{"/auth/callback"} }, oidc.ErrInvalidRedirectURI},
		{"http on a public host", func(in *CreateClientInput) { in.RedirectURIs = []string{"http://cashflow.example.com/cb"} }, oidc.ErrInvalidRedirectURI},
		{"fragment", func(in *CreateClientInput) { in.RedirectURIs = []string{"https://cashflow.example.com/cb#x"} }, oidc.ErrInvalidRedirectURI},
		{"custom scheme", func(in *CreateClientInput) { in.RedirectURIs = []string{"javascript://cb"} }, oidc.ErrInvalidRedirectURI},
		{"bad post-logout uri", func(in *CreateClientInput) { in.PostLogoutRedirectURIs = []string{"http://evil.example.com/"} }, oidc.ErrInvalidRedirectURI},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			svc := newClientTestService(t, mocks.NewMockClientRepository(t))
			in := validInput()
			c.mutate(&in)
			_, _, err := svc.CreateClient(context.Background(), in, "admin")
			assert.ErrorIs(t, err, c.want)
		})
	}
}

func TestValidateRedirectURI_AllowsHTTPOnlyForLoopback(t *testing.T) {
	for _, u := range []string{"http://localhost:8090/auth/callback", "http://127.0.0.1:8090/cb", "http://[::1]:8090/cb"} {
		assert.NoError(t, validateRedirectURI(u), u)
	}
}

func TestAuthenticateClient(t *testing.T) {
	secret := "s3cret-value"
	revokedAt := time.Now()
	cases := []struct {
		name   string
		client model.Client
		getErr error
		secret string
		want   error
	}{
		{"correct secret", model.Client{ID: "cashflow", SecretHash: hashSecret(secret)}, nil, secret, nil},
		{"wrong secret", model.Client{ID: "cashflow", SecretHash: hashSecret(secret)}, nil, "nope", oidc.ErrInvalidClientSecret},
		{"revoked client", model.Client{ID: "cashflow", SecretHash: hashSecret(secret), RevokedAt: &revokedAt}, nil, secret, oidc.ErrClientRevoked},
		{"unknown client", model.Client{}, oidc.ErrClientNotFound, secret, oidc.ErrClientNotFound},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			repo := mocks.NewMockClientRepository(t)
			repo.EXPECT().Get(mock.Anything, "cashflow").Return(c.client, c.getErr)
			err := newClientTestService(t, repo).AuthenticateClient(context.Background(), "cashflow", c.secret)
			if c.want == nil {
				assert.NoError(t, err)
			} else {
				assert.ErrorIs(t, err, c.want)
			}
		})
	}
}
