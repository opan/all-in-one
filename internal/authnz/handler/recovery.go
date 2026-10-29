package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/mail"
	"strings"
	"time"

	"github.com/all-in-one/internal/auth"
	"github.com/all-in-one/internal/authnz/model"
	"github.com/all-in-one/internal/email"
	httpHelper "github.com/all-in-one/internal/http"
	"github.com/all-in-one/internal/logging"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// Password recovery and email verification (docs/rfc/RFC-003-password-reset-by-email.md).
const (
	tempPasswordTTL         = 30 * time.Minute
	tempPasswordMaxAttempts = 5
	resetCooldown           = 5 * time.Minute
	resetDailyLimit         = 5
	verificationTTL         = 24 * time.Hour
	verificationDailyLimit  = 5
	emailPromptSnooze       = 7 * 24 * time.Hour
	backgroundEmailTimeout  = 30 * time.Second
)

// AppDirectory names the app (OIDC client) a request came from, so emails
// can say "the account you use in Cashflow". Wired from server.go when the
// OIDC provider is enabled; nil otherwise.
type AppDirectory interface {
	AppName(ctx context.Context, clientID string) (string, bool)
}

// SetMailer turns on account email. Without it, forgot-password and email
// verification answer 503 and recovery goes through the admin only.
func (h *Handler) SetMailer(m *email.Mailer) { h.mailer = m }

func (h *Handler) SetAppDirectory(d AppDirectory) { h.apps = d }

func (h *Handler) appName(ctx context.Context, clientID string) string {
	if h.apps == nil || clientID == "" {
		return ""
	}
	name, _ := h.apps.AppName(ctx, clientID)
	return name
}

// emailLocale picks the email language: the app's language when it sent one.
func emailLocale(locale string) string {
	if locale == "id" {
		return "id"
	}
	return "en"
}

// normalizeEmail trims and lower-cases an address and checks it is a bare
// address (no display name).
func normalizeEmail(raw string) (string, bool) {
	e := strings.ToLower(strings.TrimSpace(raw))
	addr, err := mail.ParseAddress(e)
	if err != nil || addr.Address != e {
		return "", false
	}
	return e, true
}

// background runs work that must not delay or change the response (sending
// a forgot-password email), detached from the request's cancellation.
func (h *Handler) background(ctx context.Context, work func(ctx context.Context)) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), backgroundEmailTimeout)
	h.async(func() {
		defer cancel()
		work(ctx)
	})
}

func (h *Handler) countReset(ctx context.Context, source, result string) {
	h.metrics.passwordResets.Add(ctx, 1, metric.WithAttributes(
		attribute.String("source", source), attribute.String("result", result)))
}

// issueTempPassword creates a temporary password for u, voiding earlier ones,
// and emails it to u's verified address.
func (h *Handler) issueTempPassword(ctx context.Context, u model.User, issuedBy *string, locale, appName string) error {
	temp, err := auth.GenerateTempPassword()
	if err != nil {
		return err
	}
	hash, err := auth.HashPassword(temp)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	reset := model.PasswordReset{
		ID: uuid.New(), UserID: u.ID, TempPasswordHash: hash,
		ExpiresAt: now.Add(tempPasswordTTL), IssuedBy: issuedBy, CreatedAt: now,
	}
	if err := h.storage.PasswordResetRepo().Create(ctx, reset); err != nil {
		return err
	}
	return h.mailer.Send(ctx, email.KindTempPassword, u.Email, emailLocale(locale), email.Data{
		AppName: appName, Username: u.Username, TempPassword: temp, Minutes: int(tempPasswordTTL.Minutes()),
	})
}

