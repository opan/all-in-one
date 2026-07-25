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

---

## ADR-003: Self-Service Sign-Up + Rate Limiting

### Status
Accepted

### Context
Users could not create their own accounts — only admins could create users (via the admin API) or a
seed script. The idea requires: (1) self-service sign-up with just username + password (email optional,
non-editable for now), (2) the account usable immediately (no email-confirmation gate, since SMTP isn't
configured yet), (3) a password-confirmation field, and (4) the sign-up endpoint sitting behind a rate
limiter.

Investigating `internal/authnz/handler/user.go` before starting found that `RegisterUser` (`POST /users`)
already implemented (1) and (2) in full — it was built for admin/seed use but has no admin check, takes
optional `email`/`name`, hashes the password, and activates the account with no confirmation step. It was
just never exposed through the frontend (no `/register` page existed). So this ADR is scoped to what was
actually missing: the frontend page, rate limiting, and the post-signup UX.

**Revised after rebasing onto `main`:** this ADR's first draft assumed no generic rate-limit app-feature
existed and built a standalone `RateLimiter` for the sign-up endpoint (extracted from
`internal/shortener/middleware`). While this branch was in progress, `internal/ratelimit` (the real
app-feature — DB-backed rules/counters, a target registry, admin UI) merged to `main`, and it already
pre-registered `POST /api/v1/users` as target `auth.signup.ip` with an explicit TODO for this exact branch
(see `internal/ratelimit/registry.go` and `.context/RATE_LIMITING_PROGRESS.md`). Decision point 3 below
reflects the rebased, corrected approach — the standalone `RateLimiter` was removed entirely.

### Decision
**1. Reuse `POST /users` as-is as the public register endpoint** rather than building a new one.
Password confirmation is handled client-side only (two input fields, equality checked before submit) —
the backend only ever needs the single resulting password, same as every other password-setting flow in
this codebase (e.g. `ResetPasswordUser`).

**2. Auto-login after sign-up.** On a successful `POST /users`, the frontend immediately calls
`POST /sessions` with the same credentials and redirects to `/`, so the account is usable with zero extra
friction — matching requirement (2) directly instead of bouncing the user to `/login` to re-enter what
they just typed.

**3. Rate limiting via the real `internal/ratelimit` app-feature — no code needed in `authnz` at all.**
`POST /api/v1/users` was already registered as target `auth.signup.ip` (a per-IP `daily_quota`, 20/day
default) before this branch existed, and `RegisterPublicRoutes` mounts onto the `publicRoutes` subrouter
in `server.go`, which already carries the limiter middleware (`rlMw`). So the daily quota was enforced
the moment this branch rebased onto `main` — zero lines changed in `internal/authnz`. The one real gap,
called out explicitly by a comment left at the `auth.signup.ip` registry entry, was that a public/
unauthenticated `daily_quota` target with no `throttle` sharing its route hits the DB counter on *every*
request, including floods — the other three `daily_quota` targets sit behind `JWTAuth`, which sheds
floods before the counter is touched, but sign-up has no such gate. Fixed by adding
`TargetAuthSignupThrottleIP` (`auth.signup.throttle.ip`, per-IP `throttle`, 5/minute default) to
`internal/ratelimit/registry.go`, bound to the same route — `orderThrottlesFirst` evaluates it before the
daily quota automatically, no other wiring required. Verified with a new middleware test asserting the
throttled request never reaches the counter store.

**4. No new password-strength policy.** The only existing password-length check in the codebase is the
settings page's client-side `length < 3` on password reset. The sign-up form matches that convention
rather than introducing a stricter, inconsistent policy as a side effect of this feature.

