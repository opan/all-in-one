package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/all-in-one/internal/oidc"
	"github.com/all-in-one/internal/oidc/model"
)

var (
	clientIDPattern   = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{1,62}$`)
	brandColorPattern = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)
)

const (
	maxClientNameLen = 100
	// An icon is an emoji or one or two characters; 16 bytes fits multi-code-
	// point emoji such as flags or skin tones.
	maxIconBytes = 16
)

// CreateClient registers an app and returns it with its plaintext secret,
// which is shown exactly once: only its SHA-256 hash is stored.
func (s *Service) CreateClient(ctx context.Context, in model.CreateClientInput, createdBy string) (model.Client, string, error) {
	in.ID = strings.TrimSpace(in.ID)
	in.Name = strings.TrimSpace(in.Name)
	if !clientIDPattern.MatchString(in.ID) {
		return model.Client{}, "", fmt.Errorf("%w: use 2-63 lowercase letters, digits, '-' or '_'", oidc.ErrInvalidClientID)
	}
	if err := validateClientName(in.Name); err != nil {
		return model.Client{}, "", err
	}
	branding, err := normalizeBranding(in.BrandColor, in.Icon)
	if err != nil {
		return model.Client{}, "", err
	}
	if len(in.RedirectURIs) == 0 {
		return model.Client{}, "", fmt.Errorf("%w: at least one redirect uri is required", oidc.ErrInvalidRedirectURI)
	}
	for _, list := range [][]string{in.RedirectURIs, in.PostLogoutRedirectURIs} {
		for _, u := range list {
			if err := validateRedirectURI(u); err != nil {
				return model.Client{}, "", err
			}
		}
	}

	secret, err := newSecret()
	if err != nil {
		return model.Client{}, "", err
	}
	c := model.Client{
		ID:                     in.ID,
		Name:                   in.Name,
		SecretHash:             hashSecret(secret),
		RedirectURIs:           in.RedirectURIs,
		PostLogoutRedirectURIs: in.PostLogoutRedirectURIs,
		Branding:               branding,
		CreatedAt:              time.Now().UTC(),
	}
	if createdBy != "" {
		c.CreatedBy = &createdBy
	}
	if err := s.store.ClientRepo().Create(ctx, c); err != nil {
		return model.Client{}, "", err
	}
	return c, secret, nil
}

func (s *Service) ListClients(ctx context.Context) ([]model.Client, error) {
	return s.store.ClientRepo().List(ctx)
}

// UpdateClient changes an app's name or branding; ids, URIs and secrets are
// fixed at registration (re-register to change them).
func (s *Service) UpdateClient(ctx context.Context, id string, in model.UpdateClientInput) (model.Client, error) {
	c, err := s.ActiveClient(ctx, id)
	if err != nil {
		return model.Client{}, err
	}
	if in.Name != nil {
		c.Name = strings.TrimSpace(*in.Name)
		if err := validateClientName(c.Name); err != nil {
			return model.Client{}, err
		}
	}
	color, icon := c.BrandColor, c.Icon
	if in.BrandColor != nil {
		color = *in.BrandColor
	}
	if in.Icon != nil {
		icon = *in.Icon
	}
	if c.Branding, err = normalizeBranding(color, icon); err != nil {
		return model.Client{}, err
	}
	if err := s.store.ClientRepo().Update(ctx, c); err != nil {
		return model.Client{}, err
	}
	return c, nil
}

func (s *Service) RevokeClient(ctx context.Context, id string) error {
	return s.store.ClientRepo().Revoke(ctx, id, time.Now())
}

// ActiveClient returns a client that may currently log users in.
func (s *Service) ActiveClient(ctx context.Context, id string) (model.Client, error) {
	c, err := s.store.ClientRepo().Get(ctx, id)
	if err != nil {
		return model.Client{}, err
	}
	if c.RevokedAt != nil {
		return model.Client{}, oidc.ErrClientRevoked
	}
	return c, nil
}

// AuthenticateClient checks a client's secret in constant time.
func (s *Service) AuthenticateClient(ctx context.Context, id, secret string) error {
	c, err := s.ActiveClient(ctx, id)
	if err != nil {
		return err
	}
	if subtle.ConstantTimeCompare([]byte(hashSecret(secret)), []byte(c.SecretHash)) != 1 {
		return oidc.ErrInvalidClientSecret
	}
	return nil
}

func validateClientName(name string) error {
	if name == "" || len(name) > maxClientNameLen {
		return fmt.Errorf("%w: 1-%d characters", oidc.ErrInvalidClientName, maxClientNameLen)
	}
	return nil
}

// normalizeBranding checks an optional "#rrggbb" colour (stored lowercase)
// and an optional short icon without spaces or control characters.
func normalizeBranding(color, icon string) (model.Branding, error) {
	color, icon = strings.TrimSpace(color), strings.TrimSpace(icon)
	if color != "" && !brandColorPattern.MatchString(color) {
		return model.Branding{}, fmt.Errorf("%w: brand color must look like #0f766e", oidc.ErrInvalidBranding)
	}
	if len(icon) > maxIconBytes || !utf8.ValidString(icon) || strings.IndexFunc(icon, func(r rune) bool {
		return unicode.IsSpace(r) || unicode.IsControl(r)
	}) >= 0 {
		return model.Branding{}, fmt.Errorf("%w: icon must be an emoji or 1-2 characters, without spaces", oidc.ErrInvalidBranding)
	}
	return model.Branding{BrandColor: strings.ToLower(color), Icon: icon}, nil
}

// validateRedirectURI requires an absolute https URL without a fragment.
// Plain http is accepted only for loopback hosts, so apps can be developed
// locally without opening a code-interception hole in production.
func validateRedirectURI(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || !u.IsAbs() || u.Host == "" {
		return fmt.Errorf("%w: %q is not an absolute URL", oidc.ErrInvalidRedirectURI, raw)
	}
	if u.Fragment != "" {
		return fmt.Errorf("%w: %q must not contain a fragment", oidc.ErrInvalidRedirectURI, raw)
	}
	switch u.Scheme {
	case "https":
		return nil
	case "http":
		if isLoopback(u.Hostname()) {
			return nil
		}
		return fmt.Errorf("%w: %q must use https (http is allowed only for localhost)", oidc.ErrInvalidRedirectURI, raw)
	default:
		return fmt.Errorf("%w: %q must use https", oidc.ErrInvalidRedirectURI, raw)
	}
}

func isLoopback(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func newSecret() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate client secret: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func hashSecret(secret string) string {
	sum := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(sum[:])
}
