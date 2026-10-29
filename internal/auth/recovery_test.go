package auth

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTempPassword(t *testing.T) {
	p, err := GenerateTempPassword()
	require.NoError(t, err)
	assert.Len(t, p, TempPasswordLength)
	assert.True(t, LooksLikeTempPassword(p))
	assert.NotContains(t, p, "0")
	assert.NotContains(t, p, "l")

	tests := []struct {
		in   string
		want bool
	}{
		{"abcd-efgh-jkmn", true},
		{"ABCD EFGH JKMN", true},
		{"abcdefghjkm", false},    // too short
		{"abcd-efgh-jkm0", false}, // 0 is not in the alphabet
		{"my old password", false},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, LooksLikeTempPassword(tt.in), tt.in)
	}
	assert.Equal(t, "abcdefghjkmn", NormalizeTempPassword("ABCD-efgh jkmn"))
}

func TestLinkToken(t *testing.T) {
	token, hash, err := GenerateLinkToken()
	require.NoError(t, err)
	assert.Len(t, token, 43)
	assert.Equal(t, hash, HashLinkToken(token))
	other, _, _ := GenerateLinkToken()
	assert.NotEqual(t, token, other)
}
