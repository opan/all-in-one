package rbac

import "errors"

// Sentinel errors shared between internal/rbac/service (which returns them)
// and internal/rbac/handler (which maps them to HTTP status codes). They
// live in this dependency-free top-level package — not in service.go —
// specifically so the handler package can check them via errors.Is without
// importing internal/rbac/service, which would create an import cycle
// (service already imports handler to construct it).
var (
	// ErrLastAdmin is returned when an operation would leave the admin group
	// with zero members. See docs/adr/ACCESS_MANAGEMENT_ADR.md ADR-004.
	ErrLastAdmin = errors.New("cannot remove the last admin")
	// ErrBuiltinGroup is returned when an operation would delete or rename a
	// built-in group, or edit the admin group's feature grants (a no-op,
	// since admin bypasses all feature gates — see ADR-002/ADR-004).
	ErrBuiltinGroup = errors.New("built-in groups cannot be modified this way")
)
