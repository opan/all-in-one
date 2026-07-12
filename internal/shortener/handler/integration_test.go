package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/all-in-one/internal/config"
	httpHelper "github.com/all-in-one/internal/http"
	"github.com/all-in-one/internal/shortener/model"
	"github.com/all-in-one/internal/shortener/repository"
	"github.com/all-in-one/internal/tester"
	"github.com/gorilla/mux"
	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newIntegDB(t *testing.T) *sqlx.DB {
	t.Helper()
	db, err := sqlx.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })

	_, err = db.Exec(`PRAGMA foreign_keys = ON`)
	require.NoError(t, err)

	_, err = db.Exec(`
		CREATE TABLE short_links (
			id               TEXT PRIMARY KEY,
			code             TEXT NOT NULL UNIQUE,
			target_url       TEXT NOT NULL,
			created_at       DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			expires_at       DATETIME,
			is_active        BOOLEAN NOT NULL DEFAULT 1,
			click_count      INTEGER NOT NULL DEFAULT 0,
			last_accessed_at DATETIME
		);
		CREATE TABLE short_link_owners (
			code    TEXT NOT NULL PRIMARY KEY REFERENCES short_links(code) ON DELETE CASCADE,
			user_id TEXT NOT NULL
		)
	`)
	require.NoError(t, err)
	return db
}

func newIntegHandler(t *testing.T) (*Handler, *sqlx.DB) {
	t.Helper()
	db := newIntegDB(t)
	storage := repository.NewStorageFromDB(db)
	cfg := config.Config{
		Shortener: config.ShortenerConfig{
			CodeLength:       7,
			MaxCreateRetries: 5,
			URL: config.ShortenerURLConfig{
				MaxLength:      2048,
				AllowedSchemes: []string{"http", "https"},
				BlockedHosts:   []string{},
			},
		},
	}
	return NewHandler(storage, cfg, zerolog.Nop()), db
}

func integRouter(h *Handler) *mux.Router {
	r := mux.NewRouter()
	api := r.PathPrefix("/api/v1").Subrouter()
	h.RegisterAuthenticatedRoutes(api)
	h.RegisterRedirectRoute(r)
	return r
}

func doRequest(t *testing.T, router *mux.Router, method, path string, body any, userID string) *httptest.ResponseRecorder {
	t.Helper()
	var bodyBytes []byte
	if body != nil {
		var err error
		bodyBytes, err = json.Marshal(body)
		require.NoError(t, err)
	}
	req := httptest.NewRequest(method, path, bytes.NewReader(bodyBytes))
	if userID != "" {
		req = req.WithContext(tester.ContextWithUser(userID, "user", "u@e.com", "sess"))
	}
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	return rr
}

func parseResponse(t *testing.T, rr *httptest.ResponseRecorder) httpHelper.Response {
	t.Helper()
	var resp httpHelper.Response
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	return resp
}

func createLinkInteg(t *testing.T, router *mux.Router, targetURL, userID string) string {
	t.Helper()
	rr := doRequest(t, router, http.MethodPost, "/api/v1/shortener/links",
		map[string]string{"target_url": targetURL}, userID)
	require.Equal(t, http.StatusCreated, rr.Code)
	resp := parseResponse(t, rr)
	return resp.Data.(map[string]any)["code"].(string)
}

// ---- Create → Resolve flow ----

func TestInteg_CreateAndResolve(t *testing.T) {
	h, _ := newIntegHandler(t)
	router := integRouter(h)

	code := createLinkInteg(t, router, "https://example.com/integration", "user-integ-1")
	assert.Len(t, code, 7)

	rr := doRequest(t, router, http.MethodGet, "/r/"+code, nil, "")
	assert.Equal(t, http.StatusFound, rr.Code)
	assert.Equal(t, "https://example.com/integration", rr.Header().Get("Location"))
	assert.Equal(t, "private, no-store", rr.Header().Get("Cache-Control"))
}

func TestInteg_ClickCountIncrements(t *testing.T) {
	h, db := newIntegHandler(t)
	router := integRouter(h)

	code := createLinkInteg(t, router, "https://example.com/click", "user-click-1")

	for range 3 {
		rr := doRequest(t, router, http.MethodGet, "/r/"+code, nil, "")
		assert.Equal(t, http.StatusFound, rr.Code)
	}

	var count uint64
	err := db.Get(&count, `SELECT click_count FROM short_links WHERE code = ?`, code)
	require.NoError(t, err)
	assert.Equal(t, uint64(3), count)
}

