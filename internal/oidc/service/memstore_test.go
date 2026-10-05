package service

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMemStore_Expiry(t *testing.T) {
	now := time.Now()
	m := newMemStore(time.Minute, 5*time.Minute)
	m.now = func() time.Time { return now }

	r := &authRequest{ClientID: "cashflow"}
	m.addRequest(r)
	require.True(t, m.saveCode(r.ID, "code-1"))
	tok := &accessToken{Subject: "u1", ClientID: "cashflow"}
	m.addToken(tok)

	require.True(t, m.saveCode(r.ID, "code-2"))

	_, ok := m.requestByCode("code-1")
	assert.True(t, ok)

	now = now.Add(2 * time.Minute)
	_, ok = m.request(r.ID)
	assert.False(t, ok, "auth requests expire after their TTL")
	_, ok = m.requestByCode("code-2")
	assert.False(t, ok, "so do the codes issued for them")
	_, ok = m.token(tok.ID)
	assert.True(t, ok, "access tokens have their own, longer lifetime")

	now = now.Add(5 * time.Minute)
	_, ok = m.token(tok.ID)
	assert.False(t, ok)
}

func TestMemStore_DeleteRequestBurnsItsCodes(t *testing.T) {
	m := newMemStore(time.Minute, time.Minute)
	r := &authRequest{}
	m.addRequest(r)
	m.saveCode(r.ID, "code-1")
	m.deleteRequest(r.ID)
	_, ok := m.requestByCode("code-1")
	assert.False(t, ok)
}

func TestMemStore_DeleteTokensFor(t *testing.T) {
	m := newMemStore(time.Minute, time.Minute)
	a := &accessToken{Subject: "u1", ClientID: "cashflow"}
	b := &accessToken{Subject: "u1", ClientID: "other"}
	c := &accessToken{Subject: "u2", ClientID: "cashflow"}
	for _, tk := range []*accessToken{a, b, c} {
		m.addToken(tk)
	}
	m.deleteTokensFor("u1", "")
	_, okA := m.token(a.ID)
	_, okB := m.token(b.ID)
	_, okC := m.token(c.ID)
	assert.False(t, okA)
	assert.False(t, okB)
	assert.True(t, okC, "other users' tokens are untouched")
}

func TestMemStore_CodeIsUsedUpOnLookup(t *testing.T) {
	m := newMemStore(time.Minute, time.Minute)
	r := &authRequest{}
	m.addRequest(r)
	require.True(t, m.saveCode(r.ID, "code-1"))

	var wins atomic.Int32
	var wg sync.WaitGroup
	for range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, ok := m.requestByCode("code-1"); ok {
				wins.Add(1)
			}
		}()
	}
	wg.Wait()
	assert.Equal(t, int32(1), wins.Load(), "simultaneous token requests can't both redeem one code")
}

func TestMemStore_CompletedRequestKeepsItsUser(t *testing.T) {
	m := newMemStore(time.Minute, time.Minute)
	r := &authRequest{}
	m.addRequest(r)

	require.True(t, m.complete(r.ID, "u1"))
	assert.True(t, m.complete(r.ID, "u1"), "the same user may finish it again (page reload)")
	assert.False(t, m.complete(r.ID, "u2"), "another user can't take over a finished request")
	got, _ := m.request(r.ID)
	assert.Equal(t, "u1", got.UserID)
}
