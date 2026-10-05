package service

import (
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/zitadel/oidc/v3/pkg/oidc"
)

// authRequest is one in-flight login: created when an app redirects a user to
// the authorize endpoint, completed when the user finishes aio's login page.
type authRequest struct {
	ID            string
	ClientID      string
	RedirectURI   string
	State         string
	Nonce         string
	Scopes        []string
	Prompt        []string
	ResponseType  oidc.ResponseType
	ResponseMode  oidc.ResponseMode
	CodeChallenge *oidc.CodeChallenge
	BrowserHash   string
	CreatedAt     time.Time

	UserID   string
	AuthTime time.Time
	done     bool
}

func (a *authRequest) GetID() string                         { return a.ID }
func (a *authRequest) GetACR() string                        { return "" }
func (a *authRequest) GetAudience() []string                 { return []string{a.ClientID} }
func (a *authRequest) GetAuthTime() time.Time                { return a.AuthTime }
func (a *authRequest) GetClientID() string                   { return a.ClientID }
func (a *authRequest) GetCodeChallenge() *oidc.CodeChallenge { return a.CodeChallenge }
func (a *authRequest) GetNonce() string                      { return a.Nonce }
func (a *authRequest) GetRedirectURI() string                { return a.RedirectURI }
func (a *authRequest) GetResponseType() oidc.ResponseType    { return a.ResponseType }
func (a *authRequest) GetResponseMode() oidc.ResponseMode    { return a.ResponseMode }
func (a *authRequest) GetScopes() []string                   { return a.Scopes }
func (a *authRequest) GetState() string                      { return a.State }
func (a *authRequest) GetSubject() string                    { return a.UserID }
func (a *authRequest) Done() bool                            { return a.done }

func (a *authRequest) GetAMR() []string {
	if a.done {
		return []string{"pwd"}
	}
	return nil
}

func (a *authRequest) wantsSignup() bool {
	for _, p := range a.Prompt {
		if p == "create" {
			return true
		}
	}
	return false
}

type accessToken struct {
	ID        string
	ClientID  string
	Subject   string
	Audience  []string
	Scopes    []string
	ExpiresAt time.Time
}

// memStore keeps auth requests, codes and access tokens in memory. They are
// all short-lived: a restart mid-login only means the user starts the login
// again, and apps only use the ID token once, at login. Expired entries are
// swept whenever a new one is created.
type memStore struct {
	mu       sync.Mutex
	requests map[string]*authRequest
	codes    map[string]string // code -> auth request id
	tokens   map[string]*accessToken
	reqTTL   time.Duration
	tokenTTL time.Duration
	now      func() time.Time
}

func newMemStore(reqTTL, tokenTTL time.Duration) *memStore {
	return &memStore{
		requests: map[string]*authRequest{},
		codes:    map[string]string{},
		tokens:   map[string]*accessToken{},
		reqTTL:   reqTTL,
		tokenTTL: tokenTTL,
		now:      time.Now,
	}
}

func (m *memStore) sweepLocked() {
	now := m.now()
	for id, r := range m.requests {
		if now.Sub(r.CreatedAt) > m.reqTTL {
			delete(m.requests, id)
		}
	}
	for code, id := range m.codes {
		if _, ok := m.requests[id]; !ok {
			delete(m.codes, code)
		}
	}
	for id, t := range m.tokens {
		if now.After(t.ExpiresAt) {
			delete(m.tokens, id)
		}
	}
}

func (m *memStore) addRequest(r *authRequest) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sweepLocked()
	r.ID = uuid.NewString()
	r.CreatedAt = m.now()
	m.requests[r.ID] = r
}

// request returns a copy so callers can't mutate shared state without the lock.
func (m *memStore) request(id string) (*authRequest, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, ok := m.requests[id]
	if !ok || m.now().Sub(r.CreatedAt) > m.reqTTL {
		return nil, false
	}
	cp := *r
	return &cp, true
}

// complete attaches the user to a request. A request that is already done
// can't be handed to a different user.
func (m *memStore) complete(id, userID string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, ok := m.requests[id]
	if !ok || m.now().Sub(r.CreatedAt) > m.reqTTL || (r.done && r.UserID != userID) {
		return false
	}
	r.UserID, r.AuthTime, r.done = userID, m.now(), true
	return true
}

func (m *memStore) saveCode(id, code string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.requests[id]; !ok {
		return false
	}
	m.codes[code] = id
	return true
}

// requestByCode uses the code up as it looks it up: the library only deletes
// the request after issuing tokens, so two simultaneous token requests with
// the same code would otherwise both succeed. A failed exchange burns the
// code too, which is what RFC 6749 asks for anyway.
func (m *memStore) requestByCode(code string) (*authRequest, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	id, ok := m.codes[code]
	if !ok {
		return nil, false
	}
	delete(m.codes, code)
	r, ok := m.requests[id]
	if !ok || m.now().Sub(r.CreatedAt) > m.reqTTL {
		return nil, false
	}
	cp := *r
	return &cp, true
}

// deleteRequest removes a request and every code pointing at it, so no
// further code can be issued or redeemed for it after an exchange.
func (m *memStore) deleteRequest(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.requests, id)
	for code, rid := range m.codes {
		if rid == id {
			delete(m.codes, code)
		}
	}
}

func (m *memStore) addToken(t *accessToken) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sweepLocked()
	t.ID = uuid.NewString()
	t.ExpiresAt = m.now().Add(m.tokenTTL)
	m.tokens[t.ID] = t
}

func (m *memStore) token(id string) (*accessToken, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	t, ok := m.tokens[id]
	if !ok || m.now().After(t.ExpiresAt) {
		return nil, false
	}
	cp := *t
	return &cp, true
}

func (m *memStore) deleteToken(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.tokens, id)
}

func (m *memStore) deleteTokensFor(userID, clientID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, t := range m.tokens {
		if t.Subject == userID && (clientID == "" || t.ClientID == clientID) {
			delete(m.tokens, id)
		}
	}
}
