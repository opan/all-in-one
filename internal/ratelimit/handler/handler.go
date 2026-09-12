package handler

import (
	"context"
	"net/http"

	"github.com/all-in-one/internal/config"
	"github.com/all-in-one/internal/ratelimit/model"
	"github.com/gorilla/mux"
)

// Service is the contract the handler depends on. Defined here (rather than
// importing internal/ratelimit/service directly) so *service.Service can
// satisfy it structurally without an import cycle — service.go constructs
// the handler by passing itself as this interface (mirrors
// internal/rbac/handler's Service pattern).
type Service interface {
	ListTargets(ctx context.Context) ([]model.Target, error)
	UpdateTarget(ctx context.Context, key string, patch model.TargetPatch, updatedBy string) (model.Target, error)
	ResetCounters(ctx context.Context, key string) error
	ResetDefaults(ctx context.Context, key string) (model.Target, error)

	// External rate-limit check API (P11).
	EffectiveRule(key string) (model.EffectiveRule, bool)
	CheckExternal(ctx context.Context, targetKey, bucketKey string) (model.CheckResponse, error)

	// External target + app token admin (P12).
	CreateExternalTarget(ctx context.Context, t model.Target, createdBy string) (model.Target, error)
	DeleteExternalTarget(ctx context.Context, key string) error
	CreateToken(ctx context.Context, app, name, scopePrefix, createdBy string) (model.AppToken, string, error)
	ListTokens(ctx context.Context) ([]model.AppToken, error)
	RevokeToken(ctx context.Context, id string) error
}

// Handler serves the admin-only Rate Limiting management API
// (/api/v1/ratelimit/*). Every route it registers is expected to already be
// gated by RequireAdmin (see internal/rbac/middleware) — the handler itself
// does not re-check admin status.
type Handler struct {
	service Service
	config  config.Config
	metrics *handlerMetrics
}

func NewHandler(service Service, config config.Config) *Handler {
	return &Handler{
		service: service,
		config:  config,
		metrics: newHandlerMetrics(),
	}
}

// RegisterAdminRoutes registers the Rate Limiting management API. Callers
// must apply admin-only gating (RequireAdmin) to router beforehand.
func (h *Handler) RegisterAdminRoutes(router *mux.Router) {
	router.HandleFunc("/ratelimit/targets", h.ListTargets).Methods(http.MethodGet)
	router.HandleFunc("/ratelimit/targets/{key}", h.UpdateTarget).Methods(http.MethodPatch)
	router.HandleFunc("/ratelimit/targets/{key}/reset", h.ResetCounters).Methods(http.MethodPost)
	router.HandleFunc("/ratelimit/targets/{key}/reset-defaults", h.ResetDefaults).Methods(http.MethodPost)

	// External target management (P12). The /external create route is
	// registered before the {key} delete route so it can't be shadowed.
	router.HandleFunc("/ratelimit/targets/external", h.CreateExternalTarget).Methods(http.MethodPost)
	router.HandleFunc("/ratelimit/targets/{key}", h.DeleteExternalTarget).Methods(http.MethodDelete)

	// App token management (P12).
	router.HandleFunc("/ratelimit/tokens", h.ListTokens).Methods(http.MethodGet)
	router.HandleFunc("/ratelimit/tokens", h.CreateToken).Methods(http.MethodPost)
	router.HandleFunc("/ratelimit/tokens/{id}", h.RevokeToken).Methods(http.MethodDelete)
}

// RegisterCheckRoutes registers the external rate-limit check API. Callers
// must apply app-token gating (middleware.AppTokenAuth) to router beforehand —
// this endpoint is authenticated by X-API-Key, not a user JWT (P13).
func (h *Handler) RegisterCheckRoutes(router *mux.Router) {
	router.HandleFunc("/ratelimit/check", h.Check).Methods(http.MethodPost)
}
