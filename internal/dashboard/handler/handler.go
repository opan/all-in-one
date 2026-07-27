package handler

import (
	"context"
	"net/http"

	"github.com/all-in-one/internal/auth"
	"github.com/all-in-one/internal/dashboard/model"
	httpHelper "github.com/all-in-one/internal/http"
	"github.com/all-in-one/internal/logging"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/rs/zerolog"
)

// SummaryProvider computes a per-user dashboard summary. Implemented by the
// dashboard service, which aggregates counts across the listing/chat/shortener
// storages and applies RBAC feature gating.
type SummaryProvider interface {
	Summary(ctx context.Context, userID uuid.UUID) (model.Summary, error)
}

type Handler struct {
	provider SummaryProvider
	log      zerolog.Logger
}

func NewHandler(provider SummaryProvider, log zerolog.Logger) *Handler {
	return &Handler{provider: provider, log: log}
}

// RegisterAuthenticatedRoutes wires the summary endpoint. It is authenticated
// but not feature-gated (registered on the selfRoutes subrouter in server.go),
// so a user with only some features still gets a 200 carrying just the sections
// they can access.
func (h *Handler) RegisterAuthenticatedRoutes(router *mux.Router) {
	router.HandleFunc("/dashboard/summary", h.Summary).Methods(http.MethodGet)
}

// Summary godoc
// @Summary      Get home dashboard summary
// @Description  Per-user counts for each app the user can access (listing topics, chat conversations and pending invites, shortener links). Sections are omitted for features the user lacks.
// @Tags         dashboard
// @Produce      json
// @Security     BearerAuth || DirectAuth
// @Success      200  {object}  httpHelper.Response{data=model.Summary}  "Dashboard summary"
// @Failure      401  {object}  httpHelper.Response                      "Unauthorized"
// @Failure      500  {object}  httpHelper.Response                      "Internal server error"
// @Router       /dashboard/summary [get]
func (h *Handler) Summary(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logging.GetLoggerFromContext(ctx)

	user, ok := auth.GetUserFromContext(ctx)
	if !ok {
		httpHelper.SendUnauthorized(w, "unauthorized")
		return
	}

	userID, err := uuid.Parse(user.UserID)
	if err != nil {
		log.Error().Err(err).Str("user_id", user.UserID).Msg("dashboard: invalid user id in context")
		httpHelper.SendError(w, "invalid user", http.StatusBadRequest)
		return
	}

	summary, err := h.provider.Summary(ctx, userID)
	if err != nil {
		log.Error().Err(err).Msg("dashboard: failed to build summary")
		httpHelper.SendError(w, "failed to build dashboard summary", http.StatusInternalServerError)
		return
	}

	httpHelper.SendJSON(w, httpHelper.Response{
		Success: true,
		Data:    summary,
	}, http.StatusOK)
}
