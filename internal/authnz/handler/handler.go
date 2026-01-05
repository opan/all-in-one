package handler

import (
	"github.com/all-in-one/internal/authnz/repository"
	"github.com/all-in-one/internal/config"
	"github.com/gorilla/mux"
)

type Handler struct {
	storage repository.Storage
	config  config.Config
}

func NewHandler(storage repository.Storage, config config.Config) *Handler {
	return &Handler{
		storage: storage,
		config:  config,
	}
}

func (h *Handler) RegisterPublicRoutes(router *mux.Router) {
	router.HandleFunc("/users", h.RegisterUser).Methods("POST")
	router.HandleFunc("/sessions", h.CreateSession).Methods("POST")
	router.HandleFunc("/sessions/refresh", h.RefreshToken).Methods("POST")
	router.HandleFunc("/sessions/verify", h.VerifySession).Methods("GET")
}

func (h *Handler) RegisterAuthenticatedRoutes(router *mux.Router) {
	router.HandleFunc("/users/me", h.GetCurrentUser).Methods("GET")
	router.HandleFunc("/users/reset_password", h.ResetPasswordUser).Methods("POST")

	router.HandleFunc("/sessions", h.DeleteSession).Methods("DELETE")
}
