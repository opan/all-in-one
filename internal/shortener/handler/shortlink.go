package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/all-in-one/internal/auth"
	httpHelper "github.com/all-in-one/internal/http"
	"github.com/all-in-one/internal/logging"
	"github.com/all-in-one/internal/shortener/model"
	"github.com/gorilla/mux"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

type createShortLinkRequest struct {
	TargetURL  string  `json:"target_url"`
	ExpiresAt  *string `json:"expires_at,omitempty"`
	CustomCode *string `json:"custom_code,omitempty"`
}

type updateShortLinkRequest struct {
	IsActive  *bool   `json:"is_active,omitempty"`
	ExpiresAt *string `json:"expires_at,omitempty"`
}

type shortLinkListResponse struct {
	Links []model.ShortLink `json:"links"`
	Total uint32            `json:"total"`
	Page  uint32            `json:"page"`
}

func (h *Handler) CreateShortLink(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logging.GetLoggerFromContext(ctx)

	user, ok := auth.GetUserFromContext(ctx)
	if !ok {
		httpHelper.SendUnauthorized(w, "unauthorized")
		return
	}

	createResult := "failure"
	defer func() {
		h.metrics.linksCreated.Add(ctx, 1, metric.WithAttributes(attribute.String("result", createResult)))
	}()

	var req createShortLinkRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpHelper.SendError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.CustomCode != nil && *req.CustomCode != "" {
		httpHelper.SendError(w, "custom codes are not yet available", http.StatusBadRequest)
		return
	}

	urlCfg := h.config.Shortener.URL
	if urlCfg.MaxLength == 0 {
		urlCfg.MaxLength = 2048
	}
	if len(urlCfg.AllowedSchemes) == 0 {
		urlCfg.AllowedSchemes = []string{"http", "https"}
	}
	if err := validateURL(req.TargetURL, urlCfg); err != nil {
		httpHelper.SendError(w, err.Error(), http.StatusBadRequest)
		return
	}

	var expiresAt *time.Time
	if req.ExpiresAt != nil {
		t, err := time.Parse(time.RFC3339, *req.ExpiresAt)
		if err != nil {
			httpHelper.SendError(w, "expires_at must be RFC3339 format", http.StatusBadRequest)
			return
		}
		if t.Before(time.Now()) {
			httpHelper.SendError(w, "expires_at must be in the future", http.StatusBadRequest)
			return
		}
		expiresAt = &t
	}

	maxRetries := h.config.Shortener.MaxCreateRetries
	if maxRetries == 0 {
		maxRetries = 5
	}
	codeLength := h.config.Shortener.CodeLength
	if codeLength == 0 {
		codeLength = 7
	}

	link := model.ShortLink{
		ID:        newULID(),
		TargetURL: req.TargetURL,
		CreatedAt: time.Now().UTC(),
		ExpiresAt: expiresAt,
		IsActive:  true,
	}

	var created model.ShortLink
	for i := 0; i < maxRetries; i++ {
		code, err := newShortCode(codeLength)
		if err != nil {
			log.Error().Err(err).Msg("failed to generate short code")
			httpHelper.SendError(w, "failed to generate code", http.StatusInternalServerError)
			return
		}
		if isReserved(code) {
			continue
		}
		link.Code = code

		created, err = h.storage.ShortLinkRepo().Create(ctx, link, user.UserID)
		if err == nil {
			break
		}
		if isUniqueConstraintError(err) {
			if i == maxRetries-1 {
				httpHelper.SendError(w, "failed to generate unique code, please retry", http.StatusInternalServerError)
				return
			}
			continue
		}
		log.Error().Err(err).Msg("failed to create short link")
		httpHelper.SendError(w, "failed to create short link", http.StatusInternalServerError)
		return
	}

	createResult = "success"
	httpHelper.SendJSON(w, httpHelper.Response{
		Success: true,
		Message: "Short link created",
		Data:    created,
	}, http.StatusCreated)
}

func (h *Handler) ListShortLinks(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logging.GetLoggerFromContext(ctx)

	user, ok := auth.GetUserFromContext(ctx)
	if !ok {
		httpHelper.SendUnauthorized(w, "unauthorized")
		return
	}

	page := parseUint32(r.URL.Query().Get("page"), 1)
	pageSize := parseUint32(r.URL.Query().Get("page_size"), 20)
	if pageSize > 100 {
		pageSize = 100
	}

	links, total, err := h.storage.ShortLinkRepo().ListByOwner(ctx, user.UserID, page, pageSize)
	if err != nil {
		log.Error().Err(err).Msg("failed to list short links")
		httpHelper.SendError(w, "failed to list short links", http.StatusInternalServerError)
		return
	}

	httpHelper.SendJSON(w, httpHelper.Response{
		Success: true,
		Data: shortLinkListResponse{
			Links: links,
			Total: total,
			Page:  page,
		},
	}, http.StatusOK)
}

