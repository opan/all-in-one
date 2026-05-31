package handler

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/all-in-one/internal/auth"
	"github.com/all-in-one/internal/authnz/model"
	httpHelper "github.com/all-in-one/internal/http"
	"github.com/all-in-one/internal/logging"
	jwt "github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/pquerna/otp/totp"
	qrcode "github.com/skip2/go-qrcode"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

const (
	challengeTTL     = 5 * time.Minute
	maxAttempts      = 5
	recoveryCodeCount = 10
)

func (h *Handler) SetupTOTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logging.GetLoggerFromContext(ctx)

	cu, ok := auth.GetUserFromContext(ctx)
	if !ok {
		httpHelper.SendError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	uid, err := uuid.Parse(cu.UserID)
	if err != nil {
		httpHelper.SendError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	user, err := h.storage.UserRepo().Find(ctx, uid)
	if err != nil {
		log.Error().Err(err).Msg("failed to find user")
		httpHelper.SendError(w, "Failed to retrieve user", http.StatusInternalServerError)
		return
	}

	if user.TOTPEnabled {
		httpHelper.SendError(w, "2FA is already enabled", http.StatusConflict)
		return
	}

	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      "AllInOne",
		AccountName: user.Email,
	})
	if err != nil {
		log.Error().Err(err).Msg("failed to generate TOTP key")
		httpHelper.SendError(w, "Failed to generate 2FA secret", http.StatusInternalServerError)
		return
	}

	encKey, err := auth.ParseEncryptionKey(h.config.Auth.TOTPEncryptionKey)
	if err != nil {
		log.Error().Err(err).Msg("invalid TOTP encryption key")
		httpHelper.SendError(w, "Server configuration error", http.StatusInternalServerError)
		return
	}

	encrypted, err := auth.EncryptTOTPSecret(key.Secret(), encKey)
	if err != nil {
		log.Error().Err(err).Msg("failed to encrypt TOTP secret")
		httpHelper.SendError(w, "Failed to setup 2FA", http.StatusInternalServerError)
		return
	}

	user.TOTPSecretEncrypted = &encrypted
	if err := h.storage.UserRepo().Update(ctx, uid, user); err != nil {
		log.Error().Err(err).Msg("failed to store TOTP secret")
		httpHelper.SendError(w, "Failed to setup 2FA", http.StatusInternalServerError)
		return
	}

	png, err := qrcode.Encode(key.URL(), qrcode.Medium, 256)
	if err != nil {
		log.Error().Err(err).Msg("failed to generate QR code")
		httpHelper.SendError(w, "Failed to generate QR code", http.StatusInternalServerError)
		return
	}
	qrBase64 := base64.StdEncoding.EncodeToString(png)

	plaintextCodes, err := auth.GenerateRecoveryCodes(recoveryCodeCount)
	if err != nil {
		log.Error().Err(err).Msg("failed to generate recovery codes")
		httpHelper.SendError(w, "Failed to generate recovery codes", http.StatusInternalServerError)
		return
	}

	// Delete any existing recovery codes first
	if err := h.storage.TOTPRepo().DeleteRecoveryCodesByUserID(ctx, uid); err != nil {
		log.Error().Err(err).Msg("failed to delete old recovery codes")
	}

	now := time.Now().UTC()
	recoveryCodes := make([]model.RecoveryCode, len(plaintextCodes))
	for i, code := range plaintextCodes {
		hash, err := auth.HashRecoveryCode(code)
		if err != nil {
			log.Error().Err(err).Msg("failed to hash recovery code")
			httpHelper.SendError(w, "Failed to setup 2FA", http.StatusInternalServerError)
			return
		}
		recoveryCodes[i] = model.RecoveryCode{
			ID:        uuid.New(),
			UserID:    uid,
			CodeHash:  hash,
			CreatedAt: now,
		}
	}

	if err := h.storage.TOTPRepo().CreateRecoveryCodes(ctx, recoveryCodes); err != nil {
		log.Error().Err(err).Msg("failed to store recovery codes")
		httpHelper.SendError(w, "Failed to setup 2FA", http.StatusInternalServerError)
		return
	}

	log.Info().Str("user_id", uid.String()).Msg("2FA setup initiated")

	resp := httpHelper.Response{
		Success: true,
		Data: model.TOTPSetupResponse{
			QRCode:        qrBase64,
			Secret:        key.Secret(),
			RecoveryCodes: plaintextCodes,
		},
	}
	httpHelper.SendJSON(w, resp, http.StatusOK)
}

