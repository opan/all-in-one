package ratelimit

import (
	"strings"

	"github.com/all-in-one/internal/ratelimit/model"
)

// Target keys — the canonical identifiers used throughout the rate limiting
// system (DB rows, registry lookups, admin API, frontend). Onboarding a new
// limited endpoint = add a constant + a Registry entry here, then ensure
// the owning subrouter carries the limiter middleware (ADR-003/ADR-004).
const (
	TargetAuthLogin            = "auth.login"
	TargetAuthSignupIP         = "auth.signup.ip"
	TargetListingItemCreate    = "listing.item.create"
	TargetListingTopicCreate   = "listing.topic.create"
	TargetChatSessionCreate    = "chat.session.create"
	TargetChatMessageSend      = "chat.message.send"
	TargetShortenerLinkCreate  = "shortener.link.create"
	TargetShortenerLinkResolve = "shortener.link.resolve"
)

// TargetDef is the code-defined source of truth for one rate-limited
// endpoint (ADR-003): its identity, the route it enforces on, its counting
// Scope (ADR-005), its counter Kind (ADR-002), and its seed defaults.
type TargetDef struct {
	Key         string
	Name        string
	Description string
	Scope       model.Scope
	Kind        model.Kind

	// Method and Path identify the route this target enforces on. Path must
	// match mux's GetPathTemplate() exactly, including the /api/v1 prefix
	// and any {var} placeholders (ADR-004) — boot-time validation (P14)
	// catches drift.
	Method string
	Path   string

	DefaultLimit       int
	DefaultWindowValue int
	DefaultWindowUnit  model.WindowUnit
}

// Registry is the single source of truth for rate-limited targets. On boot,
// one rate_limit_rules row is seeded per entry (insert-if-absent, never
// clobbering a prior admin edit — ADR-003).
var Registry = []TargetDef{
	{
		Key: TargetAuthLogin, Name: "Login attempts", Description: "Login attempts per IP",
		Scope: model.ScopeIP, Kind: model.KindThrottle,
		Method: "POST", Path: "/api/v1/sessions",
		DefaultLimit: 10, DefaultWindowValue: 1, DefaultWindowUnit: model.WindowMinute,
	},
	{
		Key: TargetAuthSignupIP, Name: "Sign-ups", Description: "Sign-ups per IP per day",
		Scope: model.ScopeIP, Kind: model.KindDailyQuota,
		Method: "POST", Path: "/api/v1/users",
		DefaultLimit: 20, DefaultWindowValue: 1, DefaultWindowUnit: model.WindowDay,
	},
	{
		Key: TargetListingItemCreate, Name: "Item creation", Description: "Items created per user per day",
		Scope: model.ScopeUser, Kind: model.KindDailyQuota,
		Method: "POST", Path: "/api/v1/topics/{topic_id}/items",
		DefaultLimit: 500, DefaultWindowValue: 1, DefaultWindowUnit: model.WindowDay,
	},
	{
		Key: TargetListingTopicCreate, Name: "Topic creation", Description: "Topics created per user per day",
		Scope: model.ScopeUser, Kind: model.KindDailyQuota,
		Method: "POST", Path: "/api/v1/topics",
		DefaultLimit: 100, DefaultWindowValue: 1, DefaultWindowUnit: model.WindowDay,
	},
	{
		Key: TargetChatSessionCreate, Name: "Chat creation", Description: "Chats created per user per day",
		Scope: model.ScopeUser, Kind: model.KindDailyQuota,
		Method: "POST", Path: "/api/v1/chats",
		DefaultLimit: 100, DefaultWindowValue: 1, DefaultWindowUnit: model.WindowDay,
	},
	{
		Key: TargetChatMessageSend, Name: "Chat messages", Description: "Messages sent per user per minute",
		Scope: model.ScopeUser, Kind: model.KindThrottle,
		Method: "POST", Path: "/api/v1/chats/{id}/messages",
		DefaultLimit: 60, DefaultWindowValue: 1, DefaultWindowUnit: model.WindowMinute,
	},
	{
		Key: TargetShortenerLinkCreate, Name: "Short link creation", Description: "Short links created per user",
		Scope: model.ScopeUser, Kind: model.KindThrottle,
		Method: "POST", Path: "/api/v1/shortener/links",
		DefaultLimit: 100, DefaultWindowValue: 15, DefaultWindowUnit: model.WindowMinute,
	},
	{
		// Public redirect — lives on the root router, outside /api/v1, so its
		// Path has no /api/v1 prefix (ADR-011; plan Appendix A #11).
		Key: TargetShortenerLinkResolve, Name: "Short link resolution", Description: "Short link redirects per IP",
		Scope: model.ScopeIP, Kind: model.KindThrottle,
		Method: "GET", Path: "/r/{code}",
		DefaultLimit: 300, DefaultWindowValue: 1, DefaultWindowUnit: model.WindowMinute,
	},
}

var (
	byKey          map[string]TargetDef
	byRouteBinding map[string][]TargetDef
)

func init() {
	buildIndex()
}

// buildIndex rebuilds the lookup maps from Registry. Split out from init()
// so tests can mutate Registry (e.g. to exercise a multi-target route) and
// rebuild the index without a process restart.
func buildIndex() {
	byKey = make(map[string]TargetDef, len(Registry))
	byRouteBinding = make(map[string][]TargetDef, len(Registry))
	for _, t := range Registry {
		byKey[t.Key] = t
		rk := routeBindingKey(t.Method, t.Path)
		byRouteBinding[rk] = append(byRouteBinding[rk], t)
	}
}

func routeBindingKey(method, path string) string {
	return strings.ToUpper(method) + " " + path
}

// ByKey looks up a target definition by its key. ok is false for an unknown
// key.
func ByKey(key string) (TargetDef, bool) {
	t, ok := byKey[key]
	return t, ok
}

// RouteBindings returns every target bound to the given method + path
// template (mux's GetPathTemplate() form — ADR-004). Usually zero or one
// entry; a route may carry more than one target (e.g. a throttle evaluated
// before a daily quota on the same endpoint).
func RouteBindings(method, pathTemplate string) []TargetDef {
	return byRouteBinding[routeBindingKey(method, pathTemplate)]
}

// Registered returns every target definition, in Registry order.
func Registered() []TargetDef {
	return Registry
}
