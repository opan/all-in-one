package service

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	httpHelper "github.com/all-in-one/internal/http"
	"github.com/all-in-one/internal/config"
	"github.com/all-in-one/internal/ratelimit"
	"github.com/all-in-one/internal/ratelimit/model"
	"github.com/all-in-one/internal/ratelimit/service/mocks"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func newTokenTestService(t *testing.T, tokenRepo *mocks.MockTokenRepository) *Service {
	t.Helper()
	store := mocks.NewMockStorage(t)
	store.EXPECT().TokenRepo().Return(tokenRepo).Maybe()
	cfg := config.Config{}
	return &Service{
		Store:      store,
		tokenCache: newTokenCache(0), // caching off by default; tests opt in
		config:     cfg,
		log:        zerolog.Nop(),
	}
}

func TestCreateToken_ReturnsPlaintextOnce(t *testing.T) {
	tokenRepo := mocks.NewMockTokenRepository(t)
	var stored model.AppToken
	tokenRepo.EXPECT().Create(mock.Anything, mock.MatchedBy(func(tok model.AppToken) bool {
		stored = tok
		return true
	})).Return(nil)

	svc := newTokenTestService(t, tokenRepo)
	tok, plaintext, err := svc.CreateToken(context.Background(), "cashflow", "cashflow prod", "", "admin")
	require.NoError(t, err)

	assert.True(t, strings.HasPrefix(plaintext, "aio_cashflow_"), "plaintext carries the readable app segment")
	assert.Equal(t, "cashflow.", tok.ScopePrefix, "scope prefix defaults to <app>.")
	assert.Equal(t, hashToken(plaintext), stored.TokenHash, "the stored hash matches the plaintext")
	assert.NotEqual(t, plaintext, stored.TokenHash, "the plaintext is never stored")
	assert.Len(t, stored.TokenPrefix, tokenPrefixLen)
	require.NotNil(t, stored.CreatedBy)
	assert.Equal(t, "admin", *stored.CreatedBy)
}

func TestCreateToken_CustomScopePrefix(t *testing.T) {
	tokenRepo := mocks.NewMockTokenRepository(t)
	tokenRepo.EXPECT().Create(mock.Anything, mock.Anything).Return(nil)

	svc := newTokenTestService(t, tokenRepo)
	tok, _, err := svc.CreateToken(context.Background(), "cashflow", "n", "cashflow.entry.", "admin")
	require.NoError(t, err)
	assert.Equal(t, "cashflow.entry.", tok.ScopePrefix)
}

func TestCreateToken_RequiresAppAndName(t *testing.T) {
	tokenRepo := mocks.NewMockTokenRepository(t)
	svc := newTokenTestService(t, tokenRepo)

	_, _, err := svc.CreateToken(context.Background(), "", "name", "", "admin")
	assert.Error(t, err)
	_, _, err = svc.CreateToken(context.Background(), "cashflow", "", "", "admin")
	assert.Error(t, err)
}

func TestVerifyToken_RoundTrip(t *testing.T) {
	tokenRepo := mocks.NewMockTokenRepository(t)
	var stored model.AppToken
	tokenRepo.EXPECT().Create(mock.Anything, mock.MatchedBy(func(tok model.AppToken) bool {
		stored = tok
		return true
	})).Return(nil)

	svc := newTokenTestService(t, tokenRepo)
	_, plaintext, err := svc.CreateToken(context.Background(), "cashflow", "prod", "", "admin")
	require.NoError(t, err)

	tokenRepo.EXPECT().GetByHash(mock.Anything, hashToken(plaintext)).Return(stored, nil)
	tokenRepo.EXPECT().TouchLastUsed(mock.Anything, stored.ID, mock.Anything).Return(nil)

	got, err := svc.VerifyToken(context.Background(), plaintext)
	require.NoError(t, err)
	assert.Equal(t, stored.ID, got.ID)
	assert.Equal(t, "cashflow.", got.ScopePrefix)
}

