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
type Rule struct {
	TargetKey   string     `json:"target_key" db:"target_key"`
	Enabled     bool       `json:"enabled" db:"enabled"`
	LimitCount  int        `json:"limit_count" db:"limit_count"`
	WindowValue int        `json:"window_value" db:"window_value"`
	WindowUnit  WindowUnit `json:"window_unit" db:"window_unit"`
	UpdatedAt   time.Time  `json:"updated_at" db:"updated_at"`
	UpdatedBy   *string    `json:"updated_by,omitempty" db:"updated_by"`
}

// Counter is one bucket's current count for a daily_quota target (ADR-006).
type Counter struct {
	TargetKey string    `json:"target_key" db:"target_key"`
	BucketKey string    `json:"bucket_key" db:"bucket_key"`
	Day       string    `json:"day" db:"day"`
	Count     int       `json:"count" db:"count"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
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
}
