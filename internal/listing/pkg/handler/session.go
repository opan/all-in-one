package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/all-in-one/internal/auth"
	httpHelper "github.com/all-in-one/internal/http"
	"github.com/all-in-one/internal/listing/pkg/model"
	"github.com/all-in-one/internal/logging"
	jwt "github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// CreateSession godoc
// @Summary      Create a new session (login)
// @Description  Authenticate user and create a new session with access and refresh tokens
// @Tags         sessions
// @Accept       json
// @Produce      json
// @Param        credentials  body      model.LoginRequest                  true  "Login credentials"
// @Success      201          {object}  httpHelper.Response{data=model.User}  "Session created successfully"
// @Failure      400          {object}  httpHelper.Response                   "Invalid request payload"
// @Failure      404          {object}  httpHelper.Response                   "Username or password not found"
// @Failure      500          {object}  httpHelper.Response                   "Internal server error"
// @Router       /sessions [post]
func (h *Handler) CreateSession(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logging.GetLoggerFromContext(ctx)

	var rl model.LoginRequest

	if err := json.NewDecoder(r.Body).Decode(&rl); err != nil {
		httpHelper.SendError(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	defer r.Body.Close()

	u, err := h.storage.UserRepo().FindByUsername(ctx, rl.Username)
	if err != nil {
		httpHelper.SendError(w, "invalid username or password", http.StatusNotFound)
		return
	}

	ok, err := auth.CheckPassword(rl.Password, u.PasswordHash)
	if err != nil || !ok {
		httpHelper.SendError(w, "invalid username or password", http.StatusUnauthorized)
		return
	}

	sid, err := uuid.NewUUID()
	if err != nil {
		httpHelper.SendError(w, fmt.Sprintf("failed to generate session id: %v", err), http.StatusInternalServerError)
		return
	}

	s := model.Session{
		ID:        sid,
		UserID:    u.ID,
		CreatedAt: time.Now(),
		UserAgent: r.UserAgent(),
		// 30 minutes
		AccessTokenExpiry: int((time.Now().Add(30 * time.Minute)).Unix()),
		// 7 days
		RefreshTokenExpiry: int((time.Now().Add(14 * 24 * time.Hour)).Unix()),
	}

	trx, err := h.storage.TopicRepo().CreateTrx(ctx)
	if err != nil {
		httpHelper.SendError(w, fmt.Sprintf("failed to create transaction: %v", err), http.StatusInternalServerError)
		return
	}
	defer trx.Rollback()

	err = h.storage.SessionRepo().Create(ctx, s, trx)
	if err != nil {
		httpHelper.SendError(w, fmt.Sprintf("failed to create user session: %v", err), http.StatusInternalServerError)
		return
	}

	at, err := h.createAccessToken(sid, u)
	if err != nil {
		log.Error().Err(err).Msg("failed to create access token")
		httpHelper.SendError(w, fmt.Sprintf("failed to create access token: %v", err), http.StatusInternalServerError)
		return
	}

	rt, err := h.createRefreshToken(sid)
	if err != nil {
		log.Error().Err(err).Msg("failed to create refresh token")
		httpHelper.SendError(w, fmt.Sprintf("failed to create refresh token: %v", err), http.StatusInternalServerError)
		return
	}

	err = trx.Commit()
	if err != nil {
		httpHelper.SendError(w, fmt.Sprintf("failed to commit session to db: %v", err), http.StatusInternalServerError)
		return
	}

	// Set access token cookie (short expiry)
	http.SetCookie(w, &http.Cookie{
		Name:     "access_token",
		Value:    at,
		HttpOnly: true,
		Secure:   true,
		Path:     "/",
		MaxAge:   1800, // 30 minutes
		SameSite: http.SameSiteStrictMode,
	})

	// Set refresh token cookie (longer expiry)
	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    rt,
		HttpOnly: true,
		Secure:   true,
		Path:     "/api/v1/sessions/refresh", // Only sent to refresh endpoint
		MaxAge:   604800,                     // 7 days
		SameSite: http.SameSiteStrictMode,
	})

	log.Info().Str("session_id", sid.String()).Msg("session successfully created")

	response := httpHelper.Response{
		Success: true,
		Data:    u,
	}

	httpHelper.SendJSON(w, response, http.StatusCreated)
}