// tryTempPassword checks password against u's active temporary passwords and
// burns the one that matches. Wrong guesses count against every active one.
func (h *Handler) tryTempPassword(ctx context.Context, u model.User, password string) bool {
	log := logging.GetLoggerFromContext(ctx)
	if !auth.LooksLikeTempPassword(password) {
		return false
	}
	resets, err := h.storage.PasswordResetRepo().Active(ctx, u.ID)
	if err != nil {
		log.Error().Err(err).Str("user_id", u.ID.String()).Msg("failed to load temporary passwords")
		return false
	}
	normalized := auth.NormalizeTempPassword(password)
	for _, reset := range resets {
		if ok, _ := auth.CheckPassword(normalized, reset.TempPasswordHash); ok {
			used, err := h.storage.PasswordResetRepo().MarkUsed(ctx, reset.ID)
			if err != nil {
				log.Error().Err(err).Msg("failed to burn temporary password")
			}
			return used
		}
	}
	for _, reset := range resets {
		if err := h.storage.PasswordResetRepo().RecordFailedAttempt(ctx, reset.ID, tempPasswordMaxAttempts); err != nil {
			log.Error().Err(err).Msg("failed to count a wrong temporary password")
		}
	}
	return false
}

// shouldPromptEmail decides whether to ask the user to add an email after
// login (RFC-003 §6.6).
func (h *Handler) shouldPromptEmail(ctx context.Context, u model.User) bool {
	if h.mailer == nil || u.HasVerifiedEmail() || h.isDemoUser(u.Username) {
		return false
	}
	if u.EmailPromptedAt != nil && time.Since(*u.EmailPromptedAt) < emailPromptSnooze {
		return false
	}
	pending, err := h.storage.EmailVerificationRepo().Pending(ctx, u.ID)
	return err == nil && pending == nil
}

func (h *Handler) isDemoUser(username string) bool {
	dm := h.config.DemoMode
	return dm.Username != "" && strings.EqualFold(username, dm.Username)
}

// notifyPasswordChanged emails a security notice after a password change.
func (h *Handler) notifyPasswordChanged(ctx context.Context, u model.User, locale, clientID string) {
	if h.mailer == nil || !u.HasVerifiedEmail() {
		return
	}
	h.background(ctx, func(ctx context.Context) {
		err := h.mailer.Send(ctx, email.KindPasswordChanged, u.Email, emailLocale(locale), email.Data{
			AppName: h.appName(ctx, clientID), Username: u.Username,
		})
		if err != nil {
			logging.GetLoggerFromContext(ctx).Error().Err(err).Str("user_id", u.ID.String()).Msg("failed to send password changed notice")
		}
	})
}

type ForgotPasswordRequest struct {
	// Login is a username or a verified email address.
	Login    string `json:"login"`
	Locale   string `json:"locale"`
	ClientID string `json:"client_id"`
}

const forgotPasswordAnswer = "if this account has a verified email, a temporary password is on its way"

// ForgotPassword godoc
// @Summary      Email a temporary password
// @Description  Sends a temporary password to the account's verified email. Always answers the same way, whether or not the account exists or has a verified email. The current password keeps working until a new one is set.
// @Tags         sessions
// @Accept       json
// @Produce      json
// @Param        body  body      ForgotPasswordRequest  true  "Username or email"
// @Success      200   {object}  httpHelper.Response    "Request accepted"
// @Failure      400   {object}  httpHelper.Response    "Missing username or email"
// @Failure      503   {object}  httpHelper.Response    "Email is not set up on this server"
// @Router       /auth/password/forgot [post]
func (h *Handler) ForgotPassword(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if h.mailer == nil {
		httpHelper.SendError(w, "password reset by email is not available", http.StatusServiceUnavailable)
		return
	}
	var req ForgotPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Login) == "" {
		httpHelper.SendError(w, "username or email is required", http.StatusBadRequest)
		return
	}
	login := strings.TrimSpace(req.Login)

	// The lookup, limits and email all happen after the response, so the
	// answer and its timing say nothing about the account.
	h.background(ctx, func(ctx context.Context) {
		h.forgotPassword(ctx, login, req.Locale, req.ClientID)
	})
	httpHelper.SendJSON(w, httpHelper.Response{Success: true, Message: forgotPasswordAnswer}, http.StatusOK)
}

