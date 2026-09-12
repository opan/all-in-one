package model

import "time"

// Scope determines the counting key a rule uses (ADR-005): per client IP,
// per authenticated user, or one shared global counter.
type Scope string

const (
	ScopeIP     Scope = "ip"
	ScopeUser   Scope = "user"
	ScopeGlobal Scope = "global"
)

// Kind determines which counter backend enforces a rule (ADR-002): a fast
// in-memory throttle, or a DB-backed daily quota that survives restarts.
type Kind string

const (
	KindThrottle   Kind = "throttle"
	KindDailyQuota Kind = "daily_quota"
)

// WindowUnit is the unit a Rule's WindowValue is expressed in.
type WindowUnit string

const (
	WindowSecond WindowUnit = "second"
	WindowMinute WindowUnit = "minute"
	WindowHour   WindowUnit = "hour"
	WindowDay    WindowUnit = "day"
)

// Rule is the DB-backed, admin-editable config for one target (ADR-001).
// Column names deliberately avoid the SQL reserved words limit/window.
//
// The App/IsExternal/Name/Description/Scope/Kind columns arrived with
// migration 10 (external rate limiting). For internal rows the last four are
// NULL — the Registry remains the source of truth for their Scope/Kind — so
// they are pointers. For external rows they carry the row's own identity,
// since an external target has no Registry entry to merge from.
type Rule struct {
	TargetKey   string     `json:"target_key" db:"target_key"`
	Enabled     bool       `json:"enabled" db:"enabled"`
	LimitCount  int        `json:"limit_count" db:"limit_count"`
	WindowValue int        `json:"window_value" db:"window_value"`
	WindowUnit  WindowUnit `json:"window_unit" db:"window_unit"`
	UpdatedAt   time.Time  `json:"updated_at" db:"updated_at"`
	UpdatedBy   *string    `json:"updated_by,omitempty" db:"updated_by"`
	App         string     `json:"app" db:"app"`
	IsExternal  bool       `json:"is_external" db:"is_external"`
	Name        *string    `json:"name,omitempty" db:"name"`
	Description *string    `json:"description,omitempty" db:"description"`
	Scope       *Scope     `json:"scope,omitempty" db:"scope"`
	Kind        *Kind      `json:"kind,omitempty" db:"kind"`
}

// AppToken is a service credential that lets another app (e.g. cashflow) call
// aio's external rate-limit check API. Admin-issued, prefix-scoped. The token
// hash is SHA-256, never bcrypt (see EXTERNAL_RATE_LIMIT plan, ATTENTION #1):
// verification runs on the caller's request hot path, and the token is
// high-entropy so a slow hash buys nothing. TokenHash is never serialized.
type AppToken struct {
	ID          string     `json:"id" db:"id"`
	App         string     `json:"app" db:"app"`
	Name        string     `json:"name" db:"name"`
	TokenHash   string     `json:"-" db:"token_hash"`
	TokenPrefix string     `json:"token_prefix" db:"token_prefix"`
	ScopePrefix string     `json:"scope_prefix" db:"scope_prefix"`
	CreatedAt   time.Time  `json:"created_at" db:"created_at"`
	CreatedBy   *string    `json:"created_by,omitempty" db:"created_by"`
	LastUsedAt  *time.Time `json:"last_used_at,omitempty" db:"last_used_at"`
	RevokedAt   *time.Time `json:"revoked_at,omitempty" db:"revoked_at"`
}

// CheckRequest is the body of POST /api/v1/ratelimit/check. The caller supplies
// the bucket key itself (unlike internal targets, where aio derives it from the
// request's JWT/IP), because aio has no visibility into the caller's session.
type CheckRequest struct {
	TargetKey string `json:"target_key"`
	BucketKey string `json:"bucket_key"`
}

// CheckResponse is the decision returned to an external caller. Limit/Remaining
// cost nothing to include and make consumer-side debugging far easier. For
// throttle-kind targets Remaining is best-effort — the in-memory store tracks a
// count, not a reservation, so it is a snapshot rather than a guarantee.
type CheckResponse struct {
	Allowed           bool `json:"allowed"`
	Limit             int  `json:"limit"`
	Remaining         int  `json:"remaining"`
	RetryAfterSeconds int  `json:"retry_after_seconds"`
}

// Counter is one bucket's current count for a daily_quota target (ADR-006).
type Counter struct {
	TargetKey string    `json:"target_key" db:"target_key"`
	BucketKey string    `json:"bucket_key" db:"bucket_key"`
	Day       string    `json:"day" db:"day"`
	Count     int       `json:"count" db:"count"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// EffectiveRule is the in-memory-cached merge of a target's code-defined
// identity (Scope/Kind, from the Registry) and its DB-backed tunables
// (Enabled/LimitCount/Window), with WindowUnit already resolved to a
// time.Duration. It is what the limiter middleware reads per request
// (docs/adr/RATE_LIMITING_ADR.md ADR-008) — declared here, not in the
// service or middleware package, so both can reference the same type
// without an import cycle.
type EffectiveRule struct {
	Key        string
	Scope      Scope
	Kind       Kind
	Enabled    bool
	LimitCount int
	Window     time.Duration
}

// Target is the admin API's read view of one registry target merged with
// its effective (DB-overlaid) rule: code metadata plus current config.
type Target struct {
	Key         string     `json:"key"`
	Name        string     `json:"name"`
	Description string     `json:"description,omitempty"`
	Scope       Scope      `json:"scope"`
	Kind        Kind       `json:"kind"`
	Method      string     `json:"method"`
	Path        string     `json:"path"`
	Enabled     bool       `json:"enabled"`
	LimitCount  int        `json:"limit_count"`
	WindowValue int        `json:"window_value"`
	WindowUnit  WindowUnit `json:"window_unit"`
	UpdatedAt   *time.Time `json:"updated_at,omitempty"`
	UpdatedBy   *string    `json:"updated_by,omitempty"`
	App         string     `json:"app"`
	IsExternal  bool       `json:"is_external"`
}

// TargetPatch is a partial edit to a target's rule. Pointer fields let the
// admin write API (P11) distinguish "omitted" from "set to zero" — nil
// means leave that field unchanged.
type TargetPatch struct {
	Enabled     *bool       `json:"enabled,omitempty"`
	LimitCount  *int        `json:"limit_count,omitempty"`
	WindowValue *int        `json:"window_value,omitempty"`
	WindowUnit  *WindowUnit `json:"window_unit,omitempty"`
}
