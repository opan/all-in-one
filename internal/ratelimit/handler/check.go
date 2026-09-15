package handler

import (
	"encoding/json"
	"net/http"
	"strings"

	httpHelper "github.com/all-in-one/internal/http"
	"github.com/all-in-one/internal/logging"
	"github.com/all-in-one/internal/ratelimit/middleware"
	"github.com/all-in-one/internal/ratelimit/model"
)

// Check godoc
// @Summary      Check a rate limit (external apps)
// @Description  Evaluate one external target against a caller-supplied bucket key. Authenticated by X-API-Key (an app token), not a user session. A rejection is reported as 200 with allowed=false (not 429) so the caller can tell "your user is limited" from "aio limited you". Fails open on an internal counter error.
// @Tags         rate-limiting
// @Accept       json
// @Produce      json
// @Param        X-API-Key  header  string             true  "App token"
// @Param        request    body    model.CheckRequest true  "Target and bucket key"
// @Success      200  {object}  httpHelper.Response{data=model.CheckResponse}  "Decision"
// @Failure      400  {object}  httpHelper.Response  "Invalid request or bucket key does not match scope"
// @Failure      401  {object}  httpHelper.Response  "Missing or invalid app token"
// @Failure      403  {object}  httpHelper.Response  "Target key outside token scope"
// @Failure      404  {object}  httpHelper.Response  "Unknown external target"
// @Failure      503  {object}  httpHelper.Response  "External rate limiting disabled"
// @Router       /ratelimit/check [post]
func (h *Handler) Check(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logging.GetLoggerFromContext(ctx)

	// 1. Kill switch: 503 so a caller can distinguish "disabled" from a silent
	//    allow.
	if !h.config.RateLimit.External.Enabled {
		httpHelper.SendError(w, "external rate limiting is disabled", http.StatusServiceUnavailable)
		return
	}

	// 2. The app token is placed in context by AppTokenAuth (P10/P13).
	tok, ok := middleware.AppTokenFromContext(ctx)
	if !ok {
		httpHelper.SendError(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req model.CheckRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpHelper.SendError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	req.TargetKey = strings.TrimSpace(req.TargetKey)
	req.BucketKey = strings.TrimSpace(req.BucketKey)
	if req.TargetKey == "" || req.BucketKey == "" {
		httpHelper.SendError(w, "target_key and bucket_key are required", http.StatusBadRequest)
		return
	}

	// Scope enforcement (locked decision #4): a token may only touch target
	// keys under its declared prefix, so a leaked token can't exhaust another
	// app's — or aio's own — quotas.
	if !strings.HasPrefix(req.TargetKey, tok.ScopePrefix) {
		httpHelper.SendError(w, "target_key is outside the token's scope", http.StatusForbidden)
		return
	}

	// 3. The target must exist and be external. An internal target is not
	//    callable through this API — allowing it would let a consumer burn
	//    aio's own quotas.
	rule, ok := h.service.EffectiveRule(req.TargetKey)
	if !ok || !rule.IsExternal {
		httpHelper.SendError(w, "unknown external target", http.StatusNotFound)
		return
	}

	// 4. Bucket-key shape must match the declared scope. ADVISORY ONLY: aio
	//    cannot verify the caller's bucket key really came from a session, so
	//    this catches an obviously-wrong caller, it is not a security control
	//    (locked decision #6).
	if !bucketKeyMatchesScope(req.BucketKey, rule.Scope) {
		httpHelper.SendError(w, "bucket_key does not match the target's scope", http.StatusBadRequest)
		return
	}

	// 5. Decide. A counter-store error fails open (ADR-007): CheckExternal
	//    returns allowed=true and a non-nil error we meter and log.
	resp, err := h.service.CheckExternal(ctx, req.TargetKey, req.BucketKey)
	if err != nil {
		log.Error().Err(err).Str("target", req.TargetKey).
			Msg("ratelimit: external check store error, failing open")
	}

	h.metrics.checkCalls.Add(ctx, 1, checkAttr(tok.App, req.TargetKey, resp.Allowed))
	// A rejection is 200 + allowed:false, never 429 — see the godoc note.
	httpHelper.SendJSON(w, httpHelper.Response{Success: true, Data: resp}, http.StatusOK)
}

// bucketKeyMatchesScope reports whether a caller-supplied bucket key has the
// prefix expected for a scope (user:/ip:/global). Advisory — see Check step 4.
func bucketKeyMatchesScope(bucketKey string, scope model.Scope) bool {
	switch scope {
	case model.ScopeUser:
		return strings.HasPrefix(bucketKey, "user:")
	case model.ScopeIP:
		return strings.HasPrefix(bucketKey, "ip:")
	case model.ScopeGlobal:
		return bucketKey == "global"
	}
	return false
}
