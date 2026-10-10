package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/all-in-one/internal/auth"
	httpHelper "github.com/all-in-one/internal/http"
	"github.com/all-in-one/internal/logging"
	"github.com/all-in-one/internal/oidc"
	"github.com/all-in-one/internal/oidc/model"
	"github.com/gorilla/mux"
)

// createClientResponse carries the client secret, shown exactly once, plus
// the issuer URL the app must be configured with.
type createClientResponse struct {
	Client       model.Client `json:"client"`
	ClientSecret string       `json:"client_secret"`
	Issuer       string       `json:"issuer"`
	Notice       string       `json:"notice"`
}

// ListClients godoc
// @Summary      List apps that log in through aio
// @Description  Every registered OIDC client, including revoked ones. Secrets are never returned (admin-only).
// @Tags         oidc
// @Produce      json
// @Security     BearerAuth || DirectAuth
// @Success      200  {object}  httpHelper.Response{data=[]model.Client}  "Clients"
// @Failure      401  {object}  httpHelper.Response  "Unauthorized"
// @Failure      403  {object}  httpHelper.Response  "Forbidden (not an admin)"
// @Router       /oidc/clients [get]
func (h *Handler) ListClients(w http.ResponseWriter, r *http.Request) {
	clients, err := h.service.ListClients(r.Context())
	if err != nil {
		logging.GetLoggerFromContext(r.Context()).Error().Err(err).Msg("oidc: list clients failed")
		httpHelper.SendError(w, "failed to list clients", http.StatusInternalServerError)
		return
	}
	httpHelper.SendJSON(w, httpHelper.Response{Success: true, Data: clients}, http.StatusOK)
}

// CreateClient godoc
// @Summary      Register an app to log in through aio
// @Description  Creates an OIDC client. The client secret is returned ONCE in this response and can never be retrieved again (admin-only).
// @Tags         oidc
// @Accept       json
// @Produce      json
// @Param        request  body      model.CreateClientInput  true  "Client id, name and redirect URIs"
// @Security     BearerAuth || DirectAuth
// @Success      201  {object}  httpHelper.Response{data=createClientResponse}  "Created client (secret shown once)"
// @Failure      400  {object}  httpHelper.Response  "Invalid id, name, redirect URI or branding"
// @Failure      401  {object}  httpHelper.Response  "Unauthorized"
// @Failure      403  {object}  httpHelper.Response  "Forbidden (not an admin)"
// @Failure      409  {object}  httpHelper.Response  "Client id already taken"
// @Router       /oidc/clients [post]
func (h *Handler) CreateClient(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var in model.CreateClientInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		httpHelper.SendError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	claims, _ := auth.GetUserFromContext(ctx)
	c, secret, err := h.service.CreateClient(ctx, in, claims.Username)
	if err != nil {
		h.sendClientError(w, r, err)
		return
	}
	h.metrics.clientChanged(ctx, "create")
	httpHelper.SendJSON(w, httpHelper.Response{Success: true, Data: createClientResponse{
		Client: c, ClientSecret: secret, Issuer: h.config.Auth.OIDC.Issuer,
		Notice: "Store this client secret now; it will not be shown again.",
	}}, http.StatusCreated)
}

// UpdateClient godoc
// @Summary      Change how an app is presented on aio's login pages
// @Description  Updates an OIDC client's display name, brand colour (#rrggbb) or icon (an emoji or 1-2 characters). Omitted fields are kept; an empty string clears the colour or icon (admin-only).
// @Tags         oidc
// @Accept       json
// @Produce      json
// @Param        id       path  string                   true  "Client id"
// @Param        request  body  model.UpdateClientInput  true  "Fields to change"
// @Security     BearerAuth || DirectAuth
// @Success      200  {object}  httpHelper.Response{data=model.Client}  "Updated client"
// @Failure      400  {object}  httpHelper.Response  "Invalid name or branding"
// @Failure      401  {object}  httpHelper.Response  "Unauthorized"
// @Failure      403  {object}  httpHelper.Response  "Forbidden (not an admin)"
// @Failure      404  {object}  httpHelper.Response  "No such active client"
// @Router       /oidc/clients/{id} [patch]
func (h *Handler) UpdateClient(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var in model.UpdateClientInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		httpHelper.SendError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	c, err := h.service.UpdateClient(ctx, mux.Vars(r)["id"], in)
	if err != nil {
		h.sendClientError(w, r, err)
		return
	}
	h.metrics.clientChanged(ctx, "update")
	httpHelper.SendJSON(w, httpHelper.Response{Success: true, Data: c}, http.StatusOK)
}

// RevokeClient godoc
// @Summary      Revoke an app's ability to log in through aio
// @Description  Revokes an OIDC client; logins already in flight for it fail. The record is kept for the audit trail (admin-only).
// @Tags         oidc
// @Produce      json
// @Param        id  path  string  true  "Client id"
// @Security     BearerAuth || DirectAuth
// @Success      200  {object}  httpHelper.Response  "Revoked"
// @Failure      401  {object}  httpHelper.Response  "Unauthorized"
// @Failure      403  {object}  httpHelper.Response  "Forbidden (not an admin)"
// @Failure      404  {object}  httpHelper.Response  "No such active client"
// @Router       /oidc/clients/{id} [delete]
func (h *Handler) RevokeClient(w http.ResponseWriter, r *http.Request) {
	if err := h.service.RevokeClient(r.Context(), mux.Vars(r)["id"]); err != nil {
		h.sendClientError(w, r, err)
		return
	}
	h.metrics.clientChanged(r.Context(), "revoke")
	httpHelper.SendJSON(w, httpHelper.Response{Success: true}, http.StatusOK)
}

func (h *Handler) sendClientError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, oidc.ErrInvalidClientID), errors.Is(err, oidc.ErrInvalidClientName), errors.Is(err, oidc.ErrInvalidRedirectURI),
		errors.Is(err, oidc.ErrInvalidBranding):
		httpHelper.SendError(w, err.Error(), http.StatusBadRequest)
	case errors.Is(err, oidc.ErrClientExists):
		httpHelper.SendError(w, "a client with this id already exists", http.StatusConflict)
	case errors.Is(err, oidc.ErrClientNotFound), errors.Is(err, oidc.ErrClientRevoked):
		httpHelper.SendError(w, "no such active client", http.StatusNotFound)
	default:
		logging.GetLoggerFromContext(r.Context()).Error().Err(err).Msg("oidc: client operation failed")
		httpHelper.SendError(w, "internal server error", http.StatusInternalServerError)
	}
}