func (h *Handler) GetShortLink(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	user, ok := auth.GetUserFromContext(ctx)
	if !ok {
		httpHelper.SendUnauthorized(w, "unauthorized")
		return
	}

	code := mux.Vars(r)["code"]
	link, err := h.storage.ShortLinkRepo().GetByCodeOwned(ctx, code, user.UserID)
	if err != nil {
		httpHelper.SendError(w, "not found", http.StatusNotFound)
		return
	}

	httpHelper.SendJSON(w, httpHelper.Response{Success: true, Data: link}, http.StatusOK)
}

func (h *Handler) UpdateShortLink(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logging.GetLoggerFromContext(ctx)

	user, ok := auth.GetUserFromContext(ctx)
	if !ok {
		httpHelper.SendUnauthorized(w, "unauthorized")
		return
	}

	code := mux.Vars(r)["code"]
	link, err := h.storage.ShortLinkRepo().GetByCodeOwned(ctx, code, user.UserID)
	if err != nil {
		httpHelper.SendError(w, "not found", http.StatusNotFound)
		return
	}

	var req updateShortLinkRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpHelper.SendError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.IsActive != nil {
		link.IsActive = *req.IsActive
	}
	if req.ExpiresAt != nil {
		if *req.ExpiresAt == "" {
			link.ExpiresAt = nil
		} else {
			t, err := time.Parse(time.RFC3339, *req.ExpiresAt)
			if err != nil {
				httpHelper.SendError(w, "expires_at must be RFC3339 format or empty string to clear", http.StatusBadRequest)
				return
			}
			link.ExpiresAt = &t
		}
	}

	updated, err := h.storage.ShortLinkRepo().Update(ctx, link, user.UserID)
	if err != nil {
		log.Error().Err(err).Str("code", code).Msg("failed to update short link")
		httpHelper.SendError(w, "failed to update short link", http.StatusInternalServerError)
		return
	}

	httpHelper.SendJSON(w, httpHelper.Response{Success: true, Data: updated}, http.StatusOK)
}

func (h *Handler) DeleteShortLink(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logging.GetLoggerFromContext(ctx)

	user, ok := auth.GetUserFromContext(ctx)
	if !ok {
		httpHelper.SendUnauthorized(w, "unauthorized")
		return
	}

	code := mux.Vars(r)["code"]
	err := h.storage.ShortLinkRepo().Delete(ctx, code, user.UserID)
	if err == httpHelper.ErrNotFound {
		httpHelper.SendError(w, "not found", http.StatusNotFound)
		return
	}
	if err != nil {
		log.Error().Err(err).Str("code", code).Msg("failed to delete short link")
		httpHelper.SendError(w, "failed to delete short link", http.StatusInternalServerError)
		return
	}

	h.metrics.linksDeleted.Add(ctx, 1)
	httpHelper.SendJSON(w, httpHelper.Response{Success: true, Message: "Short link deleted"}, http.StatusOK)
}

func (h *Handler) ResolveShortLink(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logging.GetLoggerFromContext(ctx)

	code := mux.Vars(r)["code"]
	now := time.Now().UTC()

	resolveResult := "not_found"
	defer func() {
		h.metrics.linksResolved.Add(ctx, 1, metric.WithAttributes(attribute.String("result", resolveResult)))
	}()

	link, err := h.storage.ShortLinkRepo().GetByCode(ctx, code)
	if err != nil {
		httpHelper.SendError(w, "not found", http.StatusNotFound)
		return
	}
	if !link.IsActive {
		resolveResult = "disabled"
		httpHelper.SendError(w, "not found", http.StatusNotFound)
		return
	}
	if link.ExpiresAt != nil && link.ExpiresAt.Before(now) {
		resolveResult = "expired"
		httpHelper.SendError(w, "link has expired", http.StatusGone)
		return
	}

	if err := h.storage.ShortLinkRepo().IncrementClick(ctx, code, now.Format(time.RFC3339)); err != nil {
		log.Warn().Err(err).Str("code", code).Msg("failed to increment click count")
	}

	resolveResult = "success"
	w.Header().Set("Cache-Control", "private, no-store")
	http.Redirect(w, r, link.TargetURL, http.StatusFound)
}

func parseUint32(s string, defaultVal uint32) uint32 {
	if s == "" {
		return defaultVal
	}
	n, err := strconv.ParseUint(s, 10, 32)
	if err != nil {
		return defaultVal
	}
	return uint32(n)
}
