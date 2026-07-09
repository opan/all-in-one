package handler

import (
	"context"
	"net/http"

	"github.com/all-in-one/internal/config"
	"github.com/all-in-one/internal/rbac/model"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

// Service is the contract the handler depends on. Defined here (rather than
// importing internal/rbac/service directly) so *service.Service can satisfy
// it structurally without an import cycle — service.go constructs the
// handler by passing itself as this interface, mirroring the AccessResolver
// pattern used for the authnz/rbac cross-package wiring.
type Service interface {
	ListFeatures(ctx context.Context) ([]model.Feature, error)

	ListGroups(ctx context.Context) ([]model.Group, error)
	GetGroup(ctx context.Context, id uuid.UUID) (model.Group, error)
	CreateGroup(ctx context.Context, name, description string, featureKeys []string) (model.Group, error)
	UpdateGroup(ctx context.Context, id uuid.UUID, name, description string) (model.Group, error)
	DeleteGroup(ctx context.Context, id uuid.UUID) error
	SetGroupFeatures(ctx context.Context, id uuid.UUID, featureKeys []string) (model.Group, error)

	ListUsers(ctx context.Context) ([]model.UserAccessRow, error)
	AssignUserGroup(ctx context.Context, userID uuid.UUID, groupID *uuid.UUID) error

	ListUserOverrides(ctx context.Context, userID uuid.UUID) ([]model.FeatureOverrideView, error)
	SetUserOverrides(ctx context.Context, userID uuid.UUID, overrides []model.FeatureOverrideView) error
}

// Handler serves the admin-only Access Management REST API
// (/api/v1/access/*). Every route it registers is expected to already be
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

// RegisterAdminRoutes registers the Access Management management API.
// Callers must apply admin-only gating (RequireAdmin) to router beforehand.
func (h *Handler) RegisterAdminRoutes(router *mux.Router) {
	router.HandleFunc("/access/features", h.ListFeatures).Methods(http.MethodGet)

	router.HandleFunc("/access/groups", h.ListGroups).Methods(http.MethodGet)
	router.HandleFunc("/access/groups", h.CreateGroup).Methods(http.MethodPost)
	router.HandleFunc("/access/groups/{id}", h.GetGroup).Methods(http.MethodGet)
	router.HandleFunc("/access/groups/{id}", h.UpdateGroup).Methods(http.MethodPut)
	router.HandleFunc("/access/groups/{id}", h.DeleteGroup).Methods(http.MethodDelete)
	router.HandleFunc("/access/groups/{id}/features", h.SetGroupFeatures).Methods(http.MethodPut)

	router.HandleFunc("/access/users", h.ListUsers).Methods(http.MethodGet)
	router.HandleFunc("/access/users/{id}/group", h.AssignUserGroup).Methods(http.MethodPut)
	router.HandleFunc("/access/users/{id}/overrides", h.ListUserOverrides).Methods(http.MethodGet)
	router.HandleFunc("/access/users/{id}/overrides", h.SetUserOverrides).Methods(http.MethodPut)
}

// getIDFromRequest extracts and parses the {id} path variable as a UUID.
func getIDFromRequest(r *http.Request) (uuid.UUID, error) {
	vars := mux.Vars(r)
	return uuid.Parse(vars["id"])
}
