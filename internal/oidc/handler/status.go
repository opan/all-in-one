package handler

import (
	"errors"
	"net/http"

	httpHelper "github.com/all-in-one/internal/http"
	"github.com/all-in-one/internal/logging"
	"github.com/all-in-one/internal/oidc"
	"github.com/gorilla/mux"
)

// GetUserStatus godoc
// @Summary      Check whether an app's user is an aio admin
// @Description  For apps that log in through aio (RFC-002). Reports whether the user (the ID token's `sub`) is an aio admin right now and whether they are blocked; a blocked user is never an admin. Authenticated with the app's OIDC client id and secret (HTTP Basic). Apps call it on every admin request, so the response must not be cached.
// @Tags         oidc
// @Produce      json
// @Param        sub  path  string  true  "aio user id (the ID token's sub)"
// @Security     ClientAuth
// @Success      200  {object}  httpHelper.Response{data=model.UserStatus}  "Current status"
// @Failure      401  {object}  httpHelper.Response  "Missing, wrong or revoked client credentials"
// @Failure      404  {object}  httpHelper.Response  "Unknown user"
// @Router       /oidc/users/{sub}/status [get]
func (h *Handler) GetUserStatus(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logging.GetLoggerFromContext(ctx)
	w.Header().Set("Cache-Control", "no-store")

	clientID, secret, ok := r.BasicAuth()
	if !ok {
		h.rejectClient(w, r)
		return
	}
	if err := h.service.AuthenticateClient(ctx, clientID, secret); err != nil {
		if !errors.Is(err, oidc.ErrClientNotFound) && !errors.Is(err, oidc.ErrClientRevoked) &&
			!errors.Is(err, oidc.ErrInvalidClientSecret) {
			log.Error().Err(err).Msg("oidc: authenticate client for user status failed")
			h.metrics.userStatus(ctx, "", "error")
			httpHelper.SendError(w, "internal server error", http.StatusInternalServerError)
			return
		}
		h.rejectClient(w, r)
		return
	}

	status, err := h.service.UserStatus(ctx, mux.Vars(r)["sub"])
	switch {
	case errors.Is(err, oidc.ErrUserNotFound):
		h.metrics.userStatus(ctx, clientID, "unknown_user")
		httpHelper.SendError(w, "user not found", http.StatusNotFound)
		return
	case err != nil:
		log.Error().Err(err).Str("client_id", clientID).Msg("oidc: user status failed")
		h.metrics.userStatus(ctx, clientID, "error")
		httpHelper.SendError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	result := "not_admin"
	if status.Admin {
		result = "admin"
	}
	h.metrics.userStatus(ctx, clientID, result)
	httpHelper.SendJSON(w, httpHelper.Response{Success: true, Data: status}, http.StatusOK)
}

func (h *Handler) rejectClient(w http.ResponseWriter, r *http.Request) {
	h.metrics.userStatus(r.Context(), "", "unauthorized")
	w.Header().Set("WWW-Authenticate", `Basic realm="all-in-one"`)
	httpHelper.SendError(w, "invalid client credentials", http.StatusUnauthorized)
}
