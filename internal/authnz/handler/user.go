package handler

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"time"

	"github.com/all-in-one/internal/auth"
	"github.com/all-in-one/internal/authnz/model"
	httpHelper "github.com/all-in-one/internal/http"
	"github.com/all-in-one/internal/logging"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

type RegisterUserRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Email    string `json:"email"`
	Name     string `json:"name"`
}

// GroupRef is a minimal {id,name} reference to a group, embedded in
// CurrentUserResponse.
type GroupRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// CurrentUserResponse widens model.User with the RBAC info the frontend
// needs to filter its menu and gate the Access Management section: whether
// the user is an admin, their effective group, and the non-admin-only
// feature keys they can currently access.
type CurrentUserResponse struct {
	model.User
	IsAdmin  bool      `json:"is_admin"`
	Group    *GroupRef `json:"group"`
	Features []string  `json:"features"`
}

// GetCurrentUser godoc
// @Summary      Get current authenticated user
// @Description  Retrieve the profile of the currently authenticated user, plus their effective RBAC access (is_admin, group, features)
// @Tags         users
// @Produce      json
// @Security     BearerAuth || DirectAuth
// @Success      200  {object}  httpHelper.Response{data=CurrentUserResponse}  "User profile with RBAC info"
// @Failure      401  {object}  httpHelper.Response                            "Unauthorized"
// @Failure      500  {object}  httpHelper.Response                            "Internal server error"
// @Router       /users/me [get]
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
		// The x-direct-auth-username dev bypass fabricates a non-UUID
		// UserID with no corresponding DB row (see jwt.go tryDirectAuth) —
		// that's not a malformed-claims error, it's expected for that path.
		if cu.SessionID == "direct-auth" {
			httpHelper.SendJSON(w, httpHelper.Response{
				Success: true,
				Data:    h.directAuthCurrentUser(cu.Username, cu.Email),
			}, http.StatusOK)
			return
		}
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

	response := CurrentUserResponse{User: user, Features: []string{}}

	if h.accessResolver != nil {
		isAdmin, groupID, groupName, featureKeys, err := h.accessResolver.EffectiveFeatures(ctx, uid)
		if err != nil {
			log.Error().Err(err).Str("user_id", cu.UserID).Msg("failed to resolve effective RBAC access")
			httpHelper.SendError(w, "Failed to resolve access", http.StatusInternalServerError)
			return
		}
		response.IsAdmin = isAdmin
		response.Group = &GroupRef{ID: groupID.String(), Name: groupName}
		response.Features = featureKeys
	}

	httpHelper.SendJSON(w, httpHelper.Response{Success: true, Data: response}, http.StatusOK)
}

// directAuthCurrentUser builds a minimal CurrentUserResponse for the
// x-direct-auth-username dev bypass, which has no backing DB row. Admin
// status is config-driven (rbac.direct_auth_is_admin) rather than resolved,
// matching the RequireAdmin/RequireFeature middleware's treatment of the
// same bypass (see internal/rbac/middleware/authz.go). Features is left
// empty — real enforcement never trusts this list (RequireAdmin/
// RequireFeature re-check independently on every request), so there is no
// resolvable identity to compute it against and no need to.
func (h *Handler) directAuthCurrentUser(username, email string) CurrentUserResponse {
	return CurrentUserResponse{
		User:     model.User{Username: username, Email: email},
		IsAdmin:  h.config.RBAC.DirectAuthIsAdmin,
		Features: []string{},
	}
}

// RegisterUser godoc
// @Summary      Register a new user
// @Description  Create a new user account with username, password, email, and name
// @Tags         users
// @Accept       json
// @Produce      json
// @Param        user  body      RegisterUserRequest  true  "User registration details"
// @Success      201   {object}  httpHelper.Response  "User created successfully"
// @Failure      400   {object}  httpHelper.Response  "Invalid request body or missing required fields"
// @Failure      409   {object}  httpHelper.Response  "User already exists"
// @Failure      500   {object}  httpHelper.Response  "Internal server error"
// @Router       /users/register [post]
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
		h.metrics.registrationsTotal.Add(ctx, 1, metric.WithAttributes(attribute.String("result", "failure")))
		httpHelper.SendError(w, "user already exists", http.StatusConflict)
		return
	}

	hashedPassword, err := auth.HashPassword(req.Password)
	if err != nil {
		log.Error().Err(err).Msg("failed to hash password")
		h.metrics.registrationsTotal.Add(ctx, 1, metric.WithAttributes(attribute.String("result", "failure")))
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
		h.metrics.registrationsTotal.Add(ctx, 1, metric.WithAttributes(attribute.String("result", "failure")))
		httpHelper.SendError(w, "failed to create user", http.StatusInternalServerError)
		return
	}

	h.metrics.registrationsTotal.Add(ctx, 1, metric.WithAttributes(attribute.String("result", "success")))
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

// ResetPasswordUser godoc
// @Summary      Reset user password
// @Description  Reset the password for the currently authenticated user
// @Tags         users
// @Accept       json
// @Produce      json
// @Security     BearerAuth || DirectAuth
// @Param        password  body      model.UserPasswordReset  true  "Current and new password"
// @Success      200       {object}  httpHelper.Response      "Password reset successfully"
// @Failure      400       {object}  httpHelper.Response      "Invalid request body"
// @Failure      401       {object}  httpHelper.Response      "Unauthorized or invalid current password"
// @Failure      500       {object}  httpHelper.Response      "Internal server error"
// @Router       /users/me/password [put]
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

	// Check current password
	ok, err = auth.CheckPassword(req.CurrentPassword, u.PasswordHash)
	if err != nil {
		log.Error().Err(err).Msg("failed to check current password")
		httpHelper.SendError(w, "failed to check current password", http.StatusInternalServerError)
		return
	}

	if !ok {
		log.Warn().Msg("invalid current password")
		httpHelper.SendError(w, "invalid password", http.StatusUnauthorized)
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

	sid, err := uuid.Parse(cu.SessionID)
	if err != nil {
		log.Error().Err(err).Msg("invalid session ID in context")
		httpHelper.SendError(w, "Failed to invalidate sessions", http.StatusInternalServerError)
		return
	}

	if err := h.storage.SessionRepo().DeleteByUserIDExcept(ctx, uid, sid); err != nil {
		log.Error().Err(err).Str("user_id", cu.UserID).Msg("failed to invalidate other sessions after password reset")
		httpHelper.SendError(w, "Failed to invalidate sessions", http.StatusInternalServerError)
		return
	}

	res := httpHelper.Response{
		Success: true,
		Message: "Password has been resetted successfully",
	}

	httpHelper.SendJSON(w, res, http.StatusOK)
}
