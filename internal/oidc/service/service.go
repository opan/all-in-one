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
	"github.com/all-in-one/internal/oidc/repository"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog"
	zoidc "github.com/zitadel/oidc/v3/pkg/oidc"
	"github.com/zitadel/oidc/v3/pkg/op"
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
	config   config.Config
	log      zerolog.Logger
	keys     *keySet
	mem      *memStore
	provider *op.Provider
	issuer   string
}

func NewService(ctx context.Context, db *sqlx.DB, cfg config.Config, log zerolog.Logger, users UserLookup) (*Service, error) {
	store, err := repository.NewRepo(db, cfg)
	if err != nil {
		return nil, err
	}
	s := &Service{
		store:  store,
		users:  users,
		config: cfg,
		log:    log,
		mem:    newMemStore(cfg.Auth.OIDC.AuthRequestTTL, cfg.Auth.OIDC.AccessTokenLifetime),
		issuer: strings.TrimRight(cfg.Auth.OIDC.Issuer, "/"),
	}
	if err := s.loadKeys(ctx); err != nil {
		return nil, err
	}
	if err := s.buildProvider(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Service) buildProvider() error {
	opCfg := &op.Config{
		// Encrypts opaque access tokens; derived from the JWT secret so no new
		// secret has to be configured.
		CryptoKey:       sha256.Sum256([]byte("aio-oidc|" + s.config.Auth.JWTSecret)),
		CodeMethodS256:  true,
		SupportedScopes: []string{zoidc.ScopeOpenID, zoidc.ScopeProfile, zoidc.ScopeEmail},
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

// Handler serves discovery and every provider endpoint.
func (s *Service) Handler() http.Handler {
	return s.provider
}

// AuthRequestInfo is what aio's login page needs to render an auth request:
// which app is asking, and whether it asked for the signup form.
type AuthRequestInfo struct {
	ID         string `json:"id"`
	ClientID   string `json:"client_id"`
	ClientName string `json:"client_name"`
	Signup     bool   `json:"signup"`
}

func (s *Service) AuthRequestInfo(ctx context.Context, id string) (AuthRequestInfo, error) {
	r, ok := s.mem.request(id)
	if !ok {
		return AuthRequestInfo{}, oidc.ErrAuthRequestNotFound
	}
	c, err := s.ActiveClient(ctx, r.ClientID)
	if err != nil {
		return AuthRequestInfo{}, err
	}
	return AuthRequestInfo{ID: r.ID, ClientID: c.ID, ClientName: c.Name, Signup: r.wantsSignup()}, nil
}

// CompleteAuthRequest attaches the logged-in aio user to an auth request and
// returns the URL the browser must visit next; the provider then issues the
// authorization code and redirects back to the app.
func (s *Service) CompleteAuthRequest(ctx context.Context, id, userID string) (string, error) {
	r, ok := s.mem.request(id)
	if !ok {
		return "", oidc.ErrAuthRequestNotFound
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
	if !s.mem.complete(id, userID) {
		return "", oidc.ErrAuthRequestNotFound
	}
	return op.AuthCallbackURL(s.provider)(op.ContextWithIssuer(ctx, s.issuer), id), nil
}
