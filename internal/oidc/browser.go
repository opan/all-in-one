package oidc

import (
	"encoding/base64"
	"net/http"
)

// BrowserCookie ties an in-flight app login to the browser that started it
// (see service/binding.go). It lives here so the handler can read it without
// importing the service package.
const (
	BrowserCookie  = "aio_oidc_browser"
	BrowserIDBytes = 32
)

// BrowserIDFromRequest returns the browser cookie's value, or "" when it is
// missing or malformed.
func BrowserIDFromRequest(r *http.Request) string {
	c, err := r.Cookie(BrowserCookie)
	if err != nil {
		return ""
	}
	if b, err := base64.RawURLEncoding.DecodeString(c.Value); err != nil || len(b) != BrowserIDBytes {
		return ""
	}
	return c.Value
}
