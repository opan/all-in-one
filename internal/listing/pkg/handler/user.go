package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/all-in-one/internal/auth"
	httpHelper "github.com/all-in-one/internal/http"
	"github.com/all-in-one/internal/listing/pkg/model"
	"github.com/all-in-one/internal/logging"
	"github.com/google/uuid"
)

type RegisterUserRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Email    string `json:"email"`
	Name     string `json:"name"`
}

func (h *Handler) RegisterUser(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logging.GetLoggerFromContext(ctx)

	var req RegisterUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Error().Err(err).Msg("failed to decode request body")
		httpHelper.SendError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Username == "" || req.Password == "" {
		log.Error().Msg("username or password is empty")
		httpHelper.SendError(w, "username and password are required", http.StatusBadRequest)
		return
	}

	existingUser, err := h.storage.UserRepo().FindByUsername(ctx, req.Username)
	if err == nil && existingUser != nil {
		log.Error().Str("username", req.Username).Msg("user already exists")
		httpHelper.SendError(w, "user already exists", http.StatusConflict)
		return
	}

	hashedPassword, err := auth.HashPassword(req.Password)
	if err != nil {
		log.Error().Err(err).Msg("failed to hash password")
		httpHelper.SendError(w, "failed to register user", http.StatusInternalServerError)
		return
	}

	now := time.Now().UTC()
	user := model.User{
		ID:           uuid.New(),
		Username:     req.Username,
		Email:        req.Email,
		Name:         req.Name,
		PasswordHash: hashedPassword,
		LastLogin:    &now,
	}

	if err := h.storage.UserRepo().Create(ctx, user); err != nil {
		log.Error().Err(err).Msg("failed to create user")
		httpHelper.SendError(w, "failed to create user", http.StatusInternalServerError)
		return
	}

	log.Info().Str("username", req.Username).Msg("user successfully registered")
	res := httpHelper.Response{
		Success: true,
		Message: "user created successfully",
		Data: map[string]string{
			"id":       user.ID.String(),
			"username": user.Username,
		},
	}

	httpHelper.SendJSON(w, res, http.StatusCreated)
}
