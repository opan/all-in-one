# ADR: User Authentication Mechanism

This document records design decisions made for the user authentication system (`internal/authnz`). Add a new entry here for any future feature or fix that touches auth.

---

## ADR-001: JWT + Session Table for Authentication

### Status
Accepted

### Decision
Use short-lived JWT access tokens (30 min) combined with a persistent `sessions` table. Each login creates a session row. The JWT `sub` claim carries the session ID.

### Rationale
- JWTs allow stateless signature validation on most requests.
- The session table provides a server-side anchor for invalidation (logout, password change, admin revoke).
- Refresh tokens (7-day TTL) are stored alongside session rows, enabling silent re-auth without re-login.

### Key files
- `internal/authnz/handler/session.go` — login, logout, refresh
- `internal/authnz/repository/sqlite/session_repository.go` — session CRUD
- `internal/authnz/middleware/jwt.go` — JWT validation middleware

---

## ADR-002: Session Invalidation on Password Change

### Status
Accepted — implemented on branch `fix/reset-password`

### Context
The app supports multiple concurrent sessions per user. When a user changes their password, all other active sessions must be invalidated immediately so that a compromised session is cut off as soon as the legitimate user resets the password.

### Decision
Two-part fix:

**Part 1 — Delete other sessions on password change** (`f29c148`)
After updating the password hash in the DB, call `SessionRepo().DeleteByUserIDExcept(ctx, userID, currentSessionID)` to remove all session rows except the one belonging to the browser that performed the reset.

**Part 2 — Validate session existence in JWT middleware**
`JWTMiddleware` previously only verified the token signature. Even after Part 1, the other browser's JWT remained cryptographically valid until its 30-minute expiry. Fix: inject `SessionRepository` into `JWTMiddleware` and call `sessionRepo.Get(ctx, sessionID)` in `validateJWT` after signature validation. If the session row is gone, return 401 immediately.

### Alternatives Considered

| Option | Description | Rejected because |
|--------|-------------|-----------------|
| **In-memory blocklist** | On deletion, add session IDs to a `map[uuid]time.Time` expiring at JWT TTL. Middleware checks the map (O(1), no DB). | Does not survive server restart; breaks with multiple server instances. |
| **`password_changed_at` + `iat` claim** | Store `password_changed_at` on user record; embed `iat` in JWT; reject if `iat < password_changed_at`. | Same DB-per-request cost; adds a migration and more moving parts. |
| **Shorter JWT TTL** | Reduce from 30 min to ~5 min; accept a short invalidation window. | Leaves a window where the invalidated session still works. |

### Consequences
- Every authenticated HTTP request now incurs one extra DB round-trip (primary-key lookup on `sessions`). Acceptable at current scale.
- If this becomes a bottleneck, the in-memory blocklist is the lowest-effort migration — the interface is unchanged.
- The `direct-auth` bypass (`tryDirectAuth`) returns before `validateJWT` is called and is unaffected.

### Files Changed
- `internal/authnz/middleware/jwt.go`
- `internal/authnz/handler/user.go`
- `internal/authnz/repository/sqlite/session_repository.go`
- `cmd/all-in-one/server/server.go`
