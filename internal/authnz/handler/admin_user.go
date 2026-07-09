package handler

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/mail"
	"strings"

	httpHelper "github.com/all-in-one/internal/http"
	"github.com/all-in-one/internal/logging"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// UpdateUserEmailRequest is the body for PATCH /admin/users/{id}.
type UpdateUserEmailRequest struct {
	Email string `json:"email"`
}

// RegisterAdminRoutes wires the admin-only user-management endpoints. Mounted on
// the RequireAdmin subrouter in cmd/all-in-one/server/server.go, so every route
// here is already gated to admins by the time it runs.
func (h *Handler) RegisterAdminRoutes(router *mux.Router) {
	router.HandleFunc("/admin/users/{id}", h.UpdateUserEmail).Methods("PATCH")
	router.HandleFunc("/admin/users/{id}/block", h.BlockUser).Methods("POST")
	router.HandleFunc("/admin/users/{id}/unblock", h.UnblockUser).Methods("POST")
}

func parseUserIDParam(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(mux.Vars(r)["id"])
	if err != nil {
		httpHelper.SendError(w, "invalid user id", http.StatusBadRequest)
		return uuid.Nil, false
	}
	return id, true
}

// isUniqueViolation reports whether err is a DB unique-constraint violation,
// matched driver-agnostically (sqlite: "UNIQUE constraint failed"; postgres/lib/pq:
// "duplicate key value violates unique constraint" / SQLSTATE 23505) so the handler
// stays free of driver imports.
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "unique constraint") ||
		strings.Contains(msg, "duplicate key") ||
		strings.Contains(msg, "23505")
}

