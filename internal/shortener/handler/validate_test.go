package handler

import (
	"testing"

	"github.com/all-in-one/internal/config"
	"github.com/stretchr/testify/assert"
)

func defaultURLCfg() config.ShortenerURLConfig {
	return config.ShortenerURLConfig{
		MaxLength:      2048,
		AllowedSchemes: []string{"http", "https"},
		BlockedHosts:   []string{},
	}
}

func TestValidateURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		cfg     config.ShortenerURLConfig
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid http URL",
			url:     "http://example.com/path",
			cfg:     defaultURLCfg(),
			wantErr: false,
		},
		{
			name:    "valid https URL",
			url:     "https://example.com",
			cfg:     defaultURLCfg(),
			wantErr: false,
		},
		{
			name:    "empty URL",
			url:     "",
			cfg:     defaultURLCfg(),
			wantErr: true,
			errMsg:  "target_url is required",
		},
		{
			name:    "URL exceeds max length",
			url:     "https://example.com/" + string(make([]byte, 2048)),
			cfg:     defaultURLCfg(),
			wantErr: true,
			errMsg:  "exceeds maximum length",
		},
		{
			name:    "javascript scheme rejected",
			url:     "javascript:alert(1)",
			cfg:     defaultURLCfg(),
			wantErr: true,
			errMsg:  "not allowed",
		},
		{
			name:    "ftp scheme rejected",
			url:     "ftp://files.example.com",
			cfg:     defaultURLCfg(),
			wantErr: true,
			errMsg:  "not allowed",
		},
		{
			name:    "data scheme rejected",
			url:     "data:text/html,<h1>hello</h1>",
			cfg:     defaultURLCfg(),
			wantErr: true,
		},
		{
			name:    "no scheme",
			url:     "example.com/path",
			cfg:     defaultURLCfg(),
			wantErr: true,
		},
		{
			name:    "loopback IP rejected",
			url:     "http://127.0.0.1/secret",
			cfg:     defaultURLCfg(),
			wantErr: true,
			errMsg:  "private or loopback",
		},
		{
			name:    "RFC1918 10.x rejected",
			url:     "http://10.0.0.1/internal",
			cfg:     defaultURLCfg(),
			wantErr: true,
			errMsg:  "private or loopback",
		},
		{
			name:    "RFC1918 192.168.x rejected",
			url:     "http://192.168.1.1/router",
			cfg:     defaultURLCfg(),
			wantErr: true,
			errMsg:  "private or loopback",
		},
		{
			name:    "RFC1918 172.16.x rejected",
			url:     "http://172.16.0.1/internal",
			cfg:     defaultURLCfg(),
			wantErr: true,
			errMsg:  "private or loopback",
		},
		{
			name:    "link-local IP rejected",
			url:     "http://169.254.169.254/metadata",
			cfg:     defaultURLCfg(),
			wantErr: true,
			errMsg:  "private or loopback",
		},
		{
			name:    "blocked host rejected",
			url:     "https://blocked.example.com/page",
			cfg:     config.ShortenerURLConfig{MaxLength: 2048, AllowedSchemes: []string{"https"}, BlockedHosts: []string{"blocked.example.com"}},
			wantErr: true,
			errMsg:  "not allowed",
		},
		{
			name:    "public IP allowed",
			url:     "http://8.8.8.8/test",
			cfg:     defaultURLCfg(),
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateURL(tt.url, tt.cfg)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
