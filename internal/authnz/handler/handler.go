package handler

import (
	"context"

	"github.com/all-in-one/internal/authnz/repository"
	"github.com/all-in-one/internal/config"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

// AccessResolver is the contract this package depends on for RBAC info in
// /users/me. Declared here (rather than importing internal/rbac/service
// directly) so *rbac/service.Resolver can satisfy it structurally without
// authnz ever importing anything from internal/rbac — the dependency runs
// the other way (server.go wires a concrete resolver in via
// SetAccessResolver after both services are constructed).
type AccessResolver interface {
	EffectiveFeatures(ctx context.Context, userID uuid.UUID) (isAdmin bool, groupID uuid.UUID, groupName string, featureKeys []string, err error)
}

type Handler struct {
	storage repository.Storage
	config  config.Config
	metrics *handlerMetrics

	accessResolver AccessResolver
}

func NewHandler(storage repository.Storage, config config.Config) *Handler {
	return &Handler{
		storage: storage,
		config:  config,
		metrics: newHandlerMetrics(),
	}
}

// SetAccessResolver wires in the RBAC resolver used by GetCurrentUser to
// populate is_admin/group/features on /users/me. Called once from
// cmd/all-in-one/server/server.go after both services are constructed —
// left nil (and defensively handled) in any test that doesn't set it.
func (h *Handler) SetAccessResolver(resolver AccessResolver) {
	h.accessResolver = resolver
}

func (h *Handler) RegisterPublicRoutes(router *mux.Router) {
	router.HandleFunc("/users", h.RegisterUser).Methods("POST")
	router.HandleFunc("/sessions", h.CreateSession).Methods("POST")
	router.HandleFunc("/sessions/refresh", h.RefreshToken).Methods("POST")
	router.HandleFunc("/sessions/verify", h.VerifySession).Methods("GET")
	router.HandleFunc("/sessions/2fa/verify", h.VerifyTOTP).Methods("POST")
	router.HandleFunc("/sessions/2fa/recovery", h.VerifyRecoveryCode).Methods("POST")
}

func (h *Handler) RegisterAuthenticatedRoutes(router *mux.Router) {
	router.HandleFunc("/users/me", h.GetCurrentUser).Methods("GET")
	router.HandleFunc("/users/reset_password", h.ResetPasswordUser).Methods("POST")
	router.HandleFunc("/users/2fa/setup", h.SetupTOTP).Methods("POST")
	router.HandleFunc("/users/2fa/verify-setup", h.VerifyTOTPSetup).Methods("POST")
	router.HandleFunc("/users/2fa", h.DisableTOTP).Methods("DELETE")
	router.HandleFunc("/users/2fa/status", h.GetTOTPStatus).Methods("GET")
	router.HandleFunc("/users/2fa/recovery-codes/regenerate", h.RegenerateRecoveryCodes).Methods("POST")

	router.HandleFunc("/sessions", h.DeleteSession).Methods("DELETE")
}
