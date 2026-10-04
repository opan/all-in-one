package service

import (
	"context"
	"crypto/rsa"
	"strings"
	"testing"

	"github.com/all-in-one/internal/auth"
	"github.com/all-in-one/internal/config"
	"github.com/all-in-one/internal/oidc/model"
	"github.com/all-in-one/internal/oidc/service/mocks"
	"github.com/go-jose/go-jose/v4"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const testEncKey = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

func newKeyTestService(t *testing.T, keys *mocks.MockKeyRepository, encKey string) *Service {
	t.Helper()
	store := mocks.NewMockStorage(t)
	store.EXPECT().KeyRepo().Return(keys).Maybe()
	cfg := config.Config{}
	cfg.Auth.TOTPEncryptionKey = encKey
	return &Service{store: store, config: cfg, log: zerolog.Nop()}
}

func TestLoadKeys_GeneratesAndStoresEncryptedKeyOnFirstStart(t *testing.T) {
	repo := mocks.NewMockKeyRepository(t)
	repo.EXPECT().ListActive(mock.Anything).Return(nil, nil)
	var stored model.SigningKey
	repo.EXPECT().Create(mock.Anything, mock.MatchedBy(func(k model.SigningKey) bool { stored = k; return true })).Return(nil)

	svc := newKeyTestService(t, repo, testEncKey)
	require.NoError(t, svc.loadKeys(context.Background()))

	assert.NotContains(t, stored.PrivateKeyEncrypted, "PRIVATE KEY", "the private key is never stored as plain PEM")
	assert.Equal(t, "RS256", stored.Algorithm)
	signer := svc.keys.signing()
	require.NotNil(t, signer)
	assert.Equal(t, stored.ID, signer.ID())
	assert.Equal(t, jose.RS256, signer.SignatureAlgorithm())
	assert.Equal(t, signingKeyBits, signer.Key().(*rsa.PrivateKey).N.BitLen())
	require.Len(t, svc.keys.published(), 1)
	assert.Equal(t, "sig", svc.keys.published()[0].Use())
}

func TestLoadKeys_ReusesStoredKeysNewestSigns(t *testing.T) {
	newest, err := newStoredKeyForTest(t)
	require.NoError(t, err)
	older, err := newStoredKeyForTest(t)
	require.NoError(t, err)

	repo := mocks.NewMockKeyRepository(t)
	repo.EXPECT().ListActive(mock.Anything).Return([]model.SigningKey{newest, older}, nil)
	// no Create expectation: an existing install must not generate a new key

	svc := newKeyTestService(t, repo, testEncKey)
	require.NoError(t, svc.loadKeys(context.Background()))
	assert.Equal(t, newest.ID, svc.keys.signing().ID())
	assert.Len(t, svc.keys.published(), 2, "older keys stay published so recently signed tokens still verify")
}

func TestLoadKeys_WrongEncryptionKeyFailsLoudly(t *testing.T) {
	k, err := newStoredKeyForTest(t)
	require.NoError(t, err)
	repo := mocks.NewMockKeyRepository(t)
	repo.EXPECT().ListActive(mock.Anything).Return([]model.SigningKey{k}, nil)

	otherKey := strings.Repeat("f", 64)
	err = newKeyTestService(t, repo, otherKey).loadKeys(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "totp_encryption_key")
}

func newStoredKeyForTest(t *testing.T) (model.SigningKey, error) {
	t.Helper()
	enc, err := hexKey(testEncKey)
	if err != nil {
		return model.SigningKey{}, err
	}
	return newStoredKey(enc)
}

func hexKey(s string) ([]byte, error) {
	return auth.ParseEncryptionKey(s)
}
