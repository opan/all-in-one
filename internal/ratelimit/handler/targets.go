package handler

import (
	"net/http"

	httpHelper "github.com/all-in-one/internal/http"
	"github.com/all-in-one/internal/logging"
)

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
