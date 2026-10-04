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

	CreateClient(ctx context.Context, in model.CreateClientInput, createdBy string) (model.Client, string, error)
	ListClients(ctx context.Context) ([]model.Client, error)
	RevokeClient(ctx context.Context, id string) error
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

// RegisterAdminRoutes: managing which apps may log users in through aio.
// Callers must apply RequireAdmin to the router beforehand.
func (h *Handler) RegisterAdminRoutes(r *mux.Router) {
	r.HandleFunc("/oidc/clients", h.ListClients).Methods(http.MethodGet)
	r.HandleFunc("/oidc/clients", h.CreateClient).Methods(http.MethodPost)
	r.HandleFunc("/oidc/clients/{id}", h.RevokeClient).Methods(http.MethodDelete)
}
