package handler

import (
	"context"
	"net/http"

	"github.com/all-in-one/internal/config"
	"github.com/all-in-one/internal/oidc/model"
	"github.com/gorilla/mux"
)

// Service is the contract the handler depends on, declared here so the
// service package can satisfy it structurally without an import cycle.
type Service interface {
	AuthRequestInfo(ctx context.Context, id string) (model.AuthRequestInfo, error)
	CompleteAuthRequest(ctx context.Context, id, userID string) (string, error)
}

type Handler struct {
	service Service
	config  config.Config
	metrics *handlerMetrics
}

func NewHandler(service Service, cfg config.Config) *Handler {
	return &Handler{service: service, config: cfg, metrics: newHandlerMetrics()}
}

// RegisterPublicRoutes: the login page reads an auth request before the user
// is logged in, to show which app is asking.
func (h *Handler) RegisterPublicRoutes(r *mux.Router) {
	r.HandleFunc("/oidc/auth-requests/{id}", h.GetAuthRequest).Methods(http.MethodGet)
}

// RegisterAuthenticatedRoutes: completing a login requires an aio session.
func (h *Handler) RegisterAuthenticatedRoutes(r *mux.Router) {
	r.HandleFunc("/oidc/auth-requests/{id}/complete", h.CompleteAuthRequest).Methods(http.MethodPost)
}
