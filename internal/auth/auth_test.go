package auth

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidatePassword(t *testing.T) {
	cases := []struct {
		name     string
		password string
		want     error
	}{
		{"4 characters", "abcd", ErrPasswordTooShort},
		{"5 characters", "abcde", nil},
		{"5 multi-byte characters count as 5", "ééééé", nil},
		{"4 multi-byte characters", "éééé", ErrPasswordTooShort},
		{"72 bytes", strings.Repeat("x", MaxPasswordBytes), nil},
		{"73 bytes (bcrypt's limit)", strings.Repeat("x", MaxPasswordBytes+1), ErrPasswordTooLong},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.ErrorIs(t, ValidatePassword(c.password), c.want)
		})
	}
}