func TestVerifyToken_Unknown(t *testing.T) {
	tokenRepo := mocks.NewMockTokenRepository(t)
	tokenRepo.EXPECT().GetByHash(mock.Anything, mock.Anything).Return(model.AppToken{}, httpHelper.ErrNotFound)

	svc := newTokenTestService(t, tokenRepo)
	_, err := svc.VerifyToken(context.Background(), "aio_cashflow_bogus")
	assert.ErrorIs(t, err, ratelimit.ErrTokenNotFound)
}

func TestVerifyToken_EmptyIsNotFound(t *testing.T) {
	tokenRepo := mocks.NewMockTokenRepository(t)
	svc := newTokenTestService(t, tokenRepo)
	_, err := svc.VerifyToken(context.Background(), "   ")
	assert.ErrorIs(t, err, ratelimit.ErrTokenNotFound)
}

func TestVerifyToken_RevokedIsNotFound(t *testing.T) {
	// A revoked token is excluded at the SQL level, so GetByHash returns
	// ErrNotFound for it — same path as an unknown token.
	tokenRepo := mocks.NewMockTokenRepository(t)
	tokenRepo.EXPECT().GetByHash(mock.Anything, mock.Anything).Return(model.AppToken{}, httpHelper.ErrNotFound)

	svc := newTokenTestService(t, tokenRepo)
	_, err := svc.VerifyToken(context.Background(), "aio_cashflow_revoked")
	assert.ErrorIs(t, err, ratelimit.ErrTokenNotFound)
}

func TestVerifyToken_UsesCache(t *testing.T) {
	tokenRepo := mocks.NewMockTokenRepository(t)
	stored := model.AppToken{ID: "t1", App: "cashflow", ScopePrefix: "cashflow.", TokenHash: hashToken("aio_cashflow_abc")}
	// GetByHash + TouchLastUsed must be called exactly once despite two verifies
	tokenRepo.EXPECT().GetByHash(mock.Anything, stored.TokenHash).Return(stored, nil).Once()
	tokenRepo.EXPECT().TouchLastUsed(mock.Anything, "t1", mock.Anything).Return(nil).Once()

	svc := newTokenTestService(t, tokenRepo)
	svc.tokenCache = newTokenCache(time.Minute)

	_, err := svc.VerifyToken(context.Background(), "aio_cashflow_abc")
	require.NoError(t, err)
	_, err = svc.VerifyToken(context.Background(), "aio_cashflow_abc")
	require.NoError(t, err, "second verify should hit the cache, not the DB")
}

func TestRevokeToken_ClearsCache(t *testing.T) {
	tokenRepo := mocks.NewMockTokenRepository(t)
	stored := model.AppToken{ID: "t1", App: "cashflow", ScopePrefix: "cashflow.", TokenHash: hashToken("aio_cashflow_abc")}
	tokenRepo.EXPECT().GetByHash(mock.Anything, stored.TokenHash).Return(stored, nil).Twice()
	tokenRepo.EXPECT().TouchLastUsed(mock.Anything, "t1", mock.Anything).Return(nil).Twice()
	tokenRepo.EXPECT().Revoke(mock.Anything, "t1").Return(nil)

	svc := newTokenTestService(t, tokenRepo)
	svc.tokenCache = newTokenCache(time.Minute)

	_, err := svc.VerifyToken(context.Background(), "aio_cashflow_abc")
	require.NoError(t, err)
	require.NoError(t, svc.RevokeToken(context.Background(), "t1"))
	// cache cleared → a second verify goes back to the DB (hence Twice above)
	_, err = svc.VerifyToken(context.Background(), "aio_cashflow_abc")
	require.NoError(t, err)
}

func TestAppToken_JSON_NeverLeaksHash(t *testing.T) {
	tok := model.AppToken{ID: "t1", App: "cashflow", TokenHash: "secret-hash", TokenPrefix: "aio_cashflo"}
	b, err := json.Marshal(tok)
	require.NoError(t, err)
	assert.NotContains(t, string(b), "secret-hash", "token_hash must never be serialized")
	assert.NotContains(t, string(b), "token_hash")
	assert.Contains(t, string(b), "aio_cashflo", "prefix is safe to expose")
}