### Alternatives Considered
| Option | Rejected because |
|---|---|
| Build a dedicated `/register` endpoint separate from the admin-facing create-user path | `RegisterUser` already has exactly the right shape (optional email/name, no admin gate, immediate activation); a second endpoint would duplicate logic for no behavioral difference. |
| Standalone `RateLimiter` extracted from shortener (first draft of this ADR) | Superseded once `internal/ratelimit` merged to `main` mid-branch — keeping a second, parallel rate-limiting mechanism alongside the real one would be pure duplication, and the registry already had a TODO anticipating exactly this integration. Rebased onto the real system instead (decision 3). |
| Redirect to `/login` after sign-up instead of auto-login | Simpler, but fails requirement (2) more literally — the user would have to retype credentials they just entered. Confirmed auto-login with the user before implementing. |
| Add a new password-strength policy (min length, complexity) | Not requested by the idea, and would be inconsistent with the only existing precedent (reset-password's length-≥3 check) unless that's revisited too — out of scope here. |

### Consequences
- `internal/authnz` has zero rate-limiting code — enforcement is entirely declarative via the
  `auth.signup.ip` + `auth.signup.throttle.ip` registry entries, consistent with every other app-feature
  target (listing, chat, shortener). Both limits are admin-editable at runtime through the ratelimit
  app-feature's admin UI/API, unlike the first draft's static `config.yml` values.
- When email confirmation and email-editing land later, they slot into the existing `email` column on
  `RegisterUser`/`model.User` — no schema change anticipated from this feature.

### Key files
- `internal/authnz/handler/user.go` — `RegisterUser` (unchanged logic, now reachable publicly end-to-end)
- `internal/authnz/handler/handler.go` — `RegisterPublicRoutes` (no rate-limit code; relies on `publicRoutes` carrying `rlMw` in `server.go`)
- `internal/ratelimit/registry.go` — `TargetAuthSignupIP` (pre-existing) + `TargetAuthSignupThrottleIP` (added here)
- `internal/ratelimit/middleware/limiter_test.go` — `TestLimiter_ThrottleBeforeDailyQuota_Signup`
- `web/src/routes/register/+page.svelte` — sign-up form + auto-login

---

## ADR-004: Relax `users.name`/`users.email` UNIQUE constraints for optional fields

### Status
Accepted

### Context
Live-verifying ADR-003's rebased implementation (booting a scratch server and actually firing sign-up
requests, not just reading code) surfaced a real defect unrelated to rate limiting: `users.name TEXT
UNIQUE` and `users.email TEXT NOT NULL UNIQUE` (`db/migrations/{sqlite3,postgres}/01_create_init_table`)
never caused a problem before, because every existing writer (3 seeded users, admin-created users) always
sets a distinct, non-blank name and email. Self-service sign-up is the first code path where both are
routinely blank — the idea explicitly makes `name` uncollected and `email` optional. `RegisterUserRequest`
omits them as Go zero-value `""`, and `""` is not `NULL`: the **first** sign-up with a blank
name/email succeeds, and **every one after that gets a 500** (`UNIQUE constraint failed`). Confirmed live
against a real scratch server before any fix, and again after.

`name` has no lookup-by-name anywhere in the codebase — it's a pure display field, so its `UNIQUE`
constraint looks like an unreviewed carry-over, not an intentional design. `email` is different: keeping
it unique when present still makes sense (one real email shouldn't back two accounts, and it matters once
a future confirmation-email flow needs "this email already has an account" semantics) — the defect there
is specifically that "no email" is stored as `''` instead of `NULL`, and SQL `UNIQUE` allows any number of
`NULL`s but not duplicate `''`s.

### Decision
**1. Drop `UNIQUE` from `users.name` entirely** (confirmed with user). No behavior depends on name
uniqueness anywhere in the app.

**2. Make `users.email` nullable, keep `UNIQUE`** (confirmed with user). `NULLIF(:email, '')` on write
(`Create`/`Update` in both `internal/authnz/repository/{sqlite,postgres}/user_repository.go`) stores `NULL`
instead of `''` for an omitted email; `COALESCE(email, '') AS email` on every read path (`GetAll`,
`FindByUsername`, `Find`, and `internal/rbac/repository/{sqlite,postgres}/user_group_repository.go`'s
`ListUsersWithGroup`, which powers the Access Management admin user list) keeps `model.User.Email` a plain
`string` everywhere else in the app — JWT claims, TOTP account naming, admin email-update, frontend JSON —
so the fix is fully contained to the repository layer with no type changes rippling outward. `cmd/all-in-one/db/transfer.go`'s
`userRow.Email` changed from `string` to `sql.NullString`, matching the pattern already used for `Name` in
that same struct.

