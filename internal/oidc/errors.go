package oidc

import "errors"

// Sentinel errors shared by the oidc service (which returns them) and handler
// (which maps them to HTTP status codes). They live in this dependency-free
// top-level package so the handler can check them with errors.Is without
// importing the service package (same pattern as internal/ratelimit).
var (
	ErrClientNotFound      = errors.New("oidc client not found")
	ErrClientExists        = errors.New("oidc client already exists")
	ErrClientRevoked       = errors.New("oidc client revoked")
	ErrInvalidClientID     = errors.New("invalid client id")
	ErrInvalidClientName   = errors.New("invalid client name")
	ErrInvalidRedirectURI  = errors.New("invalid redirect uri")
	ErrInvalidClientSecret = errors.New("invalid client secret")
	ErrAuthRequestNotFound = errors.New("auth request not found or expired")
	ErrUserBlocked         = errors.New("user is blocked")
	// ErrDemoAccount: the shared demo account (demo_mode) must not log in to
	// other apps, where every visitor would share one account and its data.
	ErrDemoAccount = errors.New("the shared demo account can't log in to other apps")
)