// UpdateUserEmail godoc
// @Summary      Update a user's email (admin)
// @Description  Admin-only. Change another user's email address.
// @Tags         admin-users
// @Accept       json
// @Produce      json
// @Security     BearerAuth || DirectAuth
// @Param        id     path      string                  true  "User ID"
// @Param        body   body      UpdateUserEmailRequest  true  "New email"
// @Success      200    {object}  httpHelper.Response     "Email updated"
// @Failure      400    {object}  httpHelper.Response     "Invalid user id or email"
// @Failure      404    {object}  httpHelper.Response     "User not found"
// @Failure      409    {object}  httpHelper.Response     "Email already in use"
// @Router       /admin/users/{id} [patch]
func (h *Handler) UpdateUserEmail(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logging.GetLoggerFromContext(ctx)

	id, ok := parseUserIDParam(w, r)
	if !ok {
		return
	}

	var req UpdateUserEmailRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpHelper.SendError(w, "invalid request payload", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	req.Email = strings.TrimSpace(req.Email)
	addr, err := mail.ParseAddress(req.Email)
	if err != nil || addr.Address != req.Email {
		httpHelper.SendError(w, "invalid email address", http.StatusBadRequest)
		return
	}

	if _, err := h.storage.UserRepo().Find(ctx, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			httpHelper.SendError(w, "user not found", http.StatusNotFound)
			return
		}
		log.Error().Err(err).Str("user_id", id.String()).Msg("failed to load user for email update")
		httpHelper.SendError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	if err := h.storage.UserRepo().UpdateEmail(ctx, id, req.Email); err != nil {
		if isUniqueViolation(err) {
			httpHelper.SendError(w, "email already in use", http.StatusConflict)
			return
		}
		log.Error().Err(err).Str("user_id", id.String()).Msg("failed to update user email")
		httpHelper.SendError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	h.metrics.adminEmailUpdated.Add(ctx, 1)
	log.Info().Str("user_id", id.String()).Msg("admin updated user email")
	httpHelper.SendJSON(w, httpHelper.Response{Success: true, Message: "email updated"}, http.StatusOK)
}

// BlockUser godoc
// @Summary      Block a user (admin)
// @Description  Admin-only. Block a user's login and terminate all their active sessions. Administrators cannot be blocked.
// @Tags         admin-users
// @Produce      json
// @Security     BearerAuth || DirectAuth
// @Param        id   path      string               true  "User ID"
// @Success      200  {object}  httpHelper.Response  "User blocked"
// @Failure      400  {object}  httpHelper.Response  "Invalid user id"
// @Failure      404  {object}  httpHelper.Response  "User not found"
// @Failure      409  {object}  httpHelper.Response  "Cannot block an administrator"
// @Router       /admin/users/{id}/block [post]
func (h *Handler) BlockUser(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logging.GetLoggerFromContext(ctx)

	id, ok := parseUserIDParam(w, r)
	if !ok {
		return
	}

	if _, err := h.storage.UserRepo().Find(ctx, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			httpHelper.SendError(w, "user not found", http.StatusNotFound)
			return
		}
		log.Error().Err(err).Str("user_id", id.String()).Msg("failed to load user for block")
		httpHelper.SendError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	// Guard: administrators are never blockable (must have admin access removed
	// first). This also makes self-block impossible, so admin lockout is
	// unreachable. accessResolver is always wired in production; a nil resolver
	// (tests) skips the check.
	if h.accessResolver != nil {
		isAdmin, _, _, _, err := h.accessResolver.EffectiveFeatures(ctx, id)
		if err != nil {
			log.Error().Err(err).Str("user_id", id.String()).Msg("failed to resolve admin status for block")
			httpHelper.SendError(w, "internal server error", http.StatusInternalServerError)
			return
		}
		if isAdmin {
			httpHelper.SendError(w, "cannot block an administrator; remove admin access first", http.StatusConflict)
			return
		}
	}

	if err := h.storage.UserRepo().SetBlocked(ctx, id, true); err != nil {
		log.Error().Err(err).Str("user_id", id.String()).Msg("failed to block user")
		httpHelper.SendError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	// Terminate every active session so the block takes effect immediately
	// (reuses the same invalidation path as password-reset). New logins are
	// already rejected by the blocked check in CreateSession.
	if err := h.storage.SessionRepo().DeleteByUserID(ctx, id); err != nil {
		log.Error().Err(err).Str("user_id", id.String()).Msg("failed to terminate sessions for blocked user")
		httpHelper.SendError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	h.metrics.adminUserBlocked.Add(ctx, 1, metric.WithAttributes(attribute.String("result", "success")))
	log.Info().Str("user_id", id.String()).Msg("admin blocked user")
	httpHelper.SendJSON(w, httpHelper.Response{Success: true, Message: "user blocked"}, http.StatusOK)
}

// UnblockUser godoc
// @Summary      Unblock a user (admin)
// @Description  Admin-only. Re-enable a blocked user's login.
// @Tags         admin-users
// @Produce      json
// @Security     BearerAuth || DirectAuth
// @Param        id   path      string               true  "User ID"
// @Success      200  {object}  httpHelper.Response  "User unblocked"
// @Failure      400  {object}  httpHelper.Response  "Invalid user id"
// @Failure      404  {object}  httpHelper.Response  "User not found"
// @Router       /admin/users/{id}/unblock [post]
func (h *Handler) UnblockUser(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logging.GetLoggerFromContext(ctx)

	id, ok := parseUserIDParam(w, r)
	if !ok {
		return
	}

	if _, err := h.storage.UserRepo().Find(ctx, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			httpHelper.SendError(w, "user not found", http.StatusNotFound)
			return
		}
		log.Error().Err(err).Str("user_id", id.String()).Msg("failed to load user for unblock")
		httpHelper.SendError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	if err := h.storage.UserRepo().SetBlocked(ctx, id, false); err != nil {
		log.Error().Err(err).Str("user_id", id.String()).Msg("failed to unblock user")
		httpHelper.SendError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	h.metrics.adminUserUnblocked.Add(ctx, 1)
	log.Info().Str("user_id", id.String()).Msg("admin unblocked user")
	httpHelper.SendJSON(w, httpHelper.Response{Success: true, Message: "user unblocked"}, http.StatusOK)
}
