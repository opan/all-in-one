package handler

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewShortCode(t *testing.T) {
	for _, length := range []int{5, 7, 10} {
		code, err := newShortCode(length)
		require.NoError(t, err)
		assert.Len(t, code, length)
		for _, c := range code {
			assert.Contains(t, base62Chars, string(c), "character %q not in base62 alphabet", c)
		}
	}
}

func TestNewShortCode_Uniqueness(t *testing.T) {
	seen := make(map[string]struct{}, 100)
	for i := 0; i < 100; i++ {
		code, err := newShortCode(7)
		require.NoError(t, err)
		seen[code] = struct{}{}
	}
	// With 3.5T keyspace and 100 samples, collision probability is negligible
	assert.Greater(t, len(seen), 95, "too many collisions in random code generation")
}

func TestIsReserved(t *testing.T) {
	reserved := []string{"api", "r", "admin", "login", "swagger", "health", "API", "Admin"}
	for _, code := range reserved {
		assert.True(t, isReserved(code), "expected %q to be reserved", code)
	}

	notReserved := []string{"abc1234", "xYz9999", "hello"}
	for _, code := range notReserved {
		assert.False(t, isReserved(code), "expected %q to not be reserved", code)
	}
}

func TestNewULID(t *testing.T) {
	id1 := newULID()
	id2 := newULID()
	assert.NotEmpty(t, id1)
	assert.NotEqual(t, id1, id2)
	assert.Len(t, id1, 26)
	assert.True(t, strings.ToUpper(id1) == id1, "ULID should be uppercase")
}
