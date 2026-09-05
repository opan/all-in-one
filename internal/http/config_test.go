package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/all-in-one/internal/config"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
)

func TestPublicConfig_DemoMode(t *testing.T) {
	tests := []struct {
		name         string
		demo         config.DemoModeConfig
		wantEnabled  bool
		wantUsername string // "" means the field must be absent
		wantPassword string
	}{
		{
			name:         "enabled exposes credentials",
			demo:         config.DemoModeConfig{Enabled: true, Username: "demo", Password: "demo123"},
			wantEnabled:  true,
			wantUsername: "demo",
			wantPassword: "demo123",
		},
		{
			name:        "disabled hides credentials",
			demo:        config.DemoModeConfig{Enabled: false, Username: "demo", Password: "demo123"},
			wantEnabled: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewHTTP(zerolog.Nop(), config.Config{DemoMode: tt.demo})

			req := httptest.NewRequest(http.MethodGet, "/api/v1/config", nil)
			rr := httptest.NewRecorder()
			h.PublicConfig(rr, req)

			assert.Equal(t, http.StatusOK, rr.Code)

			var body struct {
				Success bool `json:"success"`
				Data    struct {
					DemoMode map[string]any `json:"demo_mode"`
				} `json:"data"`
			}
			assert.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
			assert.True(t, body.Success)
			assert.Equal(t, tt.wantEnabled, body.Data.DemoMode["enabled"])

			if tt.wantUsername == "" {
				_, hasUser := body.Data.DemoMode["username"]
				_, hasPass := body.Data.DemoMode["password"]
				assert.False(t, hasUser, "username must be omitted when demo mode is disabled")
				assert.False(t, hasPass, "password must be omitted when demo mode is disabled")
			} else {
				assert.Equal(t, tt.wantUsername, body.Data.DemoMode["username"])
				assert.Equal(t, tt.wantPassword, body.Data.DemoMode["password"])
			}
		})
	}
}
