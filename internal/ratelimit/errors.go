package ratelimit

import "errors"

// Sentinel errors shared between internal/ratelimit/service (which returns
// them) and internal/ratelimit/handler (which maps them to HTTP status
// codes). They live in this dependency-free top-level package — not in
// service.go — specifically so the handler package can check them via
// errors.Is without importing internal/ratelimit/service, which would
// create an import cycle (service already imports handler to construct it).
var (
	// ErrUnknownTarget is returned when an admin operation references a
	// target key not present in the Registry.
	ErrUnknownTarget = errors.New("unknown rate limit target")
	// ErrInvalidWindowUnit is returned when a rule's window_unit is not one
	// of second/minute/hour/day.
	ErrInvalidWindowUnit = errors.New("invalid window unit")

	// ErrTokenNotFound is returned when an app token is not found by id or hash
	// (a revoked token is treated as not found on the hot path).
	ErrTokenNotFound = errors.New("app token not found")
	// ErrTokenRevoked is returned when an operation references a token that has
	// been revoked.
	ErrTokenRevoked = errors.New("app token revoked")
	// ErrTokenScopeViolation is returned when a token is used against a target
	// key outside its declared scope prefix.
	ErrTokenScopeViolation = errors.New("app token scope violation")
	// ErrExternalTargetExists is returned when creating an external target
	// whose key is already taken (by a DB row or a Registry entry).
	ErrExternalTargetExists = errors.New("external target already exists")
	// ErrNotSupportedForExternal is returned when an operation only defined for
	// internal (Registry-backed) targets is attempted on an external one, e.g.
	// resetting to code-defined defaults.
	ErrNotSupportedForExternal = errors.New("operation not supported for external target")
)
