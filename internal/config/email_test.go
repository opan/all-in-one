package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateEmail(t *testing.T) {
	valid := func() Config {
		return Config{
			Server: ServerConfig{PublicURL: "https://aio.example.com"},
			Email: EmailConfig{Enabled: true, From: "All-in-one <aio@example.com>",
				SMTP: SMTPConfig{Host: "smtp.gmail.com", Port: 587}},
		}
	}
	tests := []struct {
		name    string
		mutate  func(c *Config)
		wantErr string
	}{
		{"valid", func(c *Config) {}, ""},
		{"disabled needs nothing", func(c *Config) { *c = Config{} }, ""},
		{"bad from", func(c *Config) { c.Email.From = "nope" }, "email.from"},
		{"no host", func(c *Config) { c.Email.SMTP.Host = "" }, "email.smtp.host"},
		{"no public url", func(c *Config) { c.Server.PublicURL = "" }, "server.public_url"},
		{"issuer works as public url", func(c *Config) {
			c.Server.PublicURL = ""
			c.Auth.OIDC.Issuer = "https://aio.example.com"
		}, ""},
		{"bad support email", func(c *Config) { c.Auth.SupportEmail = "admin" }, "auth.support_email"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := valid()
			tt.mutate(&c)
			err := c.validateEmail()
			if tt.wantErr == "" {
				assert.NoError(t, err)
			} else {
				assert.ErrorContains(t, err, tt.wantErr)
			}
		})
	}
}

func TestPublicBaseURL(t *testing.T) {
	assert.Equal(t, "https://a.example.com", Config{Server: ServerConfig{PublicURL: "https://a.example.com/"}}.PublicBaseURL())
	assert.Equal(t, "https://b.example.com", Config{Auth: Auth{OIDC: OIDCConfig{Issuer: "https://b.example.com"}}}.PublicBaseURL())
}
