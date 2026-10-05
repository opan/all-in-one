package service

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/go-jose/go-jose/v4"
	"github.com/google/uuid"
	"github.com/zitadel/oidc/v3/pkg/oidc"
	"github.com/zitadel/oidc/v3/pkg/op"

	"time"
)

// providerStorage implements the zitadel/oidc op.Storage on top of the
// Service: clients and keys come from the DB, auth requests/codes/tokens from
// the in-memory store, user claims from aio's user table.
type providerStorage struct {
	s *Service
}

var (
	_ op.Storage                        = (*providerStorage)(nil)
	_ op.CanSetUserinfoFromRequest      = (*providerStorage)(nil)
	_ op.CanTerminateSessionFromRequest = (*providerStorage)(nil)
)

func (p *providerStorage) CreateAuthRequest(ctx context.Context, req *oidc.AuthRequest, _ string) (op.AuthRequest, error) {
	if len(req.Prompt) == 1 && req.Prompt[0] == oidc.PromptNone {
		// Silent login would need a way to check aio's session here; not supported in v1.
		return nil, oidc.ErrLoginRequired()
	}
	// Forcing a fresh login isn't supported: /oauth/login finishes with
	// single sign-on whenever an aio session exists, so refuse rather than
	// silently ignore the app's request.
	if slices.Contains(req.Prompt, oidc.PromptLogin) || req.MaxAge != nil {
		return nil, oidc.ErrInvalidRequest().WithDescription("prompt=login and max_age are not supported")
	}
	// The library only enforces PKCE for public clients; aio requires it for all.
	if req.CodeChallenge == "" || req.CodeChallengeMethod != oidc.CodeChallengeMethodS256 {
		return nil, oidc.ErrInvalidRequest().WithDescription("PKCE with code_challenge_method=S256 is required")
	}
	r := &authRequest{
		ClientID: req.ClientID, RedirectURI: req.RedirectURI, State: req.State, Nonce: req.Nonce,
		Scopes: req.Scopes, Prompt: req.Prompt, ResponseType: req.ResponseType, ResponseMode: req.ResponseMode,
		CodeChallenge: &oidc.CodeChallenge{Challenge: req.CodeChallenge, Method: req.CodeChallengeMethod},
		BrowserHash:   hashBrowserID(browserIDFromContext(ctx)),
	}
	p.s.mem.addRequest(r)
	return r, nil
}

func (p *providerStorage) AuthRequestByID(_ context.Context, id string) (op.AuthRequest, error) {
	r, ok := p.s.mem.request(id)
	if !ok {
		return nil, oidc.ErrInvalidRequest().WithDescription("auth request not found or expired")
	}
	return r, nil
}

func (p *providerStorage) AuthRequestByCode(_ context.Context, code string) (op.AuthRequest, error) {
	r, ok := p.s.mem.requestByCode(code)
	if !ok {
		return nil, oidc.ErrInvalidGrant().WithDescription("code invalid or expired")
	}
	return r, nil
}

func (p *providerStorage) SaveAuthCode(_ context.Context, id, code string) error {
	if !p.s.mem.saveCode(id, code) {
		return oidc.ErrInvalidRequest().WithDescription("auth request not found or expired")
	}
	return nil
}

func (p *providerStorage) DeleteAuthRequest(_ context.Context, id string) error {
	p.s.mem.deleteRequest(id)
	return nil
}

func (p *providerStorage) CreateAccessToken(_ context.Context, req op.TokenRequest) (string, time.Time, error) {
	t := &accessToken{Subject: req.GetSubject(), Audience: req.GetAudience(), Scopes: req.GetScopes()}
	if ar, ok := req.(*authRequest); ok {
		t.ClientID = ar.ClientID
	}
	p.s.mem.addToken(t)
	return t.ID, t.ExpiresAt, nil
}

// Refresh tokens are out of scope for v1: apps mint their own session after
// login (RFC-001 §4), so the refresh_token grant is never advertised.
func (p *providerStorage) CreateAccessAndRefreshTokens(context.Context, op.TokenRequest, string) (string, string, time.Time, error) {
	return "", "", time.Time{}, oidc.ErrUnsupportedGrantType()
}

func (p *providerStorage) TokenRequestByRefreshToken(context.Context, string) (op.RefreshTokenRequest, error) {
	return nil, op.ErrInvalidRefreshToken
}

func (p *providerStorage) GetRefreshTokenInfo(context.Context, string, string) (string, string, error) {
	return "", "", op.ErrInvalidRefreshToken
}

func (p *providerStorage) TerminateSession(_ context.Context, userID, clientID string) error {
	p.s.mem.deleteTokensFor(userID, clientID)
	return nil
}

// TerminateSessionFromRequest handles RP-initiated logout. The library calls
// it only after accepting the request (valid hint, active client, registered
// post-logout URI), so aio's own session is ended here too.
func (p *providerStorage) TerminateSessionFromRequest(ctx context.Context, req *op.EndSessionRequest) (string, error) {
	if req.UserID != "" {
		p.s.mem.deleteTokensFor(req.UserID, "")
	}
	p.s.endAioSession(ctx, req.UserID)
	return req.RedirectURI, nil
}

