package service

import (
	"context"
	"crypto/sha256"
	"fmt"
	"net/http"
	"strings"

	authnzModel "github.com/all-in-one/internal/authnz/model"
	"github.com/all-in-one/internal/config"
	"github.com/all-in-one/internal/oidc"
	"github.com/all-in-one/internal/oidc/handler"
	"github.com/all-in-one/internal/oidc/model"
	"github.com/all-in-one/internal/oidc/repository"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog"
	zoidc "github.com/zitadel/oidc/v3/pkg/oidc"
	"github.com/zitadel/oidc/v3/pkg/op"
	"golang.org/x/text/language"
)

// Endpoint paths, relative to the issuer. Discovery is served at the fixed
// /.well-known/openid-configuration path; everything else lives under the
// project's /api/v1 base path.
const (
	EndpointPrefix  = "/api/v1/oauth2/"
	DiscoveryPath   = "/.well-known/openid-configuration"
	endpointBaseRel = "api/v1/oauth2/"
)

// UserLookup is the subset of the authnz user repository the provider needs.
type UserLookup interface {
	Find(ctx context.Context, id uuid.UUID) (authnzModel.User, error)
}

// Service is aio's OpenID Connect provider (RFC-001, Option A): the client
// registry, signing keys, and the storage the zitadel/oidc provider runs on.
type Service struct {
	store    repository.Storage
	users    UserLookup
	sessions SessionStore
	config   config.Config
	log      zerolog.Logger
	keys     *keySet
	mem      *memStore
	provider *op.Provider
	issuer   string
	Handler  *handler.Handler
}

func NewService(ctx context.Context, db *sqlx.DB, cfg config.Config, log zerolog.Logger, users UserLookup, sessions SessionStore) (*Service, error) {
	store, err := repository.NewRepo(db, cfg)
	if err != nil {
		return nil, err
	}
	s := &Service{
		store:    store,
		users:    users,
		sessions: sessions,
		config:   cfg,
		log:      log,
		mem:      newMemStore(cfg.Auth.OIDC.AuthRequestTTL, cfg.Auth.OIDC.AccessTokenLifetime),
		issuer:   strings.TrimRight(cfg.Auth.OIDC.Issuer, "/"),
	}
	if err := s.loadKeys(ctx); err != nil {
		return nil, err
	}
	if err := s.buildProvider(); err != nil {
		return nil, err
	}
	s.Handler = handler.NewHandler(s, cfg)
	return s, nil
}

// NewClientRegistry returns a Service that can only manage clients: no
// signing keys, no provider. Used by the CLI, which must work even when the
// provider itself is disabled.
func NewClientRegistry(db *sqlx.DB, cfg config.Config, log zerolog.Logger) (*Service, error) {
	store, err := repository.NewRepo(db, cfg)
	if err != nil {
		return nil, err
	}
	return &Service{store: store, config: cfg, log: log}, nil
}

func (s *Service) buildProvider() error {
	opCfg := &op.Config{
		// Encrypts opaque access tokens; derived from the JWT secret so no new
		// secret has to be configured.
		CryptoKey:      sha256.Sum256([]byte("aio-oidc|" + s.config.Auth.JWTSecret)),
		CodeMethodS256: true,
		// Where logout lands when the app sent no registered post-logout URL.
		DefaultLogoutRedirectURI: "/login",
		SupportedScopes:          []string{zoidc.ScopeOpenID, zoidc.ScopeProfile, zoidc.ScopeEmail},
	}
	opts := []op.Option{
		op.WithCustomEndpoints(
			op.NewEndpoint(endpointBaseRel+"authorize"),
			op.NewEndpoint(endpointBaseRel+"token"),
			op.NewEndpoint(endpointBaseRel+"userinfo"),
			op.NewEndpoint(endpointBaseRel+"revoke"),
			op.NewEndpoint(endpointBaseRel+"end_session"),
			op.NewEndpoint(endpointBaseRel+"keys"),
		),
		op.WithLogger(newSlogBridge(s.log)),
	}
	if strings.HasPrefix(s.issuer, "http://") {
		opts = append(opts, op.WithAllowInsecure())
	}
	p, err := op.NewProvider(opCfg, &providerStorage{s: s}, op.StaticIssuer(s.issuer), opts...)
	if err != nil {
		return fmt.Errorf("create oidc provider: %w", err)
	}
	s.provider = p
	return nil
}

// ProviderHandler serves discovery and every provider endpoint.
func (s *Service) ProviderHandler() http.Handler {
	return s.withBrowserBinding(s.withAioLogout(s.provider))
}

func (s *Service) AuthRequestInfo(ctx context.Context, id string) (model.AuthRequestInfo, error) {
	r, ok := s.mem.request(id)
	if !ok {
		return model.AuthRequestInfo{}, oidc.ErrAuthRequestNotFound
	}
	c, err := s.ActiveClient(ctx, r.ClientID)
	if err != nil {
		return model.AuthRequestInfo{}, err
	}
	return model.AuthRequestInfo{ID: r.ID, ClientID: c.ID, ClientName: c.Name, Branding: c.Branding,
		Locale: r.Locale, Signup: r.wantsSignup()}, nil
}

// supportedLocales are the languages aio's login pages are written in.
var supportedLocales = language.NewMatcher([]language.Tag{language.English, language.Indonesian})

// supportedLocale picks the first of the app's ui_locales that aio's login
// pages support ("en" or "id"), or "" when it asked for none of them.
func supportedLocale(requested zoidc.Locales) string {
	for _, tag := range requested {
		if _, i, c := supportedLocales.Match(tag); c >= language.High {
			return []string{"en", "id"}[i]
		}
	}
	return ""
}

// CompleteAuthRequest attaches the logged-in aio user to an auth request and
// returns the URL the browser must visit next; the provider then issues the
// authorization code and redirects back to the app.
// browserID is the browser cookie of the caller (see binding.go): only the
// browser that started the login may finish it.
func (s *Service) CompleteAuthRequest(ctx context.Context, id, userID, browserID string) (string, error) {
	r, ok := s.mem.request(id)
	if !ok {
		return "", oidc.ErrAuthRequestNotFound
	}
	if !r.startedBy(browserID) {
		return "", oidc.ErrOtherBrowser
	}
	if _, err := s.ActiveClient(ctx, r.ClientID); err != nil {
		return "", err
	}
	uid, err := uuid.Parse(userID)
	if err != nil {
		return "", fmt.Errorf("invalid user id: %w", err)
	}
	u, err := s.users.Find(ctx, uid)
	if err != nil {
		return "", fmt.Errorf("load user: %w", err)
	}
	if u.Blocked {
		return "", oidc.ErrUserBlocked
	}
	if s.config.DemoMode.Enabled && strings.EqualFold(u.Username, s.config.DemoMode.Username) {
		return "", oidc.ErrDemoAccount
	}
	if !s.mem.complete(id, userID) {
		return "", oidc.ErrAuthRequestNotFound
	}
	return op.AuthCallbackURL(s.provider)(op.ContextWithIssuer(ctx, s.issuer), id), nil
}
