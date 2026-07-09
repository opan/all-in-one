package ratelimit

import (
	"testing"

	"github.com/all-in-one/internal/ratelimit/model"
	"github.com/stretchr/testify/assert"
)

func TestByKey(t *testing.T) {
	t.Run("known key", func(t *testing.T) {
		got, ok := ByKey(TargetAuthLogin)
		assert.True(t, ok)
		assert.Equal(t, TargetAuthLogin, got.Key)
		assert.Equal(t, model.ScopeIP, got.Scope)
		assert.Equal(t, model.KindThrottle, got.Kind)
	})

	t.Run("unknown key", func(t *testing.T) {
		_, ok := ByKey("does.not.exist")
		assert.False(t, ok)
	})
}

func TestRouteBindings(t *testing.T) {
	t.Run("no binding for unregistered route", func(t *testing.T) {
		got := RouteBindings("POST", "/api/v1/does-not-exist")
		assert.Empty(t, got)
	})

	t.Run("route with a path variable", func(t *testing.T) {
		got := RouteBindings("POST", "/api/v1/topics/{topic_id}/items")
		assert.Len(t, got, 1)
		assert.Equal(t, TargetListingItemCreate, got[0].Key)
	})

	t.Run("method must match", func(t *testing.T) {
		got := RouteBindings("GET", "/api/v1/topics/{topic_id}/items")
		assert.Empty(t, got)
	})

	t.Run("multiple targets on the same route", func(t *testing.T) {
		orig := Registry
		t.Cleanup(func() {
			Registry = orig
			buildIndex()
		})

		Registry = append(append([]TargetDef{}, orig...), TargetDef{
			Key: "test.extra.throttle", Name: "Extra throttle",
			Scope: model.ScopeUser, Kind: model.KindThrottle,
			Method: "POST", Path: "/api/v1/chats/{id}/messages",
			DefaultLimit: 5, DefaultWindowValue: 1, DefaultWindowUnit: model.WindowSecond,
		})
		buildIndex()

		got := RouteBindings("POST", "/api/v1/chats/{id}/messages")
		assert.Len(t, got, 2)
		keys := []string{got[0].Key, got[1].Key}
		assert.Contains(t, keys, TargetChatMessageSend)
		assert.Contains(t, keys, "test.extra.throttle")
	})
}

func TestRegistered(t *testing.T) {
	got := Registered()
	assert.Len(t, got, len(Registry))
	assert.Equal(t, Registry, got)
}
