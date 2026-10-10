package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOIDCConfig_Validate(t *testing.T) {
	cases := []struct {
		name    string
		cfg     OIDCConfig
		wantErr bool
	}{
		{"disabled ignores issuer", OIDCConfig{Enabled: false}, false},
		{"https issuer", OIDCConfig{Enabled: true, Issuer: "https://auth.example.com"}, false},
		{"http issuer for local dev", OIDCConfig{Enabled: true, Issuer: "http://localhost:8080"}, false},
		{"trailing slash allowed", OIDCConfig{Enabled: true, Issuer: "https://auth.example.com/"}, false},
		{"missing issuer", OIDCConfig{Enabled: true}, true},
		{"relative issuer", OIDCConfig{Enabled: true, Issuer: "auth.example.com"}, true},
		{"non-http scheme", OIDCConfig{Enabled: true, Issuer: "ftp://auth.example.com"}, true},
		{"issuer with path", OIDCConfig{Enabled: true, Issuer: "https://example.com/aio"}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := c.cfg.validate()
			if c.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
