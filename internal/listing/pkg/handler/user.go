package handler

import (
	"database/sql"
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

func (h *Handler) GetCurrentUser(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logging.GetLoggerFromContext(ctx)

	cu, ok := auth.GetUserFromContext(ctx)
	if !ok {
		log.Error().Msg("failed to get user from context")
		httpHelper.SendError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	uid, err := uuid.Parse(cu.UserID)
	if err != nil {
		log.Error().Err(err).Str("user_id", cu.UserID).Msg("invalid user ID in context")
		httpHelper.SendError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	user, err := h.storage.UserRepo().Find(ctx, uid)
	if err != nil {
		log.Error().Err(err).Str("user_id", cu.UserID).Msg("failed to get user from db")
		httpHelper.SendError(w, "Failed to retrieve user", http.StatusInternalServerError)
		return
	}

	response := httpHelper.Response{
		Success: true,
		Data:    user,
	}

	httpHelper.SendJSON(w, response, http.StatusOK)
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
	if err != nil && err != sql.ErrNoRows {
		log.Error().Err(err).Msg("failed to check existing user")
		httpHelper.SendError(w, "failed to register user", http.StatusInternalServerError)
		return
	}

	if existingUser.ID != uuid.Nil {
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

func (h *Handler) ResetPasswordUser(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logging.GetLoggerFromContext(ctx)

	cu, ok := auth.GetUserFromContext(ctx)
	if !ok {
		log.Error().Msg("failed to get user from context")
		httpHelper.SendError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	uid, err := uuid.Parse(cu.UserID)
	if err != nil {
		log.Error().Err(err).Str("user_id", cu.UserID).Msg("invalid user ID in context")
		httpHelper.SendError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req model.UserPasswordReset
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Error().Err(err).Msg("failed to decode request body")
		httpHelper.SendError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	u, err := h.storage.UserRepo().Find(ctx, uid)
	if err != nil {
		log.Error().Err(err).Str("user_id", cu.UserID).Msg("failed to get user from db")
		httpHelper.SendError(w, "Failed to retrieve user", http.StatusInternalServerError)
		return
	}

	hashedPassword, err := auth.HashPassword(req.Password)
	if err != nil {
		log.Error().Err(err).Msg("failed to hash password")
		httpHelper.SendError(w, "failed to reset password", http.StatusInternalServerError)
		return
	}

	u.PasswordHash = hashedPassword

	if err := h.storage.UserRepo().Update(ctx, uid, u); err != nil {
		log.Error().Err(err).Str("user_id", cu.UserID).Msg("failed to update user password in db")
		httpHelper.SendError(w, "Failed to update password", http.StatusInternalServerError)
		return
	}

	res := httpHelper.Response{
		Success: true,
		Message: "Password has been resetted successfully",
	}

	httpHelper.SendJSON(w, res, http.StatusOK)
}