func TestInteg_ResolveDisabledLink(t *testing.T) {
	h, _ := newIntegHandler(t)
	router := integRouter(h)
	userID := "user-dis-1"

	code := createLinkInteg(t, router, "https://example.com/dis", userID)

	rr := doRequest(t, router, http.MethodPatch, "/api/v1/shortener/links/"+code,
		map[string]any{"is_active": false}, userID)
	assert.Equal(t, http.StatusOK, rr.Code)

	rr2 := doRequest(t, router, http.MethodGet, "/r/"+code, nil, "")
	assert.Equal(t, http.StatusNotFound, rr2.Code)
}

func TestInteg_ResolveExpiredLink(t *testing.T) {
	h, db := newIntegHandler(t)
	router := integRouter(h)

	past := time.Now().Add(-time.Hour).UTC()
	link := model.ShortLink{
		ID:        newULID(),
		Code:      "exp1234",
		TargetURL: "https://example.com/exp",
		CreatedAt: time.Now().UTC(),
		ExpiresAt: &past,
		IsActive:  true,
	}
	_, err := db.Exec(`
		INSERT INTO short_links (id, code, target_url, created_at, expires_at, is_active, click_count)
		VALUES (?, ?, ?, ?, ?, ?, 0)
	`, link.ID, link.Code, link.TargetURL, link.CreatedAt, link.ExpiresAt, link.IsActive)
	require.NoError(t, err)

	rr := doRequest(t, router, http.MethodGet, "/r/exp1234", nil, "")
	assert.Equal(t, http.StatusGone, rr.Code)
}

func TestInteg_ResolveNonexistentCode(t *testing.T) {
	h, _ := newIntegHandler(t)
	router := integRouter(h)

	rr := doRequest(t, router, http.MethodGet, "/r/xxxxxxx", nil, "")
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestInteg_DeleteLink(t *testing.T) {
	h, _ := newIntegHandler(t)
	router := integRouter(h)
	userID := "user-del-1"

	code := createLinkInteg(t, router, "https://example.com/del", userID)

	rr := doRequest(t, router, http.MethodDelete, "/api/v1/shortener/links/"+code, nil, userID)
	assert.Equal(t, http.StatusOK, rr.Code)

	rr2 := doRequest(t, router, http.MethodGet, "/r/"+code, nil, "")
	assert.Equal(t, http.StatusNotFound, rr2.Code)
}

func TestInteg_CreateRejectsCustomCode(t *testing.T) {
	h, _ := newIntegHandler(t)
	router := integRouter(h)

	rr := doRequest(t, router, http.MethodPost, "/api/v1/shortener/links",
		map[string]any{"target_url": "https://example.com", "custom_code": "mycustom"}, "user-m2-1")
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestInteg_CreateRejectsUnsafeURL(t *testing.T) {
	h, _ := newIntegHandler(t)
	router := integRouter(h)

	cases := []string{
		"javascript:alert(1)",
		"http://192.168.1.1/steal",
		"ftp://files.example.com",
	}
	for _, u := range cases {
		rr := doRequest(t, router, http.MethodPost, "/api/v1/shortener/links",
			map[string]string{"target_url": u}, "user-safe-1")
		assert.Equal(t, http.StatusBadRequest, rr.Code, "expected 400 for %s", u)
	}
}

func TestInteg_ListAndGetLinks(t *testing.T) {
	h, _ := newIntegHandler(t)
	router := integRouter(h)
	userID := "user-list-1"

	for _, u := range []string{"https://a.com", "https://b.com"} {
		createLinkInteg(t, router, u, userID)
	}

	rr := doRequest(t, router, http.MethodGet, "/api/v1/shortener/links", nil, userID)
	require.Equal(t, http.StatusOK, rr.Code)
	resp := parseResponse(t, rr)
	listData := resp.Data.(map[string]any)
	assert.Equal(t, float64(2), listData["total"])
	links := listData["links"].([]any)
	assert.Len(t, links, 2)

	firstCode := links[0].(map[string]any)["code"].(string)
	rr2 := doRequest(t, router, http.MethodGet, "/api/v1/shortener/links/"+firstCode, nil, userID)
	assert.Equal(t, http.StatusOK, rr2.Code)
}

func TestInteg_GetLinkOwnershipMismatch(t *testing.T) {
	h, _ := newIntegHandler(t)
	router := integRouter(h)

	code := createLinkInteg(t, router, "https://a.com", "user-a")

	rr := doRequest(t, router, http.MethodGet, "/api/v1/shortener/links/"+code, nil, "user-b")
	assert.Equal(t, http.StatusNotFound, rr.Code)
}