func (h *Handler) forgotPassword(ctx context.Context, login, locale, clientID string) {
	log := logging.GetLoggerFromContext(ctx)
	skip := func(reason string) {
		log.Info().Str("reason", reason).Msg("forgot password: no email sent")
		h.countReset(ctx, "self", "skipped")
	}

	u, err := h.storage.UserRepo().FindByUsername(ctx, login)
	if errors.Is(err, sql.ErrNoRows) && strings.Contains(login, "@") {
		u, err = h.storage.UserRepo().FindByVerifiedEmail(ctx, strings.ToLower(login))
	}
	switch {
	case errors.Is(err, sql.ErrNoRows) || (err == nil && u.ID == uuid.Nil):
		skip("unknown_account")
		return
	case err != nil:
		log.Error().Err(err).Msg("forgot password: user lookup failed")
		h.countReset(ctx, "self", "error")
		return
	case u.Blocked:
		skip("blocked")
		return
	case h.isDemoUser(u.Username):
		skip("demo")
		return
	case !u.HasVerifiedEmail():
		skip("no_verified_email")
		return
	}

	now := time.Now()
	recent, err := h.storage.PasswordResetRepo().CountSince(ctx, u.ID, now.Add(-resetCooldown))
	if err == nil && recent > 0 {
		skip("cooldown")
		return
	}
	today, err2 := h.storage.PasswordResetRepo().CountSince(ctx, u.ID, now.Add(-24*time.Hour))
	if err2 == nil && today >= resetDailyLimit {
		skip("daily_limit")
		return
	}
	if err != nil || err2 != nil {
		log.Error().Err(errors.Join(err, err2)).Msg("forgot password: limit check failed")
		h.countReset(ctx, "self", "error")
		return
	}

	if err := h.issueTempPassword(ctx, u, nil, locale, h.appName(ctx, clientID)); err != nil {
		log.Error().Err(err).Str("user_id", u.ID.String()).Msg("forgot password: failed to issue temporary password")
		h.countReset(ctx, "self", "error")
		return
	}
	log.Info().Str("user_id", u.ID.String()).Msg("forgot password: temporary password sent")
	h.countReset(ctx, "self", "sent")
}

type SetPasswordRequest struct {
	Password string `json:"password"`
	Locale   string `json:"locale"`
	ClientID string `json:"client_id"`
}

// SetPasswordResponse tells the page where to go next.
type SetPasswordResponse struct {
	EmailPrompt bool `json:"email_prompt"`
}

