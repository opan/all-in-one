# Self-Service Sign-Up — Progress Tracker

**Status:** ✅ Feature-complete on `feat/signup` (rebased onto `main` @ `44abb71`). Backend + frontend
verified live end-to-end, including a real bug found and fixed during verification. One item needs an
operator with real Postgres access before shipping (see below) — not blocking further dev work.
**Last updated:** 2026-07-12
**Full plan:** [SIGNUP_IMPLEMENTATION_PLAN.md](SIGNUP_IMPLEMENTATION_PLAN.md) (authoritative)
**Design rationale:** [docs/adr/USER_AUTHENTICATION_ADR.md](../docs/adr/USER_AUTHENTICATION_ADR.md) ADR-003 (feature) + ADR-004 (schema bug fix)

## What this feature is
Self-service sign-up (username + password, email optional) using the already-existing `POST /users`
backend endpoint, exposed through a new `/register` frontend page, rate-limited via the real
`internal/ratelimit` app-feature, with auto-login on success. See the plan for full detail.

### Locked decisions (do NOT re-litigate — see ADR-003/ADR-004 for rationale)
1. Reuse `POST /users` as-is; no new backend endpoint.
2. Auto-login after sign-up (`POST /sessions` immediately after `POST /users` succeeds, then redirect to `/`).
3. Rate limiting via the real `internal/ratelimit` app-feature (`auth.signup.ip` daily quota, pre-existing;
   `auth.signup.throttle.ip` burst throttle, added here) — **not** a custom limiter. (An earlier draft built
   a standalone `RateLimiter` before `internal/ratelimit` had merged to `main`; discarded during rebase.)
4. No new password-strength policy — match existing length-≥3 client-side convention.
5. `users.name` UNIQUE constraint dropped entirely; `users.email` made nullable but stays UNIQUE (ADR-004).

## Checklist

- [x] **Docs first** — ADR-003 appended to `docs/adr/USER_AUTHENTICATION_ADR.md`; this progress tracker
      and the implementation plan written, per project convention, before any code changed.