// DeleteSession godoc
// @Summary      Delete session (logout)
// @Description  Invalidate the current session and clear authentication cookies
// @Tags         sessions
// @Produce      json
// @Success      200  {object}  httpHelper.Response  "Session deleted successfully"
// @Failure      401  {object}  httpHelper.Response  "Unauthorized - invalid or missing token"
// @Failure      500  {object}  httpHelper.Response  "Internal server error"
// @Router       /sessions [delete]
// @Security     ApiKeyAuth
func (h *Handler) DeleteSession(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logging.GetLoggerFromContext(ctx)

	cookie, err := r.Cookie("access_token")
	if err != nil {
		log.Warn().Err(err).Msg("Access token missing in request")
		httpHelper.SendError(w, "Access token required", http.StatusUnauthorized)
		return
	}

	token, err := jwt.Parse(cookie.Value, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return []byte(h.config.Auth.JWTSecret), nil
	})

	if err != nil || !token.Valid {
		log.Warn().Err(err).Msg("Invalid access token")
		httpHelper.SendError(w, "Invalid access token", http.StatusUnauthorized)
		return
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		log.Error().Msg("Invalid token claims")
		httpHelper.SendError(w, "Invalid token", http.StatusUnauthorized)
		return
	}

	sidStr, ok := claims["sub"].(string)
	if !ok {
		log.Error().Msg("Session ID not found in token")
		httpHelper.SendError(w, "Invalid token", http.StatusUnauthorized)
		return
	}

	sid, err := uuid.Parse(sidStr)
	if err != nil {
		log.Error().Err(err).Msg("Invalid session ID format")
		httpHelper.SendError(w, "Invalid session ID", http.StatusUnauthorized)
		return
	}

	err = h.storage.SessionRepo().Delete(ctx, sid)
	if err != nil {
		log.Error().Err(err).Str("session_id", sid.String()).Msg("Failed to delete session")
		httpHelper.SendError(w, "Failed to delete session", http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "access_token",
		Value:    "",
		HttpOnly: true,
		Secure:   true,
		Path:     "/",
		MaxAge:   -1,
		SameSite: http.SameSiteStrictMode,
	})

	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    "",
		HttpOnly: true,
		Secure:   true,
		Path:     "/api/v1/sessions/refresh",
		MaxAge:   -1,
		SameSite: http.SameSiteStrictMode,
	})

	log.Info().Str("session_id", sid.String()).Msg("Session deleted successfully")

	res := httpHelper.Response{
		Success: true,
		Message: "Session deleted successfully",
	}

	httpHelper.SendJSON(w, res, http.StatusOK)
}

func (h *Handler) createAccessToken(sessionID uuid.UUID, user *model.User) (string, error) {
	claims := jwt.MapClaims{
		"sub":      sessionID.String(),
		"user_id":  user.ID,
		"username": user.Username,
		"email":    user.Email,
		"type":     "access",
		"exp":      time.Now().Add(15 * time.Minute).Unix(), // 15 min
		"iat":      time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(h.config.Auth.JWTSecret))
}

func (h *Handler) createRefreshToken(sessionID uuid.UUID) (string, error) {
	claims := jwt.MapClaims{
		"sub":  sessionID.String(),
		"type": "refresh",
		"exp":  time.Now().Add(7 * 24 * time.Hour).Unix(), // 7 days
		"iat":  time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(h.config.Auth.JWTSecret))
}

