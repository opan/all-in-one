# Self-Service Sign-Up — Implementation Plan

> **Status tracker:** [SIGNUP_PROGRESS.md](SIGNUP_PROGRESS.md) (live status — includes a real schema bug
> found + fixed during verification that this plan predates; see ADR-004)
> **Design rationale:** [docs/adr/USER_AUTHENTICATION_ADR.md](../docs/adr/USER_AUTHENTICATION_ADR.md) ADR-003 (the feature) + ADR-004 (the `users.name`/`users.email` UNIQUE-constraint fix)
> This is the git-tracked, portable copy of the approved plan (the plan-mode copy lives at
> `~/.claude/plans/`, which is machine-local — do not rely on it for resume).

## Context

The idea: allow users to self-register with just username + password (email optional, non-editable for
now, confirmation-email flow deferred until SMTP exists), with a password-confirmation field, and the
sign-up endpoint sitting behind a rate limiter.

**Key finding from investigation:** `POST /users` (`internal/authnz/handler/user.go` `RegisterUser`)
already implements self-registration end-to-end on the backend — username/password required, email/name
optional, immediate activation, already mounted as a *public* route. It was built for admin/seed use but
has no admin check. It was just never exposed through the frontend — no `/register` page exists, only
`/login`. The login page even has an unwired "Sign Up" button already sitting there waiting for it
(`web/src/routes/login/+page.svelte:134`).

**This branch was rebased onto `main` mid-implementation** after `internal/ratelimit` (the real,
admin-configurable rate-limit app-feature — DB-backed rules/counters, target registry, admin UI) merged
via PR #22 (`44abb71`). It had already pre-registered `POST /api/v1/users` as target `auth.signup.ip`,
with an explicit TODO comment in `internal/ratelimit/registry.go` for this exact branch to add a burst
throttle in front of it. The original plan (below, backend section) built a standalone `RateLimiter`
before that merge existed — that was discarded during the rebase in favor of the real system. See
ADR-003's "Revised after rebasing onto main" note for the full story.

So this plan is scoped to what's actually missing:
1. The `/register` frontend page (with password confirmation).
2. The deferred throttle-in-front-of-daily-quota follow-up in `internal/ratelimit/registry.go`.
3. Auto-login after successful sign-up (confirmed with user — see ADR-003).

### Locked decisions (confirmed with user / recorded in ADR-003 — do not re-litigate)
1. Reuse `POST /users` as-is; no new backend endpoint.
2. Auto-login after sign-up (call `POST /sessions` immediately, then redirect to `/`).
3. Rate limiting via the real `internal/ratelimit` app-feature — no rate-limit code in `internal/authnz`
   at all. `POST /api/v1/users` is already covered by `auth.signup.ip` (daily quota) automatically via
   `publicRoutes`' `rlMw` in `server.go`. The only code change is one new registry entry
   (`auth.signup.throttle.ip`, a per-IP burst throttle evaluated first) — see Backend changes below.
4. No new password-strength policy — match the existing convention (settings page: length ≥ 3, client-side only).

## Backend changes

### 1. Add the deferred burst-throttle registry entry
`internal/ratelimit/registry.go` — add `TargetAuthSignupThrottleIP` (`auth.signup.throttle.ip`), a per-IP
`throttle` (5/minute default) bound to the same `POST /api/v1/users` route as the existing
`TargetAuthSignupIP` daily quota. `orderThrottlesFirst` (in `internal/ratelimit/middleware/limiter.go`)
automatically evaluates throttle-kind targets before daily_quota-kind targets sharing a route — no other
wiring needed; `RouteBindings` already supports multiple targets per route (proven by an existing test).

Add a middleware test proving the throttle sheds a burst before it reaches the DB-backed daily-quota
counter — `TestLimiter_ThrottleBeforeDailyQuota_Signup` in
`internal/ratelimit/middleware/limiter_test.go`, asserting the fake counter store is only touched once
even though two requests are sent.

Mark the deferred item done in `.context/RATE_LIMITING_PROGRESS.md`.

### 2. `internal/authnz` — no rate-limit code
`RegisterPublicRoutes` in `internal/authnz/handler/handler.go` stays a plain
`router.HandleFunc("/users", h.RegisterUser).Methods("POST")` (a comment notes why it's already
rate-limited). Also fix the stale swagger comment (`@Router /users/register [post]` → `@Router /users [post]`).

(The first draft's `internal/middleware.RateLimiter` extraction, `AuthNZConfig.RateLimit`, and
`registerRateLimited` metric were all removed during the rebase — superseded by the above.)

## Frontend changes

- `web/src/routes/register/+layout.ts` — `export const ssr = false;` (copy of login's).
- `web/src/routes/register/+layout.svelte` — same minimal header shell as login's layout.
- `web/src/routes/register/+page.svelte` — username (required), password (required), confirm password
  (required, client-side equality check, ≥3 chars matching settings-page convention), email (optional).
  On submit: `POST /api/v1/users` → on success, `POST /api/v1/sessions` (auto-login) → `goto('/')`.
  Errors: 409 → "Username already taken", 429 → "Too many sign-up attempts, please try again later",
  else → server error string / generic fallback. Reuses `apiPost` (`$lib/api.ts`) and the same
  shadcn-svelte `Card`/`Input`/`Label`/`Button` components as the login page.
- `web/src/routes/login/+page.svelte` — wire the existing unwired "Sign Up" button to `goto('/register')`.
  Register page gets a reciprocal "Already have an account? Log in" link.
- No change needed to `web/src/routes/+layout.svelte` — already treats `/register*` as an auth page.

## Verification
- Backend: `go build ./...`, `go test ./...` (new `internal/ratelimit/middleware` throttle test +
  `internal/ratelimit` registry tests, which key off `len(Registry)` dynamically so the new entry needs
  no test-count updates; existing `internal/authnz/handler` tests unaffected).
- Frontend: `npm run check`, `npm run build` in `web/`.
- End-to-end (frontend-browser-testing skill against the real Go backend, with the real `internal/ratelimit`
  system live — no more config.yml hand-tuning to trigger 429s, use the ratelimit admin API/DB to lower
  `auth.signup.throttle.ip`'s limit for the test run instead): sign up a new user; password-mismatch caught
  client-side; successful sign-up lands logged-in on `/`; duplicate username shows 409 message; exceeding
  the throttle gets rate-limited (429).
