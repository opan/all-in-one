package handler

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/all-in-one/internal/auth"
	httpHelper "github.com/all-in-one/internal/http"
	"github.com/all-in-one/internal/logging"
	"github.com/all-in-one/internal/ratelimit/model"
	"github.com/gorilla/mux"
)

type createTokenRequest struct {
	App         string `json:"app"`
	Name        string `json:"name"`
	ScopePrefix string `json:"scope_prefix"`
}

// createTokenResponse carries the plaintext token, which is shown exactly once
// and is never recoverable afterwards.
type createTokenResponse struct {
	model.AppToken
	Token  string `json:"token"`
	Notice string `json:"notice"`
}

// ListTokens godoc
// @Summary      List app tokens
// @Description  List every app token (including revoked ones — the audit trail). The plaintext token and its hash are never returned (admin-only).
// @Tags         rate-limiting
// @Produce      json
// @Security     BearerAuth || DirectAuth
// @Success      200  {object}  httpHelper.Response{data=[]model.AppToken}  "Tokens"
// @Failure      401  {object}  httpHelper.Response  "Unauthorized"
// @Failure      403  {object}  httpHelper.Response  "Forbidden (not an admin)"
// @Router       /ratelimit/tokens [get]
func (h *Handler) ListTokens(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logging.GetLoggerFromContext(ctx)

	tokens, err := h.service.ListTokens(ctx)
	if err != nil {
		log.Error().Err(err).Msg("failed to list app tokens")
		httpHelper.SendError(w, "failed to list app tokens", http.StatusInternalServerError)
		return
	}
	httpHelper.SendJSON(w, httpHelper.Response{Success: true, Data: tokens}, http.StatusOK)
}

// CreateToken godoc
// @Summary      Create an app token
// @Description  Mint a new app token for another app to call the rate-limit check API. The plaintext token is returned ONCE in this response and can never be retrieved again (admin-only).
// @Tags         rate-limiting
// @Accept       json
// @Produce      json
// @Param        request  body      createTokenRequest  true  "App, name, optional scope prefix"
// @Security     BearerAuth || DirectAuth
// @Success      201  {object}  httpHelper.Response{data=createTokenResponse}  "Created token (plaintext shown once)"
// @Failure      400  {object}  httpHelper.Response  "Invalid request"
// @Failure      401  {object}  httpHelper.Response  "Unauthorized"
// @Failure      403  {object}  httpHelper.Response  "Forbidden (not an admin)"
// @Router       /ratelimit/tokens [post]
func (h *Handler) CreateToken(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logging.GetLoggerFromContext(ctx)

	var req createTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpHelper.SendError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(req.App) == "" || strings.TrimSpace(req.Name) == "" {
		httpHelper.SendError(w, "app and name are required", http.StatusBadRequest)
		return
	}

	claims, _ := auth.GetUserFromContext(ctx)

	tok, plaintext, err := h.service.CreateToken(ctx, req.App, req.Name, req.ScopePrefix, claims.Username)
	if err != nil {
		log.Error().Err(err).Str("app", req.App).Msg("failed to create app token")
		httpHelper.SendError(w, "failed to create app token", http.StatusInternalServerError)
		return
	}

	h.metrics.configChanged.Add(ctx, 1, configChangedAttr(tok.App, "token.create"))
	resp := createTokenResponse{
		AppToken: tok,
		Token:    plaintext,
		Notice:   "Store this token now — it will not be shown again.",
	}
	httpHelper.SendJSON(w, httpHelper.Response{Success: true, Data: resp}, http.StatusCreated)
}

// RevokeToken godoc
// @Summary      Revoke an app token
// @Description  Soft-revoke an app token by id (the audit row is kept). Takes effect immediately (admin-only).
// @Tags         rate-limiting
// @Produce      json
// @Param        id  path  string  true  "Token id"
// @Security     BearerAuth || DirectAuth
// @Success      200  {object}  httpHelper.Response  "Revoked"
// @Failure      401  {object}  httpHelper.Response  "Unauthorized"
// @Failure      403  {object}  httpHelper.Response  "Forbidden (not an admin)"
// @Router       /ratelimit/tokens/{id} [delete]
func (h *Handler) RevokeToken(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logging.GetLoggerFromContext(ctx)
	id := mux.Vars(r)["id"]

	if err := h.service.RevokeToken(ctx, id); err != nil {
		log.Error().Err(err).Str("id", id).Msg("failed to revoke app token")
		httpHelper.SendError(w, "failed to revoke app token", http.StatusInternalServerError)
		return
	}

	h.metrics.configChanged.Add(ctx, 1, configChangedAttr(id, "token.revoke"))
	httpHelper.SendJSON(w, httpHelper.Response{Success: true}, http.StatusOK)
}