func (p *providerStorage) RevokeToken(_ context.Context, tokenID, _, clientID string) *oidc.Error {
	t, ok := p.s.mem.token(tokenID)
	if !ok {
		return nil
	}
	if t.ClientID != clientID {
		return oidc.ErrInvalidClient().WithDescription("token was not issued for this client")
	}
	p.s.mem.deleteToken(tokenID)
	return nil
}

func (p *providerStorage) SigningKey(context.Context) (op.SigningKey, error) {
	return p.s.keys.signing(), nil
}

func (p *providerStorage) SignatureAlgorithms(context.Context) ([]jose.SignatureAlgorithm, error) {
	return []jose.SignatureAlgorithm{jose.RS256}, nil
}

func (p *providerStorage) KeySet(ctx context.Context) ([]op.Key, error) {
	return p.s.publishedKeys(ctx), nil
}

func (p *providerStorage) GetClientByClientID(ctx context.Context, clientID string) (op.Client, error) {
	c, err := p.s.ActiveClient(ctx, clientID)
	if err != nil {
		return nil, oidc.ErrInvalidClient().WithDescription("unknown or revoked client")
	}
	return &opClient{c: c, idTokenTTL: p.s.config.Auth.OIDC.IDTokenLifetime}, nil
}

func (p *providerStorage) AuthorizeClientIDSecret(ctx context.Context, clientID, secret string) error {
	if err := p.s.AuthenticateClient(ctx, clientID, secret); err != nil {
		return oidc.ErrInvalidClient().WithDescription("invalid client credentials")
	}
	return nil
}

// SetUserinfoFromScopes is deprecated in the library; SetUserinfoFromRequest
// is what fills the ID token's claims.
func (p *providerStorage) SetUserinfoFromScopes(context.Context, *oidc.UserInfo, string, string, []string) error {
	return nil
}

func (p *providerStorage) SetUserinfoFromRequest(ctx context.Context, info *oidc.UserInfo, req op.IDTokenRequest, scopes []string) error {
	return p.s.fillUserinfo(ctx, info, req.GetSubject(), scopes)
}

func (p *providerStorage) SetUserinfoFromToken(ctx context.Context, info *oidc.UserInfo, tokenID, _, _ string) error {
	t, ok := p.s.mem.token(tokenID)
	if !ok {
		return errors.New("token is invalid or has expired")
	}
	return p.s.fillUserinfo(ctx, info, t.Subject, t.Scopes)
}

func (p *providerStorage) SetIntrospectionFromToken(ctx context.Context, intro *oidc.IntrospectionResponse, tokenID, subject, clientID string) error {
	t, ok := p.s.mem.token(tokenID)
	if !ok {
		return errors.New("token is invalid or has expired")
	}
	for _, aud := range t.Audience {
		if aud == clientID {
			info := new(oidc.UserInfo)
			if err := p.s.fillUserinfo(ctx, info, subject, t.Scopes); err != nil {
				return err
			}
			intro.SetUserInfo(info)
			intro.Scope = t.Scopes
			intro.ClientID = t.ClientID
			intro.Expiration = oidc.FromTime(t.ExpiresAt)
			return nil
		}
	}
	return errors.New("token is not valid for this client")
}

func (p *providerStorage) GetPrivateClaimsFromScopes(context.Context, string, string, []string) (map[string]any, error) {
	return nil, nil
}

// JWT profile grants (service accounts signing their own assertions) are not
// supported; nothing advertises them.
func (p *providerStorage) GetKeyByIDAndClientID(context.Context, string, string) (*jose.JSONWebKey, error) {
	return nil, errors.New("jwt profile grant not supported")
}

func (p *providerStorage) ValidateJWTProfileScopes(_ context.Context, _ string, scopes []string) ([]string, error) {
	return nil, errors.New("jwt profile grant not supported")
}

func (p *providerStorage) Health(context.Context) error { return nil }

// fillUserinfo maps an aio user to OIDC claims: sub always, profile ->
// preferred_username (+ name when set), email -> email when the user has one.
// aio does not verify emails, so email_verified is always false.
func (s *Service) fillUserinfo(ctx context.Context, info *oidc.UserInfo, subject string, scopes []string) error {
	id, err := uuid.Parse(subject)
	if err != nil {
		return fmt.Errorf("invalid subject %q", subject)
	}
	u, err := s.users.Find(ctx, id)
	if err != nil {
		return fmt.Errorf("load user: %w", err)
	}
	for _, scope := range scopes {
		switch scope {
		case oidc.ScopeOpenID:
			info.Subject = u.ID.String()
		case oidc.ScopeProfile:
			info.PreferredUsername = u.Username
			info.Name = u.Name
		case oidc.ScopeEmail:
			if u.Email != "" {
				info.Email = u.Email
				info.EmailVerified = oidc.Bool(false)
			}
		}
	}
	return nil
}