func (h *Handler) VerifyTOTPSetup(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logging.GetLoggerFromContext(ctx)

	cu, ok := auth.GetUserFromContext(ctx)
	if !ok {
		httpHelper.SendError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	uid, err := uuid.Parse(cu.UserID)
	if err != nil {
		httpHelper.SendError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req model.TOTPVerifyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpHelper.SendError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	user, err := h.storage.UserRepo().Find(ctx, uid)
	if err != nil {
		log.Error().Err(err).Msg("failed to find user")
		httpHelper.SendError(w, "Failed to retrieve user", http.StatusInternalServerError)
		return
	}

	if user.TOTPEnabled {
		httpHelper.SendError(w, "2FA is already enabled", http.StatusConflict)
		return
	}

	if user.TOTPSecretEncrypted == nil {
		httpHelper.SendError(w, "2FA setup not initiated", http.StatusBadRequest)
		return
	}

	encKey, err := auth.ParseEncryptionKey(h.config.Auth.TOTPEncryptionKey)
	if err != nil {
		log.Error().Err(err).Msg("invalid TOTP encryption key")
		httpHelper.SendError(w, "Server configuration error", http.StatusInternalServerError)
		return
	}

	secret, err := auth.DecryptTOTPSecret(*user.TOTPSecretEncrypted, encKey)
	if err != nil {
		log.Error().Err(err).Msg("failed to decrypt TOTP secret")
		httpHelper.SendError(w, "Failed to verify 2FA", http.StatusInternalServerError)
		return
	}

	valid := totp.Validate(req.Code, secret)
	if !valid {
		httpHelper.SendError(w, "Invalid verification code", http.StatusUnauthorized)
		return
	}

	now := time.Now().UTC()
	user.TOTPEnabled = true
	user.TOTPVerifiedAt = &now
	if err := h.storage.UserRepo().Update(ctx, uid, user); err != nil {
		log.Error().Err(err).Msg("failed to enable 2FA")
		httpHelper.SendError(w, "Failed to enable 2FA", http.StatusInternalServerError)
		return
	}

	// Invalidate ALL sessions to force re-login with 2FA
	if err := h.storage.SessionRepo().DeleteByUserID(ctx, uid); err != nil {
		log.Error().Err(err).Msg("failed to invalidate sessions after enabling 2FA")
	}

	log.Info().Str("user_id", uid.String()).Msg("2FA successfully enabled")

	h.metrics.twoFAStateChanges.Add(ctx, 1, metric.WithAttributes(attribute.String("action", "enabled")))

	resp := httpHelper.Response{
		Success: true,
		Message: "Two-factor authentication has been enabled. Please log in again.",
	}
	httpHelper.SendJSON(w, resp, http.StatusOK)
}

func (h *Handler) DisableTOTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logging.GetLoggerFromContext(ctx)

	cu, ok := auth.GetUserFromContext(ctx)
	if !ok {
		httpHelper.SendError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	uid, err := uuid.Parse(cu.UserID)
	if err != nil {
		httpHelper.SendError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req model.DisableTOTPRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpHelper.SendError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.CurrentPassword == "" {
		httpHelper.SendError(w, "Current password is required", http.StatusBadRequest)
		return
	}

	user, err := h.storage.UserRepo().Find(ctx, uid)
	if err != nil {
		log.Error().Err(err).Msg("failed to find user")
		httpHelper.SendError(w, "Failed to retrieve user", http.StatusInternalServerError)
		return
	}

	ok, err = auth.CheckPassword(req.CurrentPassword, user.PasswordHash)
	if err != nil || !ok {
		httpHelper.SendError(w, "Invalid password", http.StatusUnauthorized)
		return
	}

	user.TOTPEnabled = false
	user.TOTPSecretEncrypted = nil
	user.TOTPVerifiedAt = nil
	if err := h.storage.UserRepo().Update(ctx, uid, user); err != nil {
		log.Error().Err(err).Msg("failed to disable 2FA")
		httpHelper.SendError(w, "Failed to disable 2FA", http.StatusInternalServerError)
		return
	}

	if err := h.storage.TOTPRepo().DeleteRecoveryCodesByUserID(ctx, uid); err != nil {
		log.Error().Err(err).Msg("failed to delete recovery codes")
	}

	// Invalidate all OTHER sessions
	sessionID, _ := uuid.Parse(cu.SessionID)
	if err := h.storage.SessionRepo().DeleteByUserIDExcept(ctx, uid, sessionID); err != nil {
		log.Error().Err(err).Msg("failed to invalidate other sessions")
	}

	log.Info().Str("user_id", uid.String()).Msg("2FA disabled")

	h.metrics.twoFAStateChanges.Add(ctx, 1, metric.WithAttributes(attribute.String("action", "disabled")))

	resp := httpHelper.Response{
		Success: true,
		Message: "Two-factor authentication has been disabled",
	}
	httpHelper.SendJSON(w, resp, http.StatusOK)
}

func (h *Handler) GetTOTPStatus(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logging.GetLoggerFromContext(ctx)

	cu, ok := auth.GetUserFromContext(ctx)
	if !ok {
		httpHelper.SendError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	uid, err := uuid.Parse(cu.UserID)
	if err != nil {
		httpHelper.SendError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	user, err := h.storage.UserRepo().Find(ctx, uid)
	if err != nil {
		log.Error().Err(err).Msg("failed to find user")
		httpHelper.SendError(w, "Failed to retrieve user", http.StatusInternalServerError)
		return
	}

	remaining := 0
	if user.TOTPEnabled {
		codes, err := h.storage.TOTPRepo().GetUnusedRecoveryCodes(ctx, uid)
		if err != nil {
			log.Error().Err(err).Msg("failed to get recovery codes count")
		} else {
			remaining = len(codes)
		}
	}

	resp := httpHelper.Response{
		Success: true,
		Data: model.TOTPStatusResponse{
			Enabled:                 user.TOTPEnabled,
			RecoveryCodesRemaining: remaining,
		},
	}
	httpHelper.SendJSON(w, resp, http.StatusOK)
}

func (h *Handler) RegenerateRecoveryCodes(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logging.GetLoggerFromContext(ctx)

	cu, ok := auth.GetUserFromContext(ctx)
	if !ok {
		httpHelper.SendError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	uid, err := uuid.Parse(cu.UserID)
	if err != nil {
		httpHelper.SendError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req model.RegenerateRecoveryCodesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpHelper.SendError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.CurrentPassword == "" {
		httpHelper.SendError(w, "Current password is required", http.StatusBadRequest)
		return
	}

	user, err := h.storage.UserRepo().Find(ctx, uid)
	if err != nil {
		log.Error().Err(err).Msg("failed to find user")
		httpHelper.SendError(w, "Failed to retrieve user", http.StatusInternalServerError)
		return
	}

	if !user.TOTPEnabled {
		httpHelper.SendError(w, "2FA is not enabled", http.StatusBadRequest)
		return
	}

	ok, err = auth.CheckPassword(req.CurrentPassword, user.PasswordHash)
	if err != nil || !ok {
		httpHelper.SendError(w, "Invalid password", http.StatusUnauthorized)
		return
	}

	if err := h.storage.TOTPRepo().DeleteRecoveryCodesByUserID(ctx, uid); err != nil {
		log.Error().Err(err).Msg("failed to delete old recovery codes")
		httpHelper.SendError(w, "Failed to regenerate recovery codes", http.StatusInternalServerError)
		return
	}

	plaintextCodes, err := auth.GenerateRecoveryCodes(recoveryCodeCount)
	if err != nil {
		log.Error().Err(err).Msg("failed to generate recovery codes")
		httpHelper.SendError(w, "Failed to regenerate recovery codes", http.StatusInternalServerError)
		return
	}

	now := time.Now().UTC()
	recoveryCodes := make([]model.RecoveryCode, len(plaintextCodes))
	for i, code := range plaintextCodes {
		hash, err := auth.HashRecoveryCode(code)
		if err != nil {
			log.Error().Err(err).Msg("failed to hash recovery code")
			httpHelper.SendError(w, "Failed to regenerate recovery codes", http.StatusInternalServerError)
			return
		}
		recoveryCodes[i] = model.RecoveryCode{
			ID:        uuid.New(),
			UserID:    uid,
			CodeHash:  hash,
			CreatedAt: now,
		}
	}

	if err := h.storage.TOTPRepo().CreateRecoveryCodes(ctx, recoveryCodes); err != nil {
		log.Error().Err(err).Msg("failed to store recovery codes")
		httpHelper.SendError(w, "Failed to regenerate recovery codes", http.StatusInternalServerError)
		return
	}

	log.Info().Str("user_id", uid.String()).Msg("recovery codes regenerated")

	resp := httpHelper.Response{
		Success: true,
		Data: model.RegenerateRecoveryCodesResponse{
			RecoveryCodes: plaintextCodes,
		},
	}
	httpHelper.SendJSON(w, resp, http.StatusOK)
}

// VerifyTOTP handles the 2FA verification step during login
func (h *Handler) VerifyTOTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logging.GetLoggerFromContext(ctx)

	var req model.TOTPLoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpHelper.SendError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	challenge, user, err := h.validateChallengeToken(ctx, req.ChallengeToken)
	if err != nil {
		log.Warn().Err(err).Msg("invalid challenge token")
		httpHelper.SendError(w, "Invalid or expired challenge", http.StatusUnauthorized)
		return
	}

	if challenge.Attempts >= maxAttempts {
		h.storage.TOTPRepo().DeleteChallenge(ctx, challenge.ID)
		log.Warn().Str("user_id", challenge.UserID.String()).Msg("max 2FA attempts exceeded")
		httpHelper.SendError(w, "Too many attempts, please login again", http.StatusTooManyRequests)
		return
	}

	if err := h.storage.TOTPRepo().IncrementChallengeAttempts(ctx, challenge.ID); err != nil {
		log.Error().Err(err).Msg("failed to increment challenge attempts")
	}

	encKey, err := auth.ParseEncryptionKey(h.config.Auth.TOTPEncryptionKey)
	if err != nil {
		log.Error().Err(err).Msg("invalid TOTP encryption key")
		httpHelper.SendError(w, "Server error", http.StatusInternalServerError)
		return
	}

	secret, err := auth.DecryptTOTPSecret(*user.TOTPSecretEncrypted, encKey)
	if err != nil {
		log.Error().Err(err).Msg("failed to decrypt TOTP secret")
		httpHelper.SendError(w, "Server error", http.StatusInternalServerError)
		return
	}

	valid := totp.Validate(req.Code, secret)
	if !valid {
		log.Warn().Str("user_id", user.ID.String()).Int("attempts", challenge.Attempts+1).Msg("invalid 2FA code")
		h.metrics.twoFAVerifications.Add(ctx, 1, metric.WithAttributes(
			attribute.String("method", "totp"),
			attribute.String("result", "failure"),
		))
		httpHelper.SendError(w, "Invalid verification code", http.StatusUnauthorized)
		return
	}

	h.storage.TOTPRepo().DeleteChallenge(ctx, challenge.ID)
	h.metrics.twoFAVerifications.Add(ctx, 1, metric.WithAttributes(
		attribute.String("method", "totp"),
		attribute.String("result", "success"),
	))
	h.completeLogin(w, r, user)
}

// VerifyRecoveryCode handles recovery code verification during login
func (h *Handler) VerifyRecoveryCode(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logging.GetLoggerFromContext(ctx)

	var req model.RecoveryCodeLoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpHelper.SendError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	challenge, user, err := h.validateChallengeToken(ctx, req.ChallengeToken)
	if err != nil {
		log.Warn().Err(err).Msg("invalid challenge token")
		httpHelper.SendError(w, "Invalid or expired challenge", http.StatusUnauthorized)
		return
	}

	if challenge.Attempts >= maxAttempts {
		h.storage.TOTPRepo().DeleteChallenge(ctx, challenge.ID)
		log.Warn().Str("user_id", challenge.UserID.String()).Msg("max recovery code attempts exceeded")
		httpHelper.SendError(w, "Too many attempts, please login again", http.StatusTooManyRequests)
		return
	}

	if err := h.storage.TOTPRepo().IncrementChallengeAttempts(ctx, challenge.ID); err != nil {
		log.Error().Err(err).Msg("failed to increment challenge attempts")
	}

	codes, err := h.storage.TOTPRepo().GetUnusedRecoveryCodes(ctx, user.ID)
	if err != nil {
		log.Error().Err(err).Msg("failed to get recovery codes")
		httpHelper.SendError(w, "Server error", http.StatusInternalServerError)
		return
	}

	var matchedCodeID *uuid.UUID
	for _, code := range codes {
		if auth.CheckRecoveryCode(req.RecoveryCode, code.CodeHash) {
			id := code.ID
			matchedCodeID = &id
			break
		}
	}

	if matchedCodeID == nil {
		log.Warn().Str("user_id", user.ID.String()).Msg("invalid recovery code")
		h.metrics.twoFAVerifications.Add(ctx, 1, metric.WithAttributes(
			attribute.String("method", "recovery_code"),
			attribute.String("result", "failure"),
		))
		httpHelper.SendError(w, "Invalid recovery code", http.StatusUnauthorized)
		return
	}

	if err := h.storage.TOTPRepo().MarkRecoveryCodeUsed(ctx, *matchedCodeID); err != nil {
		log.Error().Err(err).Msg("failed to mark recovery code as used")
	}

	remaining := len(codes) - 1
	if remaining <= 2 {
		log.Warn().Str("user_id", user.ID.String()).Int("remaining", remaining).Msg("low recovery codes remaining")
	}

	h.storage.TOTPRepo().DeleteChallenge(ctx, challenge.ID)
	h.metrics.twoFAVerifications.Add(ctx, 1, metric.WithAttributes(
		attribute.String("method", "recovery_code"),
		attribute.String("result", "success"),
	))
	h.completeLogin(w, r, user)
}

func (h *Handler) validateChallengeToken(ctx context.Context, tokenStr string) (*model.TOTPChallenge, model.User, error) {
	token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return []byte(h.config.Auth.JWTSecret), nil
	})
	if err != nil || !token.Valid {
		return nil, model.User{}, fmt.Errorf("invalid challenge token: %w", err)
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || claims["type"] != "2fa_challenge" {
		return nil, model.User{}, fmt.Errorf("invalid token type")
	}

	challengeID, err := uuid.Parse(claims["sub"].(string))
	if err != nil {
		return nil, model.User{}, fmt.Errorf("invalid challenge ID: %w", err)
	}

	challenge, err := h.storage.TOTPRepo().GetChallenge(ctx, challengeID)
	if err != nil {
		return nil, model.User{}, fmt.Errorf("challenge not found: %w", err)
	}

	if time.Now().After(challenge.ExpiresAt) {
		h.storage.TOTPRepo().DeleteChallenge(ctx, challenge.ID)
		return nil, model.User{}, fmt.Errorf("challenge expired")
	}

	user, err := h.storage.UserRepo().Find(ctx, challenge.UserID)
	if err != nil {
		return nil, model.User{}, fmt.Errorf("user not found: %w", err)
	}

	return challenge, user, nil
}

func (h *Handler) completeLogin(w http.ResponseWriter, r *http.Request, user model.User) {
	ctx := r.Context()
	log := logging.GetLoggerFromContext(ctx)

	sid, err := uuid.NewUUID()
	if err != nil {
		httpHelper.SendError(w, "Failed to create session", http.StatusInternalServerError)
		return
	}

	s := model.Session{
		ID:                 sid,
		UserID:             user.ID,
		CreatedAt:          time.Now(),
		UserAgent:          r.UserAgent(),
		AccessTokenExpiry:  int(time.Now().Add(30 * time.Minute).Unix()),
		RefreshTokenExpiry: int(time.Now().Add(7 * 24 * time.Hour).Unix()),
	}

	trx, err := h.storage.SessionRepo().CreateTrx(ctx)
	if err != nil {
		httpHelper.SendError(w, "Failed to create session", http.StatusInternalServerError)
		return
	}
	defer trx.Rollback()

	if err := h.storage.SessionRepo().Create(ctx, s, trx); err != nil {
		httpHelper.SendError(w, "Failed to create session", http.StatusInternalServerError)
		return
	}

	at, err := h.createAccessToken(sid, user)
	if err != nil {
		log.Error().Err(err).Msg("failed to create access token")
		httpHelper.SendError(w, "Failed to create session", http.StatusInternalServerError)
		return
	}

	rt, err := h.createRefreshToken(sid)
	if err != nil {
		log.Error().Err(err).Msg("failed to create refresh token")
		httpHelper.SendError(w, "Failed to create session", http.StatusInternalServerError)
		return
	}

	if err := trx.Commit(); err != nil {
		httpHelper.SendError(w, "Failed to create session", http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "access_token",
		Value:    at,
		HttpOnly: true,
		Secure:   h.config.Auth.SecureCookie,
		Path:     "/",
		MaxAge:   1800,
		SameSite: http.SameSiteLaxMode,
	})

	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    rt,
		HttpOnly: true,
		Secure:   h.config.Auth.SecureCookie,
		Path:     "/",
		MaxAge:   604800,
		SameSite: http.SameSiteLaxMode,
	})

	log.Info().Str("session_id", sid.String()).Msg("session successfully created via 2FA")

	resp := httpHelper.Response{
		Success: true,
		Data:    user,
	}
	httpHelper.SendJSON(w, resp, http.StatusCreated)
}

func (h *Handler) createChallengeToken(challengeID uuid.UUID, expiresAt time.Time) (string, error) {
	claims := jwt.MapClaims{
		"sub":  challengeID.String(),
		"type": "2fa_challenge",
		"exp":  expiresAt.Unix(),
		"iat":  time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(h.config.Auth.JWTSecret))
}
