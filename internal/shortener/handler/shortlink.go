package handler

import (
	"net/http"

	httpHelper "github.com/all-in-one/internal/http"
)

func (h *Handler) CreateShortLink(w http.ResponseWriter, r *http.Request) {
	httpHelper.SendError(w, "not implemented", http.StatusNotImplemented)
}

func (h *Handler) ListShortLinks(w http.ResponseWriter, r *http.Request) {
	httpHelper.SendError(w, "not implemented", http.StatusNotImplemented)
}

func (h *Handler) GetShortLink(w http.ResponseWriter, r *http.Request) {
	httpHelper.SendError(w, "not implemented", http.StatusNotImplemented)
}

func (h *Handler) UpdateShortLink(w http.ResponseWriter, r *http.Request) {
	httpHelper.SendError(w, "not implemented", http.StatusNotImplemented)
}

func (h *Handler) DeleteShortLink(w http.ResponseWriter, r *http.Request) {
	httpHelper.SendError(w, "not implemented", http.StatusNotImplemented)
}

func (h *Handler) ResolveShortLink(w http.ResponseWriter, r *http.Request) {
	httpHelper.SendError(w, "not implemented", http.StatusNotImplemented)
}
