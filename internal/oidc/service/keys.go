package service

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"sync"
	"time"

	"github.com/all-in-one/internal/auth"
	"github.com/all-in-one/internal/oidc/model"
	"github.com/go-jose/go-jose/v4"
	"github.com/google/uuid"
	"github.com/zitadel/oidc/v3/pkg/op"
)

const signingKeyBits = 2048

type signingKey struct {
	id   string
	priv *rsa.PrivateKey
}

func (k *signingKey) SignatureAlgorithm() jose.SignatureAlgorithm { return jose.RS256 }
func (k *signingKey) Key() any                                    { return k.priv }
func (k *signingKey) ID() string                                  { return k.id }

type publicKey struct {
	id  string
	pub *rsa.PublicKey
}

func (k *publicKey) ID() string                         { return k.id }
func (k *publicKey) Algorithm() jose.SignatureAlgorithm { return jose.RS256 }
func (k *publicKey) Use() string                        { return "sig" }
func (k *publicKey) Key() any                           { return k.pub }

// keySet holds the decrypted signing keys in memory: the newest active key
// signs, and every active key is published so tokens signed just before a
// rotation still verify.
type keySet struct {
	mu     sync.RWMutex
	active *signingKey
	public []op.Key
	encKey []byte
}

func (ks *keySet) signing() *signingKey {
	ks.mu.RLock()
	defer ks.mu.RUnlock()
	return ks.active
}

func (ks *keySet) published() []op.Key {
	ks.mu.RLock()
	defer ks.mu.RUnlock()
	return ks.public
}

// publishedKeys returns every active key in the database, not only the ones
// loaded at startup: with several replicas each may sign with a key the
// others never loaded (two fresh pods each generate one), and an app may
// fetch the key set from any of them. Newly seen keys are cached.
func (s *Service) publishedKeys(ctx context.Context) []op.Key {
	stored, err := s.store.KeyRepo().ListActive(ctx)
	if err != nil {
		s.log.Warn().Err(err).Msg("oidc: list signing keys; publishing the ones loaded at startup")
		return s.keys.published()
	}
	ks := s.keys
	ks.mu.Lock()
	defer ks.mu.Unlock()
	known := make(map[string]bool, len(ks.public))
	for _, k := range ks.public {
		known[k.ID()] = true
	}
	for _, k := range stored {
		if known[k.ID] {
			continue
		}
		priv, err := decryptKey(k, ks.encKey)
		if err != nil {
			s.log.Error().Err(err).Msg("oidc: skipping a signing key that can't be decrypted")
			continue
		}
		ks.public = append(ks.public, &publicKey{id: k.ID, pub: &priv.PublicKey})
	}
	return ks.public
}

// loadKeys decrypts the active signing keys, generating and storing the first
// one on a fresh install. Private keys are encrypted with
// auth.totp_encryption_key using the same AES-GCM helper as TOTP secrets.
func (s *Service) loadKeys(ctx context.Context) error {
	encKey, err := auth.ParseEncryptionKey(s.config.Auth.TOTPEncryptionKey)
	if err != nil {
		return fmt.Errorf("oidc signing keys: %w", err)
	}

	stored, err := s.store.KeyRepo().ListActive(ctx)
	if err != nil {
		return fmt.Errorf("list signing keys: %w", err)
	}
	if len(stored) == 0 {
		k, err := newStoredKey(encKey)
		if err != nil {
			return err
		}
		if err := s.store.KeyRepo().Create(ctx, k); err != nil {
			return fmt.Errorf("store signing key: %w", err)
		}
		s.log.Info().Str("kid", k.ID).Msg("oidc: generated signing key")
		stored = []model.SigningKey{k}
	}

	ks := &keySet{encKey: encKey}
	for i, k := range stored {
		priv, err := decryptKey(k, encKey)
		if err != nil {
			return err
		}
		if i == 0 {
			ks.active = &signingKey{id: k.ID, priv: priv}
		}
		ks.public = append(ks.public, &publicKey{id: k.ID, pub: &priv.PublicKey})
	}
	s.keys = ks
	return nil
}

func newStoredKey(encKey []byte) (model.SigningKey, error) {
	priv, err := rsa.GenerateKey(rand.Reader, signingKeyBits)
	if err != nil {
		return model.SigningKey{}, fmt.Errorf("generate signing key: %w", err)
	}
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)})
	enc, err := auth.EncryptTOTPSecret(string(pemBytes), encKey)
	if err != nil {
		return model.SigningKey{}, fmt.Errorf("encrypt signing key: %w", err)
	}
	return model.SigningKey{
		ID: uuid.NewString(), Algorithm: string(jose.RS256),
		PrivateKeyEncrypted: enc, CreatedAt: time.Now().UTC(),
	}, nil
}

func decryptKey(k model.SigningKey, encKey []byte) (*rsa.PrivateKey, error) {
	pemStr, err := auth.DecryptTOTPSecret(k.PrivateKeyEncrypted, encKey)
	if err != nil {
		return nil, fmt.Errorf("decrypt signing key %s (was auth.totp_encryption_key changed?): %w", k.ID, err)
	}
	block, _ := pem.Decode([]byte(pemStr))
	if block == nil {
		return nil, fmt.Errorf("signing key %s: invalid PEM", k.ID)
	}
	priv, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("signing key %s: %w", k.ID, err)
	}
	return priv, nil
}
