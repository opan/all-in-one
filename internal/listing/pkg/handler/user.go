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
		res := httpHelper.Response{
			Success: false,
			Error:   "invalid request body",
		}
		httpHelper.SendJSON(w, res, http.StatusBadRequest)
		return
	}

	if req.Username == "" || req.Password == "" {
		log.Error().Msg("username or password is empty")
		res := httpHelper.Response{
			Success: false,
			Error:   "username and password are required",
		}
		httpHelper.SendJSON(w, res, http.StatusBadRequest)
		return
	}

	existingUser, err := h.storage.UserRepo().FindByUsername(ctx, req.Username)
	if err == nil && existingUser != nil {
		log.Error().Str("username", req.Username).Msg("user already exists")
		res := httpHelper.Response{
			Success: false,
			Error:   "user already exists",
		}
		httpHelper.SendJSON(w, res, http.StatusConflict)
		return
	}

	hashedPassword, err := auth.HashPassword(req.Password)
	if err != nil {
		log.Error().Err(err).Msg("failed to hash password")
		res := httpHelper.Response{
			Success: false,
			Error:   "failed to create user",
		}
		httpHelper.SendJSON(w, res, http.StatusInternalServerError)
		return
	}

	user := model.User{
		ID:           uuid.New().String(),
		Username:     req.Username,
		Email:        req.Email,
		Name:         req.Name,
		PasswordHash: hashedPassword,
		LastLogin:    time.Now(),
	}

	if err := h.storage.UserRepo().Create(ctx, user); err != nil {
		log.Error().Err(err).Msg("failed to create user")
		res := httpHelper.Response{
			Success: false,
			Error:   "failed to create user",
		}
		httpHelper.SendJSON(w, res, http.StatusInternalServerError)
		return
	}

	log.Info().Str("username", req.Username).Msg("user successfully registered")
	res := httpHelper.Response{
		Success: true,
		Message: "user created successfully",
		Data: map[string]string{
			"id":       user.ID,
			"username": user.Username,
		},
	}

	httpHelper.SendJSON(w, res, http.StatusCreated)
}
