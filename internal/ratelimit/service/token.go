package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	httpHelper "github.com/all-in-one/internal/http"
	"github.com/all-in-one/internal/ratelimit"
	"github.com/all-in-one/internal/ratelimit/model"
	"github.com/google/uuid"
)

// tokenPrefixLen is how many leading characters of the plaintext token are
// stored for display (the "aio_cashflow" portion), never enough to reconstruct
// the secret.
const tokenPrefixLen = 12

// tokenCache is an optional in-memory cache of verified tokens, keyed by hash,
// mirroring ruleCache. It bounds how often the hot-path /check verify touches
// the DB. Revocation is enforced at the SQL level (GetByHash excludes revoked),
// and RevokeToken clears this cache, so a stale hit cannot outlive a revoke by
// more than the cache TTL even without the clear. A nil or zero-TTL cache is a
// no-op (one indexed SELECT per check).
type tokenCache struct {
	ttl time.Duration
	mu  sync.RWMutex
	m   map[string]tokenCacheEntry
}

type tokenCacheEntry struct {
	token   model.AppToken
	expires time.Time
}

func newTokenCache(ttl time.Duration) *tokenCache {
	return &tokenCache{ttl: ttl, m: make(map[string]tokenCacheEntry)}
}

func (c *tokenCache) get(hash string) (model.AppToken, bool) {
	if c == nil || c.ttl <= 0 {
		return model.AppToken{}, false
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	e, ok := c.m[hash]
	if !ok || time.Now().After(e.expires) {
		return model.AppToken{}, false
	}
	return e.token, true
}

func (c *tokenCache) put(hash string, t model.AppToken) {
	if c == nil || c.ttl <= 0 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.m[hash] = tokenCacheEntry{token: t, expires: time.Now().Add(c.ttl)}
}

func (c *tokenCache) clear() {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.m = make(map[string]tokenCacheEntry)
}

// hashToken is the one-way lookup hash for an app token. SHA-256, not bcrypt:
// the token is a 256-bit random secret, so the brute-force resistance bcrypt
// buys does not apply, and bcrypt's deliberate slowness would land on the
// caller's request hot path (EXTERNAL_RATE_LIMIT plan ATTENTION #1).
func hashToken(plaintext string) string {
	sum := sha256.Sum256([]byte(plaintext))
	return hex.EncodeToString(sum[:])
}

// generateToken mints a new plaintext token and returns it alongside its hash
// and display prefix. Format: aio_<app>_<base64url(32 random bytes)>. The app
// segment is a readability affordance only — authorization always comes from
// the DB row, never from parsing the string.
func generateToken(app string) (plaintext, hash, prefix string, err error) {
	raw := make([]byte, 32)
	if _, err = rand.Read(raw); err != nil {
		return "", "", "", fmt.Errorf("generate token: %w", err)
	}
	plaintext = fmt.Sprintf("aio_%s_%s", app, base64.RawURLEncoding.EncodeToString(raw))
	prefix = plaintext
	if len(prefix) > tokenPrefixLen {
		prefix = prefix[:tokenPrefixLen]
	}
	return plaintext, hashToken(plaintext), prefix, nil
}

// CreateToken mints and stores a new app token, returning the model row and the
// plaintext token. The plaintext is returned exactly once, here, and is never
// stored or recoverable afterwards.
func (s *Service) CreateToken(ctx context.Context, app, name, scopePrefix, createdBy string) (model.AppToken, string, error) {
	app = strings.TrimSpace(app)
	name = strings.TrimSpace(name)
	if app == "" {
		return model.AppToken{}, "", fmt.Errorf("app is required")
	}
	if name == "" {
		return model.AppToken{}, "", fmt.Errorf("name is required")
	}

	scopePrefix = strings.TrimSpace(scopePrefix)
	if scopePrefix == "" {
		scopePrefix = app + "."
	}
	// A token with an empty scope prefix would match every target, defeating
	// the point of prefix scoping (locked decision #4). It cannot be empty
	// here given the default above, but assert it so a future refactor can't
	// silently regress the invariant.
	if scopePrefix == "" {
		return model.AppToken{}, "", fmt.Errorf("scope prefix must not be empty")
	}

	plaintext, hash, prefix, err := generateToken(app)
	if err != nil {
		return model.AppToken{}, "", err
	}

	tok := model.AppToken{
		ID:          uuid.NewString(),
		App:         app,
		Name:        name,
		TokenHash:   hash,
		TokenPrefix: prefix,
		ScopePrefix: scopePrefix,
		CreatedAt:   time.Now().UTC(),
	}
	if createdBy != "" {
		tok.CreatedBy = &createdBy
	}

	if err := s.Store.TokenRepo().Create(ctx, tok); err != nil {
		return model.AppToken{}, "", fmt.Errorf("create token: %w", err)
	}
	return tok, plaintext, nil
}

// ListTokens returns all app tokens (including revoked ones — the audit trail
// is the point). The plaintext token is never included; TokenHash is elided by
// the model's json:"-" tag.
func (s *Service) ListTokens(ctx context.Context) ([]model.AppToken, error) {
	return s.Store.TokenRepo().List(ctx)
}

// RevokeToken soft-revokes a token and clears the token cache so the revocation
// takes effect immediately rather than after the cache TTL.
func (s *Service) RevokeToken(ctx context.Context, id string) error {
	if err := s.Store.TokenRepo().Revoke(ctx, id); err != nil {
		return fmt.Errorf("revoke token: %w", err)
	}
	s.tokenCache.clear()
	return nil
}

// VerifyToken resolves a plaintext token to its (live, non-revoked) row.
// Returns ratelimit.ErrTokenNotFound for an unknown, malformed, or revoked
// token. On a cache miss it records last-used best-effort — a failure there
// never fails the verify.
func (s *Service) VerifyToken(ctx context.Context, plaintext string) (model.AppToken, error) {
	plaintext = strings.TrimSpace(plaintext)
	if plaintext == "" {
		return model.AppToken{}, ratelimit.ErrTokenNotFound
	}
	hash := hashToken(plaintext)

	if tok, ok := s.tokenCache.get(hash); ok {
		return tok, nil
	}

	tok, err := s.Store.TokenRepo().GetByHash(ctx, hash)
	if err != nil {
		if errors.Is(err, httpHelper.ErrNotFound) {
			return model.AppToken{}, ratelimit.ErrTokenNotFound
		}
		return model.AppToken{}, err
	}

	s.tokenCache.put(hash, tok)
	// Best-effort last-seen; only on cache miss, so writes are naturally
	// bounded to once per TTL per token. Never let it fail the verify.
	if err := s.Store.TokenRepo().TouchLastUsed(ctx, tok.ID, time.Now().UTC()); err != nil {
		s.log.Debug().Err(err).Str("token_prefix", tok.TokenPrefix).Msg("ratelimit: touch last_used failed")
	}
	return tok, nil
}