// SetPassword godoc
// @Summary      Choose a new password after logging in with a temporary one
// @Description  Only for a session created with a temporary password. Sets the new password, ends the user's other sessions and lifts the session's restriction.
// @Tags         users
// @Accept       json
// @Produce      json
// @Security     BearerAuth || DirectAuth
// @Param        body  body      SetPasswordRequest  true  "New password"
// @Success      200   {object}  httpHelper.Response{data=SetPasswordResponse}  "Password set"
// @Failure      400   {object}  httpHelper.Response  "Password too short or too long"
// @Failure      409   {object}  httpHelper.Response  "This session has no password change pending"
// @Router       /users/password/set [post]
func (h *Handler) SetPassword(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logging.GetLoggerFromContext(ctx)

	cu, ok := auth.GetUserFromContext(ctx)
	uid, err1 := uuid.Parse(cu.UserID)
	sid, err2 := uuid.Parse(cu.SessionID)
	if !ok || err1 != nil || err2 != nil {
		httpHelper.SendError(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	session, err := h.storage.SessionRepo().Get(ctx, sid)
	if err != nil {
		httpHelper.SendError(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if !session.PasswordChangeRequired {
		httpHelper.SendError(w, "no password change is pending; change it from your settings with your current password", http.StatusConflict)
		return
	}

	var req SetPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpHelper.SendError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if err := auth.ValidatePassword(req.Password); err != nil {
		httpHelper.SendError(w, err.Error(), http.StatusBadRequest)
		return
	}
	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		httpHelper.SendError(w, "failed to set password", http.StatusInternalServerError)
		return
	}
	u, err := h.storage.UserRepo().Find(ctx, uid)
	if err != nil {
		httpHelper.SendError(w, "failed to set password", http.StatusInternalServerError)
		return
	}
	if err := h.storage.UserRepo().SetPasswordHash(ctx, uid, hash); err != nil {
		log.Error().Err(err).Str("user_id", cu.UserID).Msg("failed to store new password")
		httpHelper.SendError(w, "failed to set password", http.StatusInternalServerError)
		return
	}
	if err := h.storage.PasswordResetRepo().VoidActive(ctx, uid); err != nil {
		log.Error().Err(err).Msg("failed to void temporary passwords")
	}
	if err := h.storage.SessionRepo().DeleteByUserIDExcept(ctx, uid, sid); err != nil {
		log.Error().Err(err).Msg("failed to end other sessions after password reset")
	}
	if err := h.storage.SessionRepo().ClearPasswordChangeRequired(ctx, sid); err != nil {
		log.Error().Err(err).Msg("failed to lift the session restriction")
		httpHelper.SendError(w, "failed to set password", http.StatusInternalServerError)
		return
	}

	h.notifyPasswordChanged(ctx, u, req.Locale, req.ClientID)
	log.Info().Str("user_id", cu.UserID).Msg("password set after temporary password")
	httpHelper.SendJSON(w, httpHelper.Response{
		Success: true,
		Message: "password set",
		Data:    SetPasswordResponse{EmailPrompt: h.shouldPromptEmail(ctx, u)},
	}, http.StatusOK)
}

type UpdateMyEmailRequest struct {
	Email    string `json:"email"`
	Locale   string `json:"locale"`
	ClientID string `json:"client_id"`
}

// UpdateMyEmail godoc
// @Summary      Add or change your email
// @Description  Sends a verification link to the address. It replaces the current email only once the link is opened.
// @Tags         users
// @Accept       json
// @Produce      json
// @Security     BearerAuth || DirectAuth
// @Param        body  body      UpdateMyEmailRequest  true  "New email"
// @Success      200   {object}  httpHelper.Response   "Verification link sent"
// @Failure      400   {object}  httpHelper.Response   "Invalid email, or already your email"
// @Failure      403   {object}  httpHelper.Response   "Not available for the demo account"
// @Failure      409   {object}  httpHelper.Response   "This email can't be used"
// @Failure      429   {object}  httpHelper.Response   "Too many verification emails today"
// @Failure      502   {object}  httpHelper.Response   "The email could not be sent"
// @Failure      503   {object}  httpHelper.Response   "Email is not set up on this server"
// @Router       /users/me/email [put]
func (h *Handler) UpdateMyEmail(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logging.GetLoggerFromContext(ctx)
	if h.mailer == nil {
		httpHelper.SendError(w, "adding an email is not available on this server", http.StatusServiceUnavailable)
		return
	}
	cu, ok := auth.GetUserFromContext(ctx)
	uid, err := uuid.Parse(cu.UserID)
	if !ok || err != nil {
		httpHelper.SendError(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	var req UpdateMyEmailRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpHelper.SendError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	addr, ok := normalizeEmail(req.Email)
	if !ok {
		httpHelper.SendError(w, "invalid email address", http.StatusBadRequest)
		return
	}
	u, err := h.storage.UserRepo().Find(ctx, uid)
	if err != nil {
		httpHelper.SendError(w, "failed to update email", http.StatusInternalServerError)
		return
	}
	if h.isDemoUser(u.Username) {
		httpHelper.SendError(w, "the demo account can't have an email", http.StatusForbidden)
		return
	}
	if u.HasVerifiedEmail() && strings.EqualFold(u.Email, addr) {
		httpHelper.SendError(w, "this is already your email", http.StatusBadRequest)
		return
	}
	if other, err := h.storage.UserRepo().FindByVerifiedEmail(ctx, addr); err == nil && other.ID != uid {
		h.metrics.emailVerifications.Add(ctx, 1, metric.WithAttributes(attribute.String("result", "taken")))
		httpHelper.SendError(w, "this email can't be used", http.StatusConflict)
		return
	}
	if n, err := h.storage.EmailVerificationRepo().CountSince(ctx, uid, time.Now().Add(-24*time.Hour)); err != nil || n >= verificationDailyLimit {
		httpHelper.SendError(w, "too many verification emails today; try again tomorrow", http.StatusTooManyRequests)
		return
	}

	if err := h.sendVerification(ctx, u, addr, req.Locale, req.ClientID); err != nil {
		log.Error().Err(err).Str("user_id", cu.UserID).Msg("failed to send verification email")
		h.metrics.emailVerifications.Add(ctx, 1, metric.WithAttributes(attribute.String("result", "error")))
		httpHelper.SendError(w, "the email could not be sent; try again later", http.StatusBadGateway)
		return
	}
	h.metrics.emailVerifications.Add(ctx, 1, metric.WithAttributes(attribute.String("result", "sent")))
	httpHelper.SendJSON(w, httpHelper.Response{Success: true, Message: "verification link sent",
		Data: map[string]string{"pending_email": addr}}, http.StatusOK)
}

// sendVerification stores a pending email for u and emails its link.
func (h *Handler) sendVerification(ctx context.Context, u model.User, addr, locale, clientID string) error {
	token, tokenHash, err := auth.GenerateLinkToken()
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	v := model.EmailVerification{ID: uuid.New(), UserID: u.ID, Email: addr, TokenHash: tokenHash,
		ExpiresAt: now.Add(verificationTTL), CreatedAt: now}
	if err := h.storage.EmailVerificationRepo().Create(ctx, v); err != nil {
		return fmt.Errorf("store email verification: %w", err)
	}
	return h.mailer.Send(ctx, email.KindVerifyEmail, addr, emailLocale(locale), email.Data{
		AppName: h.appName(ctx, clientID), Username: u.Username, Link: h.config.PublicBaseURL() + "/verify-email?token=" + token,
	})
}

type VerifyEmailRequest struct {
	Token string `json:"token"`
}

// VerifyEmail godoc
// @Summary      Confirm an email address
// @Description  Opens the link from a verification email. Works without being logged in.
// @Tags         users
// @Accept       json
// @Produce      json
// @Param        body  body      VerifyEmailRequest   true  "Token from the link"
// @Success      200   {object}  httpHelper.Response  "Email confirmed"
// @Failure      400   {object}  httpHelper.Response  "Invalid or expired link"
// @Failure      409   {object}  httpHelper.Response  "This email can't be used"
// @Router       /auth/email/verify [post]
func (h *Handler) VerifyEmail(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logging.GetLoggerFromContext(ctx)
	count := func(result string) {
		h.metrics.emailVerifications.Add(ctx, 1, metric.WithAttributes(attribute.String("result", result)))
	}

	var req VerifyEmailRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Token == "" {
		httpHelper.SendError(w, "this link is invalid or has expired", http.StatusBadRequest)
		return
	}
	v, err := h.storage.EmailVerificationRepo().FindUsable(ctx, auth.HashLinkToken(req.Token))
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			log.Error().Err(err).Msg("failed to look up email verification")
		}
		count("invalid")
		httpHelper.SendError(w, "this link is invalid or has expired", http.StatusBadRequest)
		return
	}
	u, err := h.storage.UserRepo().Find(ctx, v.UserID)
	if err != nil {
		count("invalid")
		httpHelper.SendError(w, "this link is invalid or has expired", http.StatusBadRequest)
		return
	}
	previous, hadVerified := u.Email, u.HasVerifiedEmail()

	if err := h.storage.UserRepo().SetVerifiedEmail(ctx, u.ID, v.Email, time.Now()); err != nil {
		if isUniqueViolation(err) {
			count("taken")
			httpHelper.SendError(w, "this email can't be used", http.StatusConflict)
			return
		}
		log.Error().Err(err).Msg("failed to store verified email")
		httpHelper.SendError(w, "failed to confirm email", http.StatusInternalServerError)
		return
	}
	if _, err := h.storage.EmailVerificationRepo().MarkUsed(ctx, v.ID); err != nil {
		log.Error().Err(err).Msg("failed to mark email verification used")
	}

	if hadVerified && h.mailer != nil && !strings.EqualFold(previous, v.Email) {
		h.background(ctx, func(ctx context.Context) {
			err := h.mailer.Send(ctx, email.KindEmailChanged, previous, "en", email.Data{
				Username: u.Username, NewEmail: email.MaskAddress(v.Email),
			})
			if err != nil {
				logging.GetLoggerFromContext(ctx).Error().Err(err).Msg("failed to send email changed notice")
			}
		})
	}
	count("verified")
	log.Info().Str("user_id", u.ID.String()).Msg("email verified")
	httpHelper.SendJSON(w, httpHelper.Response{Success: true, Message: "email confirmed",
		Data: map[string]string{"email": v.Email}}, http.StatusOK)
}

// SnoozeEmailPrompt godoc
// @Summary      Ask again later about adding an email
// @Description  Hides the "add your email" prompt shown after login for 7 days.
// @Tags         users
// @Produce      json
// @Security     BearerAuth || DirectAuth
// @Success      200  {object}  httpHelper.Response  "Prompt snoozed"
// @Router       /users/me/email-prompt/snooze [post]
func (h *Handler) SnoozeEmailPrompt(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	cu, ok := auth.GetUserFromContext(ctx)
	uid, err := uuid.Parse(cu.UserID)
	if !ok || err != nil {
		httpHelper.SendError(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if err := h.storage.UserRepo().SetEmailPrompted(ctx, uid, time.Now()); err != nil {
		logging.GetLoggerFromContext(ctx).Error().Err(err).Msg("failed to snooze email prompt")
		httpHelper.SendError(w, "failed to snooze", http.StatusInternalServerError)
		return
	}
	httpHelper.SendJSON(w, httpHelper.Response{Success: true, Message: "prompt snoozed"}, http.StatusOK)
}

type AdminPasswordResetRequest struct {
	Locale string `json:"locale"`
}

// AdminSendTempPassword godoc
// @Summary      Email a user a temporary password (admin)
// @Description  Admin-only. Emails a temporary password to the user's verified email; the admin never sees it. Confirm the user's identity first (RFC-003 §6.5).
// @Tags         admin-users
// @Accept       json
// @Produce      json
// @Security     BearerAuth || DirectAuth
// @Param        id    path      string                     true   "User ID"
// @Param        body  body      AdminPasswordResetRequest  false  "Email language"
// @Success      200   {object}  httpHelper.Response  "Temporary password sent"
// @Failure      404   {object}  httpHelper.Response  "User not found"
// @Failure      409   {object}  httpHelper.Response  "User has no verified email, or is blocked"
// @Failure      502   {object}  httpHelper.Response  "The email could not be sent"
// @Failure      503   {object}  httpHelper.Response  "Email is not set up on this server"
// @Router       /admin/users/{id}/password-reset [post]
func (h *Handler) AdminSendTempPassword(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logging.GetLoggerFromContext(ctx)
	if h.mailer == nil {
		httpHelper.SendError(w, "email is not set up on this server", http.StatusServiceUnavailable)
		return
	}
	id, ok := parseUserIDParam(w, r)
	if !ok {
		return
	}
	var req AdminPasswordResetRequest
	_ = json.NewDecoder(r.Body).Decode(&req)

	u, err := h.storage.UserRepo().Find(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			httpHelper.SendError(w, "user not found", http.StatusNotFound)
			return
		}
		httpHelper.SendError(w, "internal server error", http.StatusInternalServerError)
		return
	}
	switch {
	case !u.HasVerifiedEmail():
		httpHelper.SendError(w, "the user has no verified email; save their email as verified first", http.StatusConflict)
		return
	case u.Blocked:
		httpHelper.SendError(w, "the user is blocked; unblock them first", http.StatusConflict)
		return
	}

	admin, _ := auth.GetUserFromContext(ctx)
	issuedBy := admin.UserID
	if err := h.issueTempPassword(ctx, u, &issuedBy, req.Locale, ""); err != nil {
		log.Error().Err(err).Str("user_id", id.String()).Msg("admin reset: failed to send temporary password")
		h.countReset(ctx, "admin", "error")
		httpHelper.SendError(w, "the email could not be sent; try again later", http.StatusBadGateway)
		return
	}
	h.countReset(ctx, "admin", "sent")
	log.Info().Str("user_id", id.String()).Str("admin", admin.Username).Str("reason", "admin_reset").Msg("admin sent a temporary password")
	httpHelper.SendJSON(w, httpHelper.Response{Success: true,
		Message: "temporary password sent to " + email.MaskAddress(u.Email)}, http.StatusOK)
}
