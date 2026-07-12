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
)
