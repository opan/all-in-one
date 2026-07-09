package handler

import (
	"encoding/json"
	"net/http"

	httpHelper "github.com/all-in-one/internal/http"
	"github.com/all-in-one/internal/logging"
	"github.com/all-in-one/internal/shortener/model"
	"github.com/gorilla/mux"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// adminModerateShortLinkRequest is the body for PATCH /admin/shortener/links/{code}.
type adminModerateShortLinkRequest struct {
	IsActive bool `json:"is_active"`
}

type adminShortLinkListResponse struct {
	Links []model.ShortLinkWithOwner `json:"links"`
	Total uint32                     `json:"total"`
	Page  uint32                     `json:"page"`
}

// RegisterAdminRoutes wires the admin-only shortener moderation endpoints. Mounted
// on the RequireAdmin subrouter in cmd/all-in-one/server/server.go, so every route
// here is already gated to admins by the time it runs.
func (h *Handler) RegisterAdminRoutes(router *mux.Router) {
	router.HandleFunc("/admin/shortener/links", h.ListAllShortLinks).Methods(http.MethodGet)
	router.HandleFunc("/admin/shortener/links/{code}", h.AdminModerateShortLink).Methods(http.MethodPatch)
	router.HandleFunc("/admin/shortener/links/{code}", h.AdminDeleteShortLink).Methods(http.MethodDelete)
}

// ListAllShortLinks godoc
// @Summary      List all short links (admin)
// @Description  Admin-only. List short links across all owners, paginated.
// @Tags         admin-shortener
// @Produce      json
// @Security     BearerAuth || DirectAuth
// @Param        page       query     int                  false  "Page number (default 1)"
// @Param        page_size  query     int                  false  "Page size (default 20, max 100)"
// @Success      200        {object}  httpHelper.Response  "Paginated list of short links"
// @Router       /admin/shortener/links [get]
func (h *Handler) ListAllShortLinks(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logging.GetLoggerFromContext(ctx)

	page := parseUint32(r.URL.Query().Get("page"), 1)
	pageSize := parseUint32(r.URL.Query().Get("page_size"), 20)
	if pageSize > 100 {
		pageSize = 100
	}

	links, total, err := h.storage.ShortLinkRepo().ListAll(ctx, page, pageSize)
	if err != nil {
		log.Error().Err(err).Msg("failed to list all short links")
		httpHelper.SendError(w, "failed to list short links", http.StatusInternalServerError)
		return
	}

	httpHelper.SendJSON(w, httpHelper.Response{
		Success: true,
		Data: adminShortLinkListResponse{
			Links: links,
			Total: total,
			Page:  page,
		},
	}, http.StatusOK)
}

// AdminModerateShortLink godoc
// @Summary      Activate or deactivate a short link (admin)
// @Description  Admin-only. Set a short link's active state regardless of owner.
// @Tags         admin-shortener
// @Accept       json
// @Produce      json
// @Security     BearerAuth || DirectAuth
// @Param        code  path      string                         true  "Short link code"
// @Param        body  body      adminModerateShortLinkRequest  true  "Desired active state"
// @Success      200   {object}  httpHelper.Response            "Link updated"
// @Failure      400   {object}  httpHelper.Response            "Invalid request body"
// @Failure      404   {object}  httpHelper.Response            "Link not found"
// @Router       /admin/shortener/links/{code} [patch]
func (h *Handler) AdminModerateShortLink(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logging.GetLoggerFromContext(ctx)

	code := mux.Vars(r)["code"]

	var req adminModerateShortLinkRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpHelper.SendError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	if err := h.storage.ShortLinkRepo().SetActiveByCode(ctx, code, req.IsActive); err != nil {
		if err == httpHelper.ErrNotFound {
			httpHelper.SendError(w, "not found", http.StatusNotFound)
			return
		}
		log.Error().Err(err).Str("code", code).Msg("failed to moderate short link")
		httpHelper.SendError(w, "failed to update short link", http.StatusInternalServerError)
		return
	}

	action := "deactivate"
	if req.IsActive {
		action = "activate"
	}
	h.metrics.adminLinkModerated.Add(ctx, 1, metric.WithAttributes(attribute.String("action", action)))
	log.Info().Str("code", code).Str("action", action).Msg("admin moderated short link")
	httpHelper.SendJSON(w, httpHelper.Response{Success: true, Message: "short link updated"}, http.StatusOK)
}

// AdminDeleteShortLink godoc
// @Summary      Delete a short link (admin)
// @Description  Admin-only. Delete a short link regardless of owner.
// @Tags         admin-shortener
// @Produce      json
// @Security     BearerAuth || DirectAuth
// @Param        code  path      string               true  "Short link code"
// @Success      200   {object}  httpHelper.Response  "Link deleted"
// @Failure      404   {object}  httpHelper.Response  "Link not found"
// @Router       /admin/shortener/links/{code} [delete]
func (h *Handler) AdminDeleteShortLink(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logging.GetLoggerFromContext(ctx)

	code := mux.Vars(r)["code"]

	if err := h.storage.ShortLinkRepo().DeleteByCode(ctx, code); err != nil {
		if err == httpHelper.ErrNotFound {
			httpHelper.SendError(w, "not found", http.StatusNotFound)
			return
		}
		log.Error().Err(err).Str("code", code).Msg("failed to delete short link")
		httpHelper.SendError(w, "failed to delete short link", http.StatusInternalServerError)
		return
	}

	h.metrics.adminLinkDeleted.Add(ctx, 1)
	log.Info().Str("code", code).Msg("admin deleted short link")
	httpHelper.SendJSON(w, httpHelper.Response{Success: true, Message: "short link deleted"}, http.StatusOK)
}