- [x] **Rebase onto `main`** — stashed WIP, updated local `main` (`44abb71`, already current), created
      `feat/signup` off it, popped the stash. Resolved conflicts: dropped the now-obsolete
      `internal/middleware.RateLimiter` extraction and `AuthNZConfig.RateLimit` entirely (superseded by
      `internal/ratelimit`, which merged to `main` mid-branch via PR #22).
- [x] **Backend: rate limiting via the real app-feature** — added `TargetAuthSignupThrottleIP`
      (`auth.signup.throttle.ip`, per-IP throttle, 5/min default) to `internal/ratelimit/registry.go`,
      bound to the same `POST /api/v1/users` route as the pre-existing `auth.signup.ip` daily quota —
      exactly the follow-up the registry's own TODO comment called out for this branch. Zero rate-limit
      code in `internal/authnz` itself; `RegisterPublicRoutes` is a plain route registration, covered
      automatically by `publicRoutes`' `rlMw` in `server.go`. New test
      `TestLimiter_ThrottleBeforeDailyQuota_Signup` proves the throttle sheds a burst before the DB-backed
      daily-quota counter is ever touched. Marked done in `.context/RATE_LIMITING_PROGRESS.md`.
- [x] **Bug found + fixed: `users.name`/`users.email` UNIQUE constraints** (ADR-004) — discovered via live
      verification (not code review): the *second* self-registered user with a blank name/email 500'd
      (`UNIQUE constraint failed`), since `''` (Go zero value) isn't `NULL` and both columns were `UNIQUE`.
      Fixed with migration `09_relax_users_name_email_uniqueness` (name loses UNIQUE; email becomes
      nullable, stays UNIQUE), `NULLIF`/`COALESCE` in `internal/authnz/repository/{sqlite,postgres}`,
      the same fix in `internal/rbac/repository/{sqlite,postgres}/user_group_repository.go`'s
      `ListUsersWithGroup` (Access Management admin list — same defect, found by grepping for other raw
      `users.email` queries rather than assuming the first fix was complete), and `transfer.go`'s
      `userRow.Email` → `sql.NullString`. Required enabling `NoTxWrap: true` on the SQLite migrator
      (`internal/storage/sqlite.go`) — SQLite silently no-ops `PRAGMA foreign_keys=OFF` inside a
      transaction, which golang-migrate wraps every migration in by default; without `NoTxWrap`, the
      table-rebuild's `DROP TABLE users` hit a real FK violation against `sessions`/`topics` once tested
      against non-empty data (the first "successful" test was a false positive — empty DB, nothing to
      violate). All three of these (the two repo layers, the transfer.go type, and NoTxWrap) were found
      by follow-through investigation, not requested up front — see ADR-004 Consequences for what's still
      only structurally reviewed (Postgres side).
- [x] **Docs: `docs/metrics.md`** — no authnz-specific rate-limit metric needed; the real `internal/ratelimit`
      app-feature already emits generic `ratelimit_rejected_total` labeled by target, covering
      `auth.signup.ip`/`auth.signup.throttle.ip` for free.
- [x] **Frontend: `/register` route** — `+layout.ts`, `+layout.svelte`, `+page.svelte` (username,
      password, confirm password, optional email; submit → auto-login → redirect `/`).
- [x] **Frontend: wire login page's "Sign Up" button** to `/register`; reciprocal "Log in" link back
      to `/login` on the register page.
- [x] **Verify** — `go build ./...` and full `go test ./...` clean on every iteration (no regressions).
      `npm run check` (0 errors, only 14 pre-existing warnings in untouched files) and `npm run build`
      clean. Live headless-browser walkthrough (Playwright, real backend + scratch SQLite DB) — 10/10
      checks passed on the first pass (before the name/email bug was found; that bug only surfaces on the
      *second* real registration with blank fields, which the browser E2E script's structure didn't
      happen to exercise before rate-limiting kicked in). Separately, direct `curl` reproduction against a
      live scratch server confirmed both the original bug (four sequential blank-name sign-ups: 1 success
      + 3× 500) and the fix (same sequence: 4× 201 after the migration + repo-layer fix). Migration
      verified with a full up→down→up cycle on SQLite against **populated** data (real seeded
      users/sessions/topics, not an empty DB) — this is what caught the `NoTxWrap` requirement; an empty-DB
      test would have missed it. All scratch servers/DBs/processes cleaned up after each run.
- [ ] **Postgres side of migration 09 — not verified live.** No `psql`/`initdb`/Docker access in this
      sandbox. Structurally reviewed (dynamic `pg_constraint` lookup for `name`'s constraint, rather than
      hardcoding an assumed name). Should be run against real Postgres before this ships — see ADR-004.

## Notes / findings worth remembering
- `RegisterUser` was already fully functional server-side before this feature started — the gap was
  purely: no frontend page, no rate limit wiring, no auto-login UX. Don't re-build backend registration logic.
- Settings page password-reset form is the only precedent for password validation in this codebase
  (`length < 3`, client-side only) — matched that instead of inventing a new policy.
- **Always test migrations against populated data, not just an empty freshly-migrated DB.** A rebuild
  migration that passes on an empty DB can still be completely broken against real foreign-key-linked
  rows — that's exactly what happened here, twice (once for the FK/NoTxWrap issue, and the underlying
  UNIQUE-constraint bug itself only manifests on a *second* real registration, not the first).
- When grepping for a schema-level bug's blast radius, search for every raw query touching the affected
  column (`grep -rn "FROM users\|users\.email"`), not just the one code path you were originally looking
  at — the RBAC admin user list (`ListUsersWithGroup`) had the identical defect and would not have been
  caught by only fixing `internal/authnz`.
