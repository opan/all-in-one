package handler

import (
	"errors"
	"net/http"

	"github.com/all-in-one/internal/auth"
	httpHelper "github.com/all-in-one/internal/http"
	"github.com/all-in-one/internal/logging"
	"github.com/all-in-one/internal/oidc"
	"github.com/gorilla/mux"
	"golang.org/x/text/language"
)

type completeResponse struct {
	RedirectURL string `json:"redirect_url"`
}

// GetAuthRequest godoc
// @Summary      Describe a pending app login
// @Description  Returns which app started a login through aio and whether it asked for the signup form. Public: aio's login page calls it before the user is logged in.
// @Tags         oidc
// @Produce      json
// @Param        id  path  string  true  "Auth request id"
// @Success      200  {object}  httpHelper.Response{data=model.AuthRequestInfo}  "Auth request"
// @Failure      404  {object}  httpHelper.Response  "Unknown or expired auth request"
// @Router       /oidc/auth-requests/{id} [get]
func (h *Handler) GetAuthRequest(w http.ResponseWriter, r *http.Request) {
	info, err := h.service.AuthRequestInfo(r.Context(), mux.Vars(r)["id"])
	if err != nil {
		h.sendError(w, r, err)
		return
	}
	httpHelper.SendJSON(w, httpHelper.Response{Success: true, Data: info}, http.StatusOK)
}

// CompleteAuthRequest godoc
// @Summary      Finish a pending app login
// @Description  Attaches the logged-in aio user to the auth request and returns the URL the browser must visit next; aio then redirects back to the app with an authorization code.
// @Tags         oidc
// @Produce      json
// @Param        id  path  string  true  "Auth request id"
// @Security     BearerAuth || DirectAuth
// @Success      200  {object}  httpHelper.Response{data=completeResponse}  "Where to send the browser next"
// @Failure      401  {object}  httpHelper.Response  "Not logged in to aio"
// @Failure      403  {object}  httpHelper.Response  "User is blocked, or the login was started in another browser"
// @Failure      404  {object}  httpHelper.Response  "Unknown or expired auth request, or the app was revoked"
// @Router       /oidc/auth-requests/{id}/complete [post]
func (h *Handler) CompleteAuthRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	claims, ok := auth.GetUserFromContext(ctx)
	if !ok || claims.UserID == "" {
		httpHelper.SendError(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	id := mux.Vars(r)["id"]
	next, err := h.service.CompleteAuthRequest(ctx, id, claims.UserID, oidc.BrowserIDFromRequest(r))
	if err != nil {
		h.sendError(w, r, err)
		return
	}
	info, _ := h.service.AuthRequestInfo(ctx, id)
	h.metrics.loginCompleted(ctx, info.ClientID)
	logging.GetLoggerFromContext(ctx).Info().Str("client_id", info.ClientID).Str("username", claims.Username).
		Msg("oidc: login completed")
	httpHelper.SendJSON(w, httpHelper.Response{Success: true, Data: completeResponse{RedirectURL: next}}, http.StatusOK)
}

// authRequestErrors are the hand-off page's error messages per reason, in
// the languages aio's login pages support. The page asks for the app's
// language with Accept-Language.
var authRequestErrors = map[string]map[string]string{
	"en": {
		"expired":       "this login link has expired; go back to the app and try again",
		"client":        "this app is no longer allowed to log in with all-in-one",
		"other_browser": "this login was started in a different browser; go back to the app and log in from this browser",
		"blocked":       "this account is blocked",
		"demo":          "the shared demo account can't be used to log in to other apps; use your own account",
		"internal":      "internal server error",
	},
	"id": {
		"expired":       "tautan masuk ini sudah kedaluwarsa; kembali ke aplikasi dan coba lagi",
		"client":        "aplikasi ini tidak lagi diizinkan masuk dengan all-in-one",
		"other_browser": "proses masuk ini dimulai di browser lain; kembali ke aplikasi dan masuk dari browser ini",
		"blocked":       "akun ini diblokir",
		"demo":          "akun demo bersama tidak bisa dipakai untuk masuk ke aplikasi lain; pakai akun kamu sendiri",
		"internal":      "terjadi kesalahan di server",
	},
}

// messageLanguage is "id" when the request prefers Indonesian, else "en".
func messageLanguage(r *http.Request) string {
	tags, _, err := language.ParseAcceptLanguage(r.Header.Get("Accept-Language"))
	if err != nil || len(tags) == 0 {
		return "en"
	}
	if _, i, c := messageLanguages.Match(tags...); c >= language.High && i == 1 {
		return "id"
	}
	return "en"
}

var messageLanguages = language.NewMatcher([]language.Tag{language.English, language.Indonesian})

func (h *Handler) sendError(w http.ResponseWriter, r *http.Request, err error) {
	reason, status := "internal", http.StatusInternalServerError
	switch {
	case errors.Is(err, oidc.ErrAuthRequestNotFound):
		reason, status = "expired", http.StatusNotFound
	case errors.Is(err, oidc.ErrClientNotFound), errors.Is(err, oidc.ErrClientRevoked):
		reason, status = "client", http.StatusNotFound
	case errors.Is(err, oidc.ErrOtherBrowser):
		reason, status = "other_browser", http.StatusForbidden
	case errors.Is(err, oidc.ErrUserBlocked):
		reason, status = "blocked", http.StatusForbidden
	case errors.Is(err, oidc.ErrDemoAccount):
		reason, status = "demo", http.StatusForbidden
	default:
		logging.GetLoggerFromContext(r.Context()).Error().Err(err).Msg("oidc: auth request failed")
	}
	h.metrics.loginFailed(r.Context(), reason)
	httpHelper.SendError(w, authRequestErrors[messageLanguage(r)][reason], status)
}
