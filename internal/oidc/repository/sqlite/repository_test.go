package sqlite

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/all-in-one/internal/oidc"
	"github.com/all-in-one/internal/oidc/model"
	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestDB(t *testing.T) *sqlx.DB {
	t.Helper()
	db, err := sqlx.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	for _, m := range []string{"11_add_oidc_clients_and_keys", "12_add_oidc_client_branding"} {
		schema, err := os.ReadFile("../../../../db/migrations/sqlite3/" + m + ".up.sql")
		require.NoError(t, err)
		_, err = db.Exec(string(schema))
		require.NoError(t, err)
	}
	return db
}

func sampleClient(id string) model.Client {
	return model.Client{
		ID: id, Name: "Cashflow", SecretHash: "hash",
		RedirectURIs:           model.StringList{"https://cashflow.example.com/auth/callback"},
		PostLogoutRedirectURIs: model.StringList{"https://cashflow.example.com/"},
		CreatedAt:              time.Now().UTC(),
	}
}

func TestClientRepository_CreateGetRoundTrip(t *testing.T) {
	repo := NewClientRepository(newTestDB(t))
	ctx := context.Background()

	require.NoError(t, repo.Create(ctx, sampleClient("cashflow")))
	got, err := repo.Get(ctx, "cashflow")
	require.NoError(t, err)
	assert.Equal(t, "Cashflow", got.Name)
	assert.Equal(t, model.StringList{"https://cashflow.example.com/auth/callback"}, got.RedirectURIs)
	assert.Equal(t, model.StringList{"https://cashflow.example.com/"}, got.PostLogoutRedirectURIs)
	assert.Nil(t, got.RevokedAt)
}

func TestClientRepository_BrandingRoundTripAndUpdate(t *testing.T) {
	repo := NewClientRepository(newTestDB(t))
	ctx := context.Background()

	c := sampleClient("cashflow")
	c.Branding = model.Branding{BrandColor: "#0f766e", Icon: "💰"}
	require.NoError(t, repo.Create(ctx, c))
	got, err := repo.Get(ctx, "cashflow")
	require.NoError(t, err)
	assert.Equal(t, c.Branding, got.Branding)

	got.Name, got.Branding = "Cashflow Kas", model.Branding{BrandColor: "#1d4ed8"}
	require.NoError(t, repo.Update(ctx, got))
	updated, err := repo.Get(ctx, "cashflow")
	require.NoError(t, err)
	assert.Equal(t, "Cashflow Kas", updated.Name)
	assert.Equal(t, model.Branding{BrandColor: "#1d4ed8"}, updated.Branding)

	require.NoError(t, repo.Revoke(ctx, "cashflow", time.Now()))
	assert.ErrorIs(t, repo.Update(ctx, updated), oidc.ErrClientNotFound, "a revoked client can't be edited")
	assert.ErrorIs(t, repo.Update(ctx, sampleClient("nope")), oidc.ErrClientNotFound)
}

func TestClientRepository_Errors(t *testing.T) {
	repo := NewClientRepository(newTestDB(t))
	ctx := context.Background()
	require.NoError(t, repo.Create(ctx, sampleClient("cashflow")))

	cases := []struct {
		name string
		run  func() error
		want error
	}{
		{"duplicate id", func() error { return repo.Create(ctx, sampleClient("cashflow")) }, oidc.ErrClientExists},
		{"get unknown", func() error { _, err := repo.Get(ctx, "nope"); return err }, oidc.ErrClientNotFound},
		{"revoke unknown", func() error { return repo.Revoke(ctx, "nope", time.Now()) }, oidc.ErrClientNotFound},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) { assert.ErrorIs(t, c.run(), c.want) })
	}
}

func TestClientRepository_RevokeOnlyOnce(t *testing.T) {
	repo := NewClientRepository(newTestDB(t))
	ctx := context.Background()
	require.NoError(t, repo.Create(ctx, sampleClient("cashflow")))

	require.NoError(t, repo.Revoke(ctx, "cashflow", time.Now()))
	got, err := repo.Get(ctx, "cashflow")
	require.NoError(t, err)
	assert.NotNil(t, got.RevokedAt, "revoked client is still readable for the audit trail")
	assert.ErrorIs(t, repo.Revoke(ctx, "cashflow", time.Now()), oidc.ErrClientNotFound, "already revoked")
}

func TestClientRepository_List(t *testing.T) {
	repo := NewClientRepository(newTestDB(t))
	ctx := context.Background()
	a, b := sampleClient("a-app"), sampleClient("b-app")
	b.CreatedAt = a.CreatedAt.Add(time.Second)
	require.NoError(t, repo.Create(ctx, b))
	require.NoError(t, repo.Create(ctx, a))

	got, err := repo.List(ctx)
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, "a-app", got[0].ID, "ordered by creation time")
}

func TestKeyRepository_ListActiveNewestFirst(t *testing.T) {
	repo := NewKeyRepository(newTestDB(t))
	ctx := context.Background()
	now := time.Now().UTC()
	require.NoError(t, repo.Create(ctx, model.SigningKey{ID: "old", Algorithm: "RS256", PrivateKeyEncrypted: "x", CreatedAt: now}))
	require.NoError(t, repo.Create(ctx, model.SigningKey{ID: "new", Algorithm: "RS256", PrivateKeyEncrypted: "y", CreatedAt: now.Add(time.Hour)}))

	keys, err := repo.ListActive(ctx)
	require.NoError(t, err)
	require.Len(t, keys, 2)
	assert.Equal(t, "new", keys[0].ID)
}