**3. New migration `09_relax_users_name_email_uniqueness`.** Postgres: direct `ALTER TABLE ... ALTER COLUMN
email DROP NOT NULL` + a dynamic `pg_constraint` lookup to drop `name`'s system-generated UNIQUE constraint
(not hardcoded, since its name was never assigned explicitly) — no live Postgres was available to verify
this migration; it's structurally reviewed only (see Consequences). SQLite has no `ALTER COLUMN`/`DROP
CONSTRAINT`, so it uses the standard rebuild pattern (create table, copy data, drop old, rename) — verified
live end-to-end (see below).

**4. Enable `NoTxWrap: true` for the SQLite migrator** (`internal/storage/sqlite.go`, confirmed with
user). SQLite refuses to change `PRAGMA foreign_keys` inside a transaction — golang-migrate wraps every
migration file in one by default, so `PRAGMA foreign_keys=OFF` in migration 09 was silently a no-op. With
enforcement genuinely still on, `DROP TABLE users` (the rebuild's core step) triggers SQLite's documented
implicit-delete-before-drop, which fails because `sessions.user_id`/`topics.user_id`/etc. still reference
those rows. This is SQLite's own documented procedure for constraint-changing rebuilds — `PRAGMA
foreign_keys=OFF` must run outside a transaction — so `NoTxWrap` is required, not optional, for this class
of migration to ever work against real data.

### Alternatives Considered
| Option | Rejected because |
|---|---|
| `model.User.Email` as `*string` | Idiomatic sqlx nullable-field pattern (matches `LastLogin`, `GroupID` already on the struct), but every consumer of `model.User.Email` — JWT claims, TOTP account naming, admin flows — would need nil-safe handling for no functional benefit; the app never needs to distinguish "explicitly null" from "no email" at that layer, only the DB does. |
| Drop `UNIQUE` from `email` too, matching `name` | Simplest, but permanently allows multiple accounts to share the same real email — matters once email is actually used for anything (confirmation, password reset by email, etc.). |
| Leave `NoTxWrap` off; find a rebuild technique that avoids disabling FK enforcement | Investigated (rename-swap tricks) — SQLite's `ALTER TABLE RENAME` auto-rewrites other tables' `REFERENCES` clauses to follow the rename, so avoiding the drop-triggers-implicit-delete problem would mean rewriting every child table's FK bindings too. More fragile than the documented procedure, for no upside. |

### Consequences
- SQLite migrations no longer auto-rollback a partially-failed file (golang-migrate's dirty-flag guard
  still blocks re-running until manually resolved — recovery is manual, not automatic). Postgres migrations
  are unaffected (separate driver/config, still transactional).
- The Postgres side of migration 09 has **not** been verified against a live Postgres instance — this
  sandbox has neither `psql`/`initdb` nor Docker permissions. Structurally reviewed (dynamic constraint
  lookup avoids hardcoding Postgres's auto-generated name) but should be run against real Postgres before
  this ships, consistent with how the RBAC migration (ADR-related, `.context/RBAC_PROGRESS.md` Phase 1)
  was eventually verified live in a later phase.
- `internal/rbac/repository/{sqlite,postgres}/user_group_repository.go`'s `ListUsersWithGroup` and the
  `internal/authnz` read paths both needed the same `COALESCE` treatment independently — any *other* raw
  query against `users.email` added in the future needs the same care, since a plain `SELECT ... email
  ... ` into a non-nullable `string` field will error the instant any row has `NULL` email.

### Key files
- `db/migrations/{sqlite3,postgres}/09_relax_users_name_email_uniqueness.{up,down}.sql`
- `internal/storage/sqlite.go` — `NoTxWrap: true`
- `internal/authnz/repository/{sqlite,postgres}/user_repository.go` — `NULLIF`/`COALESCE`
- `internal/rbac/repository/{sqlite,postgres}/user_group_repository.go` — `ListUsersWithGroup` COALESCE
- `cmd/all-in-one/db/transfer.go` — `userRow.Email sql.NullString`
