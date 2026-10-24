package service

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSameOriginAsAny(t *testing.T) {
	registered := []string{"https://kas.example.com/auth/callback", "https://kas.example.com/"}
	tests := []struct {
		returnTo string
		want     bool
	}{
		{"https://kas.example.com/", true},
		{"https://kas.example.com/settings?tab=1", true},
		{"https://KAS.example.com/x", true},
		{"http://kas.example.com/", false},       // scheme differs
		{"https://kas.example.com:8443/", false}, // port differs
		{"https://evil.example.com/", false},
		{"https://kas.example.com@evil.com/", false},
		{"//kas.example.com/", false},
		{"javascript:alert(1)", false},
		{"", false},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, sameOriginAsAny(tt.returnTo, registered), tt.returnTo)
	}
}