// RefreshToken godoc
// @Summary      Refresh access token
// @Description  Generate a new access token using a valid refresh token
// @Tags         sessions
// @Produce      json
// @Success      200  {object}  httpHelper.Response  "Token refreshed successfully"
// @Failure      401  {object}  httpHelper.Response  "Unauthorized - invalid or missing refresh token"
// @Failure      500  {object}  httpHelper.Response  "Internal server error"
// @Router       /sessions/refresh [post]
// @Security     ApiKeyAuth
func (h *Handler) RefreshToken(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logging.GetLoggerFromContext(ctx)

	// Get refresh token from cookie
	cookie, err := r.Cookie("refresh_token")
	if err != nil {
		log.Warn().Err(err).Msg("Refresh token missing in request")
		httpHelper.SendError(w, "Refresh token required", http.StatusUnauthorized)
		return
	}

	// Validate refresh token
	token, err := jwt.Parse(cookie.Value, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return []byte(h.config.Auth.JWTSecret), nil
	})

	if err != nil || !token.Valid {
		log.Warn().Err(err).Msg("Invalid refresh token")
		httpHelper.SendError(w, "Invalid refresh token, please login again", http.StatusUnauthorized)
		return
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || claims["type"] != "refresh" {
		log.Error().Msg("Invalid token type")
		httpHelper.SendError(w, "Invalid token type", http.StatusUnauthorized)
		return
	}

	sessionID := claims["sub"].(uuid.UUID)
	sid, err := uuid.Parse(claims["sub"].(string))
	if err != nil {
		log.Error().Err(err).Msg("Invalid session ID format in token")
		httpHelper.SendError(w, "Invalid session ID", http.StatusUnauthorized)
		return
	}

	// Verify session still exists in DB
	session, err := h.storage.SessionRepo().Get(ctx, sid)
	if err != nil {
		log.Warn().Err(err).Str("session_id", sessionID.String()).Msg("Session not found")
		httpHelper.SendError(w, "Session expired, please login again", http.StatusUnauthorized)
		return
	}

	if !session.IsAccessTokenExpired() {
		log.Info().Msg("access token still valid")
		httpHelper.SendError(w, "access token still valid", http.StatusBadRequest)
		return
	}

	if session.IsRefreshTokenExpired() {
		log.Info().Msg("refresh token expired")
		httpHelper.SendError(w, "refresh token expired, please login again", http.StatusUnauthorized)
		return
	}

	// Get user details
	user, err := h.storage.UserRepo().Find(ctx, session.UserID)
	if err != nil {
		log.Error().Err(err).Str("user_id", session.UserID.String()).Msg("User not found")
		httpHelper.SendError(w, "User not found", http.StatusUnauthorized)
		return
	}

	// Create new access token
	accessToken, err := h.createAccessToken(sid, user)
	if err != nil {
		log.Error().Err(err).Msg("Failed to create access token")
		httpHelper.SendError(w, "Failed to refresh token", http.StatusInternalServerError)
		return
	}

	// Set new access token
	http.SetCookie(w, &http.Cookie{
		Name:     "access_token",
		Value:    accessToken,
		HttpOnly: true,
		Secure:   true,
		Path:     "/",
		MaxAge:   1800, // 30 minutes
		SameSite: http.SameSiteStrictMode,
	})

	httpHelper.SendJSON(w, "token refreshed successfully", http.StatusOK)
}

func (h *Handler) VerifySession(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logging.GetLoggerFromContext(ctx)

	cookie, err := r.Cookie("access_token")
	if err != nil {
		log.Error().Err(err).Msg("cannot find JWT token")
		httpHelper.SendUnauthorized(w, "cannot find JWT token")
		return
	}

	token, err := jwt.Parse(cookie.Value, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return []byte(h.config.Auth.JWTSecret), nil
	})

	if err != nil || !token.Valid {
		log.Error().Err(err).Msg("invalid JWT token")
		httpHelper.SendUnauthorized(w, "invalid jwt token")
		return
	}

	log.Info().Msg("session verified successfully")

	res := httpHelper.Response{
		Success: true,
		Message: "session verified successfully",
	}
	httpHelper.SendJSON(w, res, http.StatusOK)
}
