package service

import (
	"net/url"
	"time"

	"github.com/all-in-one/internal/oidc/model"
	"github.com/zitadel/oidc/v3/pkg/oidc"
	"github.com/zitadel/oidc/v3/pkg/op"
)

// LoginPath is aio's SPA page that runs the login for an auth request.
const LoginPath = "/oauth/login"

// opClient adapts a registered client to the library's op.Client: a
// confidential web app using the authorization code flow with
// client_secret_basic, whose ID tokens carry the profile claims directly so
// the app doesn't need a userinfo call.
type opClient struct {
	c          model.Client
	idTokenTTL time.Duration
}

func (c *opClient) GetID() string                       { return c.c.ID }
func (c *opClient) RedirectURIs() []string              { return c.c.RedirectURIs }
func (c *opClient) PostLogoutRedirectURIs() []string    { return c.c.PostLogoutRedirectURIs }
func (c *opClient) ApplicationType() op.ApplicationType { return op.ApplicationTypeWeb }
func (c *opClient) AuthMethod() oidc.AuthMethod         { return oidc.AuthMethodBasic }
func (c *opClient) ResponseTypes() []oidc.ResponseType {
	return []oidc.ResponseType{oidc.ResponseTypeCode}
}
func (c *opClient) GrantTypes() []oidc.GrantType         { return []oidc.GrantType{oidc.GrantTypeCode} }
func (c *opClient) AccessTokenType() op.AccessTokenType  { return op.AccessTokenTypeBearer }
func (c *opClient) IDTokenLifetime() time.Duration       { return c.idTokenTTL }
func (c *opClient) DevMode() bool                        { return false }
func (c *opClient) IsScopeAllowed(string) bool           { return false }
func (c *opClient) IDTokenUserinfoClaimsAssertion() bool { return true }
func (c *opClient) ClockSkew() time.Duration             { return 0 }

func (c *opClient) LoginURL(authRequestID string) string {
	return LoginPath + "?authRequestID=" + url.QueryEscape(authRequestID)
}

func (c *opClient) RestrictAdditionalIdTokenScopes() func([]string) []string {
	return func(s []string) []string { return s }
}

func (c *opClient) RestrictAdditionalAccessTokenScopes() func([]string) []string {
	return func(s []string) []string { return s }
}
