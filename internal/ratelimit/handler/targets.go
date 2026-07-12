package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/all-in-one/internal/auth"
	httpHelper "github.com/all-in-one/internal/http"
	"github.com/all-in-one/internal/logging"
	"github.com/all-in-one/internal/ratelimit"
	"github.com/all-in-one/internal/ratelimit/model"
	"github.com/gorilla/mux"
)

// handleServiceError maps a service-layer error to the appropriate HTTP
// status: 404 for an unknown target key, 400 for an invalid window unit
// (only reachable if the patch's WindowUnit passes JSON decoding but isn't
// one of second/minute/hour/day), 500 otherwise.
func handleServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ratelimit.ErrUnknownTarget):
		httpHelper.SendError(w, "unknown rate limit target", http.StatusNotFound)
	case errors.Is(err, ratelimit.ErrInvalidWindowUnit):
		httpHelper.SendError(w, err.Error(), http.StatusBadRequest)
	default:
		httpHelper.SendError(w, "internal server error", http.StatusInternalServerError)
	}
}

// ListTargets godoc
// @Summary      List rate limit targets
// @Description  List every rate-limited target, merging its code-defined identity (scope/kind/route) with its current effective rule (admin-only)
// @Tags         rate-limiting
// @Produce      json
// @Security     BearerAuth || DirectAuth
// @Success      200  {object}  httpHelper.Response{data=[]model.Target}  "List of targets"
// @Failure      401  {object}  httpHelper.Response                       "Unauthorized"
// @Failure      403  {object}  httpHelper.Response                       "Forbidden (not an admin)"
// @Router       /ratelimit/targets [get]
func (h *Handler) ListTargets(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logging.GetLoggerFromContext(ctx)

	targets, err := h.service.ListTargets(ctx)
	if err != nil {
		log.Error().Err(err).Msg("failed to list rate limit targets")
		httpHelper.SendError(w, "failed to list rate limit targets", http.StatusInternalServerError)
		return
	}

	httpHelper.SendJSON(w, httpHelper.Response{Success: true, Data: targets}, http.StatusOK)
}

// UpdateTarget godoc
// @Summary      Update a rate limit target's rule
// @Description  Partially update a target's enabled/limit/window (admin-only). Omitted fields are left unchanged (pointer semantics).
// @Tags         rate-limiting
// @Accept       json
// @Produce      json
// @Param        key    path      string             true  "Target key"
// @Param        patch  body      model.TargetPatch  true  "Fields to update"
// @Security     BearerAuth || DirectAuth
// @Success      200  {object}  httpHelper.Response{data=model.Target}  "Updated target"
// @Failure      400  {object}  httpHelper.Response                     "Invalid request"
// @Failure      401  {object}  httpHelper.Response                     "Unauthorized"
// @Failure      403  {object}  httpHelper.Response                     "Forbidden (not an admin)"
// @Failure      404  {object}  httpHelper.Response                     "Unknown target"
// @Router       /ratelimit/targets/{key} [patch]
func (h *Handler) UpdateTarget(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logging.GetLoggerFromContext(ctx)
	key := mux.Vars(r)["key"]

	var patch model.TargetPatch
	if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
		httpHelper.SendError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if patch.LimitCount != nil && *patch.LimitCount < 1 {
		httpHelper.SendError(w, "limit_count must be at least 1", http.StatusBadRequest)
		return
	}

	claims, _ := auth.GetUserFromContext(ctx)

	target, err := h.service.UpdateTarget(ctx, key, patch, claims.Username)
	if err != nil {
		log.Error().Err(err).Str("target", key).Msg("failed to update rate limit target")
		handleServiceError(w, err)
		return
	}

	h.metrics.configChanged.Add(ctx, 1, configChangedAttr(key, "update"))
	httpHelper.SendJSON(w, httpHelper.Response{Success: true, Data: target}, http.StatusOK)
}

// ResetCounters godoc
// @Summary      Reset a target's counters
// @Description  Clears today's daily-quota counters for a target (admin-only). Does not clear an in-memory throttle bucket (per-process state).
// @Tags         rate-limiting
// @Produce      json
// @Param        key  path      string  true  "Target key"
// @Security     BearerAuth || DirectAuth
// @Success      200  {object}  httpHelper.Response  "Reset acknowledged"
// @Failure      401  {object}  httpHelper.Response  "Unauthorized"
// @Failure      403  {object}  httpHelper.Response  "Forbidden (not an admin)"
// @Failure      404  {object}  httpHelper.Response  "Unknown target"
// @Router       /ratelimit/targets/{key}/reset [post]
func (h *Handler) ResetCounters(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logging.GetLoggerFromContext(ctx)
	key := mux.Vars(r)["key"]

	if err := h.service.ResetCounters(ctx, key); err != nil {
		log.Error().Err(err).Str("target", key).Msg("failed to reset rate limit counters")
		handleServiceError(w, err)
		return
	}

	h.metrics.configChanged.Add(ctx, 1, configChangedAttr(key, "reset"))
	httpHelper.SendJSON(w, httpHelper.Response{Success: true}, http.StatusOK)
}

// ResetDefaults godoc
// @Summary      Reset a target's rule to its code-defined defaults
// @Description  Overwrites a target's enabled/limit/window back to its Registry defaults, clearing any prior admin edit (admin-only)
// @Tags         rate-limiting
// @Produce      json
// @Param        key  path      string  true  "Target key"
// @Security     BearerAuth || DirectAuth
// @Success      200  {object}  httpHelper.Response{data=model.Target}  "Target reset to defaults"
// @Failure      401  {object}  httpHelper.Response                     "Unauthorized"
// @Failure      403  {object}  httpHelper.Response                     "Forbidden (not an admin)"
// @Failure      404  {object}  httpHelper.Response                     "Unknown target"
// @Router       /ratelimit/targets/{key}/reset-defaults [post]
func (h *Handler) ResetDefaults(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logging.GetLoggerFromContext(ctx)
	key := mux.Vars(r)["key"]

	target, err := h.service.ResetDefaults(ctx, key)
	if err != nil {
		log.Error().Err(err).Str("target", key).Msg("failed to reset rate limit target to defaults")
		handleServiceError(w, err)
		return
	}

	h.metrics.configChanged.Add(ctx, 1, configChangedAttr(key, "reset-defaults"))
	httpHelper.SendJSON(w, httpHelper.Response{Success: true, Data: target}, http.StatusOK)
}
