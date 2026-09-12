package handler

import (
	"encoding/json"
	"net/http"

	"github.com/all-in-one/internal/auth"
	httpHelper "github.com/all-in-one/internal/http"
	"github.com/all-in-one/internal/logging"
	"github.com/all-in-one/internal/ratelimit/model"
	"github.com/gorilla/mux"
)

type createExternalTargetRequest struct {
	Key         string           `json:"key"`
	App         string           `json:"app"`
	Name        string           `json:"name"`
	Description string           `json:"description"`
	Scope       model.Scope      `json:"scope"`
	Kind        model.Kind       `json:"kind"`
	LimitCount  int              `json:"limit_count"`
	WindowValue int              `json:"window_value"`
	WindowUnit  model.WindowUnit `json:"window_unit"`
	Enabled     *bool            `json:"enabled"`
}

// CreateExternalTarget godoc
// @Summary      Create an external rate limit target
// @Description  Create a DB-backed external target (no Registry entry) that other apps can call via /ratelimit/check (admin-only). Scope/kind are fixed at creation.
// @Tags         rate-limiting
// @Accept       json
// @Produce      json
// @Param        request  body      createExternalTargetRequest  true  "External target"
// @Security     BearerAuth || DirectAuth
// @Success      201  {object}  httpHelper.Response{data=model.Target}  "Created target"
// @Failure      400  {object}  httpHelper.Response  "Invalid request"
// @Failure      401  {object}  httpHelper.Response  "Unauthorized"
// @Failure      403  {object}  httpHelper.Response  "Forbidden (not an admin)"
// @Failure      409  {object}  httpHelper.Response  "Target key already exists"
// @Router       /ratelimit/targets/external [post]
func (h *Handler) CreateExternalTarget(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logging.GetLoggerFromContext(ctx)

	var req createExternalTargetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpHelper.SendError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	target := model.Target{
		Key: req.Key, App: req.App, Name: req.Name, Description: req.Description,
		Scope: req.Scope, Kind: req.Kind, Enabled: enabled,
		LimitCount: req.LimitCount, WindowValue: req.WindowValue, WindowUnit: req.WindowUnit,
	}

	claims, _ := auth.GetUserFromContext(ctx)

	created, err := h.service.CreateExternalTarget(ctx, target, claims.Username)
	if err != nil {
		log.Error().Err(err).Str("target", req.Key).Msg("failed to create external target")
		handleServiceError(w, err)
		return
	}

	h.metrics.configChanged.Add(ctx, 1, configChangedAttr(created.Key, "external.create"))
	httpHelper.SendJSON(w, httpHelper.Response{Success: true, Data: created}, http.StatusCreated)
}

// DeleteExternalTarget godoc
// @Summary      Delete an external rate limit target
// @Description  Delete a DB-backed external target by key (admin-only). Internal (Registry) targets cannot be deleted.
// @Tags         rate-limiting
// @Produce      json
// @Param        key  path  string  true  "Target key"
// @Security     BearerAuth || DirectAuth
// @Success      200  {object}  httpHelper.Response  "Deleted"
// @Failure      400  {object}  httpHelper.Response  "Not an external target"
// @Failure      401  {object}  httpHelper.Response  "Unauthorized"
// @Failure      403  {object}  httpHelper.Response  "Forbidden (not an admin)"
// @Failure      404  {object}  httpHelper.Response  "Unknown target"
// @Router       /ratelimit/targets/{key} [delete]
func (h *Handler) DeleteExternalTarget(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logging.GetLoggerFromContext(ctx)
	key := mux.Vars(r)["key"]

	if err := h.service.DeleteExternalTarget(ctx, key); err != nil {
		log.Error().Err(err).Str("target", key).Msg("failed to delete external target")
		handleServiceError(w, err)
		return
	}

	h.metrics.configChanged.Add(ctx, 1, configChangedAttr(key, "external.delete"))
	httpHelper.SendJSON(w, httpHelper.Response{Success: true}, http.StatusOK)
}
