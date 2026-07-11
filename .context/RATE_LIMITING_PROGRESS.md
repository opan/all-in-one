# Rate Limiting App-Feature — Progress Tracker

> **Plan:** [RATE_LIMITING_IMPLEMENTATION_PLAN.md](RATE_LIMITING_IMPLEMENTATION_PLAN.md) · **Decisions:** [docs/adr/RATE_LIMITING_ADR.md](../docs/adr/RATE_LIMITING_ADR.md)
> Live status of the build (18 small phases). Tick each box, update **Resume here**, and commit after each phase.

**Overall status:** ✅ **Done.** All 18 phases complete and verified on both SQLite and Postgres. Docker was
installed in the sandbox (`docker.io` + `docker-compose-v2` via apt) and `docker compose up -d postgres`
gave a reachable Postgres instance, closing the one outstanding gap from earlier sessions — see the
Postgres verification checklist below for what was run.

**Nothing left to resume** for this feature. Remaining open items (reverse-proxy config, SQLite WAL
hardening) are operator decisions, not implementation work — see "Open items needing operator action"
below. One code follow-up is intentionally **deferred to the future sign-up branch** (add an in-memory
throttle in front of the signup quota) — see "Follow-up work (deferred to future branches)" below.

Legend: ⬜ not started · 🟨 in progress · ✅ done

---

## Phase checklist

**Foundations** _(P1–P3 independent; any order)_
- ✅ **P1 — Migration 08 (schema only)** · both backends · `db:migrate up`/`down` clean on SQLite+Postgres
- ✅ **P2 — Models + errors + registry** · `model.go`, `errors.go`, `registry.go` (6 targets) + lookup tests
- ✅ **P3 — Config** · `RateLimitConfig` + defaults + per-key `BindEnv` + `config.yml` block

**Repository** _(need P1, P2)_
- ✅ **P4 — Rule repository** · interface/factory/adapter + sqlite/postgres + seed-idempotency tests + mocks
- ✅ **P5 — Counter repository** · atomic `IncrAndGet` (`ON CONFLICT … RETURNING`) + concurrency test + mocks

**Service** _(need P4, P5, P3)_
- ✅ **P6 — Service core** · `NewService` + seed + `ruleCache` (RuleProvider) + tickers + `Close` + tests
- ✅ **P7 — Service admin ops** · List/Update/ResetCounters/ResetDefaults + reload-on-write + tests

**Middleware** _(need P2, P3; P9 needs P8)_
- ✅ **P8 — In-memory store (`memStore`)** · per-call limit/window + cleanup + `Stop` + tests
- ✅ **P9 — Limiter middleware** · dispatch + `clientIP` + reject/fail-open + metrics + mux/httptest tests

**Handler (admin API)** _(need P6, P7)_
- ✅ **P10 — Read API** · `Handler` + `RegisterAdminRoutes` + `GET /ratelimit/targets` + swagger + mocks
- ✅ **P11 — Write API** · PATCH + reset + reset-defaults + `config.changed` metric + tests

**Wiring** _(need P9, P10, P11)_
- ✅ **P12 — Mount admin API (no enforcement)** · construct `rlsvc` + `defer Close` + `RegisterAdminRoutes`
- ✅ **P13 — Enforce public routes** · `publicRoutes.Use(rlMw)` → login/signup 429
- ✅ **P14 — Enforce gated routes + boot validation** · `sr.Use(rlMw)` in `mkGated` + `r.Walk` check · regression

**Frontend** _(need P10, P11)_
- ✅ **P15 — API client + sidebar** · `ratelimit-api.ts` + `adminItems` entry · `npm run check`/`build`
- ✅ **P16 — Admin page: list + toggle** · `Table` + `Switch` optimistic
- ✅ **P17 — Admin page: edit + reset** · edit `Dialog` (limit/window) + reset `AlertDialog`

**Closeout**
- ✅ **P18 — Metrics doc + verification** · `docs/metrics.md` · curl walkthrough on SQLite **and** Postgres · tracker → done

---

## Decision log (quick reference — full rationale in the ADR)

| # | Decision | ADR |
|---|---|---|
| 1 | Admin-only app-feature, DB-backed config (not config-file) | ADR-001 |
| 2 | Hybrid counters: in-memory throttles + DB daily quotas | ADR-002 |
| 3 | Named target registry synced to DB rules | ADR-003 |
| 4 | Route-resolving middleware + boot-time binding validation | ADR-004 |
| 5 | Per-rule natural key (ip/user/global); global config | ADR-005 |
| 6 | UTC calendar-day buckets, count-on-entry | ADR-006 |
| 7 | Fail-open on counter/store errors | ADR-007 |
| 8 | Rule cache reloaded on write; master switch boot-time | ADR-008 |
| 9 | Client IP opt-in proxy-aware (`trust_proxy_headers`, default false) | ADR-009 |
| 10 | Admins not exempt in v1 | ADR-010 |

## Postgres verification checklist — ✅ completed 2026-07-09

Ran against a real Postgres 15.18 via `docker compose up -d postgres` (repo root) once Docker was installed
in the sandbox (`sudo apt install docker.io docker-compose-v2`, `usermod -aG docker`, `newgrp docker`).
`docker.io` alone does **not** include the compose plugin — needed `docker-compose-v2` separately, and a
fresh shell/`newgrp` after the group change (both tripped the user before landing on the working combo).

- ✅ **Migration 08** — `up` against an empty DB, confirmed via `information_schema.columns` that both
  tables + `idx_rate_limit_counters_day` exist with the expected Postgres types (`enabled boolean`, not
  `integer` — confirms the twin migration's `BOOLEAN`/`INTEGER` split is correct); `down --steps 1` confirmed
  both tables fully gone (`pg_tables` count → 0). *(P1)*
- ✅ **Rule repository** — booted the server with `storage.type=postgres` (boot log: `"targets":6 ...
  bound`), exercised the full admin API for real: `GET` list, `PATCH` (limit + `updated_by`), `POST
  reset-defaults`, `POST reset`, unknown-key `PATCH` → 404 — all matched SQLite behavior exactly. *(P4)*
- ✅ **Counter repository** — the daily-quota walkthrough (`auth.signup.ip`) hit `IncrAndGet` for real:
  count-on-entry held through a request that itself 500'd, then 429'd on the 3rd attempt. Went further than
  the checklist asked: fired **30 concurrent** `POST /api/v1/topics/{id}/items` requests (`xargs -P 30`)
  against `listing.item.create` and read `rate_limit_counters` directly afterward — `count = 30` exactly,
  proving Postgres's qualified-column `ON CONFLICT ... DO UPDATE SET count = rate_limit_counters.count + 1`
  form (the one structurally-different piece of SQL vs. SQLite) has no lost updates under real concurrency.
  *(P5)*
- ✅ **Full enforcement walkthrough on Postgres** — login throttle → 429 (+`Retry-After`), signup quota →
  429, records/user/day (`listing.item.create`) → 429, then `PATCH .../auth.login {"enabled":false}` → login
  passed again immediately, no restart. Identical to the SQLite walkthrough in every observable respect.
  Bonus: also reconfirmed that *raising* a throttled target's limit via `PATCH` un-sticks it immediately
  (cache-reload-on-write, ADR-008) — needed this twice mid-walkthrough just to get a fresh login through
  after intentionally exhausting `auth.login` in an earlier step.

- ✅ **Frontend/UI against Postgres** — the earlier curl walkthrough only exercised the API; separately ran
  the exact P16/P17 Playwright checks (`frontend-browser-testing` skill) against a Postgres-backed server.
  Needed an admin login for the real browser form (unlike curl, the SPA can't use the `x-direct-auth-username`
  header trick), and `db:seed` had skipped creating the usual `admin/admin123` user since `pguser1` already
  existed from the earlier curl testing — promoted `pguser1` to the admin group via the RBAC admin API
  instead (explicitly user-authorized after the safety classifier correctly blocked the first unprompted
  attempt — a fair block, since only a status question had been asked, not an instruction to do this).
  9/9 checks passed: sidebar link visible, table lists all 6 targets, enable/disable toggle persists through
  a reload, and an edited limit (`auth.login` → 42) persists through a reload — all round-tripping through
  real Postgres, not just the API layer.

All scratch server processes and temp files from this verification were cleaned up afterward. The
`docker compose` Postgres container itself was left running (operator's infrastructure, not a scratch
artifact) — stop it with `docker compose down` if it's no longer needed. Its data now includes verification
scratch state (`pguser1`/`pguser2`/`pguser3` test users, `pguser1` promoted to admin, a test topic, and
various limit/enabled edits from both the curl and browser passes) — harmless for a throwaway verification
DB, but worth a `docker compose down -v` (drops the volume) before using this container for anything else.

## Open items needing operator action (carried from plan)

- ⬜ Confirm whether the public deployment is behind a reverse proxy → if yes, set
     `ratelimit.trust_proxy_headers: true` (else per-IP limits collapse). *(ADR-009)*
- ⬜ Decide whether to include the optional SQLite WAL/`busy_timeout` DSN hardening. *(ADR-007)*

## Follow-up work (deferred to future branches)

- ⬜ **Add an in-memory throttle in front of the signup daily quota** — do this when the sign-up feature is
     built out on its own branch (developed separately, later). Today `auth.signup.ip` (`POST /api/v1/users`,
     `daily_quota`) is the only **public/unauthenticated** rate-limit target with no `throttle` sharing its
     route, so a signup flood performs a DB counter upsert on *every* request — including every request past
     the quota, since the counter keeps incrementing — contrary to ADR-002's "shed floods in-memory before
     any DB write" goal. The other three `daily_quota` targets sit behind `JWTAuth`, so auth sheds
     unauthenticated floods before the counter is touched; signup is the exception. On SQLite this write
     amplification can also trip `database is locked` → fail-open (ADR-007), disabling the quota under the
     very load it exists to resist.
     - **Fix (small, no new machinery):** add a per-IP `throttle` `TargetDef` on `POST /api/v1/users` in
       `internal/ratelimit/registry.go` (a burst cap, e.g. a few/min). `orderThrottlesFirst` evaluates it
       before the daily quota, so a burst is shed in-memory while the quota still enforces the slow-drip
       daily total — exactly the shape `auth.login` already has. Add a matching middleware test.
     - **General principle:** any *public* (unauthenticated) `daily_quota` target should have a throttle in
       front of it; JWT-gated quotas are already covered by auth. *(ADR-002)*
     - Marker left at the `auth.signup.ip` entry in `internal/ratelimit/registry.go`.

- ⬜ **(Watch — no action yet) Per-account throttle for login.** `auth.login` is deliberately **IP-scoped**,
     and that's a strong default, not a compromise: `ScopeUser` can't apply at login (login *creates* the
     auth, so there's no user in context), per-username keying would require the body-blind route middleware
     to parse the request body (ADR-004), and IP is the natural key for the flood/burst threat model a
     `throttle` targets (ADR-002). Known uncovered case: a **distributed** brute-force against a single known
     account, where each IP stays under the per-IP limit. **Decision (2026-07-11): keep IP-only until a real
     case actually happens to the AIO; revisit then.** If it's ever needed, the fix is a failures-only
     per-username counter inside `authnz` `CreateSession` (not a new registry scope) — and note there's no
     account-lockout elsewhere today (only the 2FA step caps attempts). *(ADR-005)*

## Notes / deviations

- P18: added a "Rate Limiting" section to `docs/metrics.md` (3 metrics, label-values table, cardinality
  entry: `rejected_total` bounded by target count not the label cross-product since scope/kind are fixed
  per target, `errors_total` bounded by the 4 `daily_quota` targets since throttle targets never touch the
  counter store, `config_changed_total` at the full 6×3 target×action product). Added a Consequences bullet
  to ADR-004 documenting the `rlMw` vs `RequireFeature` ordering decision made in P14 (not explicitly
  specified when the ADR was written). Full local verification pass: `go build`/`vet`/`test -race` all green
  repo-wide, `npm run check`/`build` clean, `mockery` regenerated with zero drift in any `ratelimit` mock.
  Ran the plan's exact SQLite curl walkthrough end-to-end on a fresh scratch DB: login throttle → 429,
  signup quota → 429 (count-on-entry through a 500), records/user/day (`listing.item.create`) → 429, then
  `PATCH .../auth.login {"enabled":false}` → login passes again (404 for bad creds, not 429) with **no
  restart**. Bonus: also confirmed *raising* a throttle's limit via PATCH immediately un-sticks an
  already-throttled bucket (cache reload takes effect on the very next request), which the plan didn't
  explicitly call for but follows directly from ADR-008. Postgres wasn't reachable in this sandbox at the
  time — **resolved**, see the Postgres verification checklist above (repeated this exact walkthrough there).
- P17: added a "Defaults" (reset-to-defaults) row action beyond the plan's minimum ask, sharing one
  `AlertDialog` parameterized by `resetAction: 'reset' | 'reset-defaults'` rather than two separate dialogs.
  DoD verified live via the browser-testing skill: opened the Edit dialog, changed limit to 25 and window to
  "2 hours" (using `[data-slot="select-trigger"]` per the skill's documented bits-ui `Select` gotcha, not
  `role="combobox"`), saved, confirmed the success toast + updated row values, reloaded to confirm server-side
  persistence, then opened Reset, confirmed, and confirmed its toast too — 9/9 checks passed.
- P16: DoD verified live via the `frontend-browser-testing` skill (seeded scratch DB, real server, headless
  Chromium): table lists all 6 targets with name/key/scope-badge/kind-badge/limit/window; clicking the
  `auth.login` switch flips it and a page reload confirms the change persisted server-side (8/8 checks
  passed, screenshots saved then discarded with the scratch dir). Scope/kind use hand-rolled badge spans
  colored via the same Tailwind pattern as the existing shortener Active/Inactive badge (blue=ip,
  purple=user, amber=throttle, teal=daily quota) — not the dataviz skill's palette, since these are simple
  categorical status badges consistent with the app's existing design language, not a chart/visualization.
- P15: `node`/`npm` aren't on `PATH` by default in this sandbox — available via `nvm` (`export
  NVM_DIR="$HOME/.nvm" && . "$NVM_DIR/nvm.sh"` then the normal commands, all in one Bash call since env
  doesn't persist across tool calls). `npm run check` (0 errors, only pre-existing unrelated warnings) and
  `npm run build` both passed. Used the `frontend-browser-testing` skill to confirm the "Rate Limits" link
  (with its Gauge icon) actually renders in the Admin sidebar group for a logged-in admin — 5/5 checks
  passed, screenshot showed it positioned right after Shortener as expected.
- P14: `rlMw` (from `rlsvc.LimiterMiddleware()`, computed once) is shared across `publicRoutes` and every
  `mkGated(feature)` subrouter — one `*middleware.Limiter`/`memStore` for the whole process, not one per
  subrouter. Placed `sr.Use(rlMw)` between `JWTAuth` and `RequireFeature` (literal reading of "after
  JWTAuth"): a request that fails the feature gate still consumes its rate limit counter. Boot-time
  validation (`validateRateLimitBindings`, `r.Walk` over the whole router) logs `Fatal` (process exit 1) on
  any Registry target with no matching registered route. DoD verified live end-to-end against a scratch DB:
  boot log shows `"targets":6 ... all targets bound to a registered route"`; authed
  `POST /api/v1/topics/{topic_id}/items` 429s with `Retry-After` after the (admin-lowered) quota, keyed per
  user via JWT claims; listing/shortener authed traffic still works normally (200/201) through the extra
  middleware; and a deliberately introduced path typo (`/api/v1/sessions-typo`) made the boot fail with
  `level:fatal` + exit code 1 and a precise "missing" list — then was reverted (clean `git diff`) before
  committing.
- P13: `Service.LimiterMiddleware()` (deferred from P9, see that note) is now built: `Service` gained a
  `limiter *middleware.Limiter` field, constructed eagerly in `NewService` from `s.cache` (satisfies
  `RuleProvider`) and `store.CounterRepo()` (satisfies `CounterStore`); `Close()` now also stops it. Existing
  service tests' `newTestService` helper needed a real (non-nil) limiter too — `Close()` unconditionally
  calls `s.limiter.Stop()`, which nil-panics on a zero-value `Service`; fixed by constructing
  `middleware.NewLimiter(cache, nil, cfg)` in the test helper (the `nil` counters arg is never dereferenced
  by `Stop()`). DoD verified live: booted against a scratch DB, used the admin PATCH API to drop
  `auth.login`/`auth.signup.ip` limits to 3 for a fast test, then confirmed both throttle (429 +
  `Retry-After` after 3 requests/minute) and daily-quota (429 + `Retry-After` ≈ seconds-to-midnight after 3
  requests/day, counted on entry regardless of the signup's own success/failure — a `UNIQUE constraint`
  500 on repeated usernames still consumed quota, confirming ADR-006 count-on-entry) enforcement fire
  correctly, and a normal first request passes through untouched (201).
- P12: DoD verified live (not just built) — booted the server twice against scratch SQLite DBs
  (`ALLINONE_STORAGE_SQLITE_DB_PATH=/tmp/...`), once with `rbac.direct_auth_is_admin=true` (GET/PATCH/reset/
  reset-defaults all 200, unknown-key PATCH 404) and once with it `false` (GET → 403). Confirmed 12 rapid
  `POST /api/v1/sessions` calls all returned 404 (bad credentials), never 429 — enforcement genuinely isn't
  wired yet. `rlsvc` sits on the same `adminRoutes` subrouter as rbac/authnz/shortener admin APIs, so it
  inherits the already-proven `RequireAdmin` gating rather than needing its own auth test.
- P11: the PATCH body is decoded directly into `model.TargetPatch` (from P7) rather than a separate
  handler-local request struct — its pointer fields and `json` tags already matched what P11 needed.
  `window_unit` enum validation relies entirely on the service layer's `ratelimit.ErrInvalidWindowUnit` →
  400 mapping (no duplicate enum check in the handler); only `limit_count ≥ 1` is validated at the handler
  layer, per the plan's explicit callout. `updated_by` comes from `auth.GetUserFromContext(ctx).Username`
  (the route is admin-gated, so JWTAuth has already populated it by the time this handler runs).
- P10: skipped an empty `handler/metrics.go` (the plan lists it as a P10 file) since `ListTargets` needs no
  metric — no admin write action exists yet. `handlerMetrics` will be introduced in P11 alongside the
  `aio.ratelimit.config.changed` counter it's actually for; a metrics.go with nothing in it would be dead
  code. `Handler` construction is wired into `service.NewService` (`s.Handler = handler.NewHandler(s,
  config)`) and `Service.RegisterAdminRoutes` passes through to it, exactly as the plan specified.
- P9: `Limiter`/`Middleware()` is built and fully tested standalone (constructed directly with a
  `RuleProvider`/`CounterStore` in tests), per the plan's file list (`middleware/limiter.go`,
  `middleware/metrics.go` only). `Service.LimiterMiddleware()` — the glue that constructs a real
  `*middleware.Limiter` from `s.cache` (satisfies `RuleProvider` structurally) and
  `s.Store.CounterRepo()` (satisfies `CounterStore`) — is deferred to P12 (Wiring), since that's where
  `server.go` actually needs it and touching `service.go` for it now would go beyond P9's stated scope.
  `Limiter.Stop()` must be called wherever that glue method is added (mirrors `rlsvc.Close()`).
  Retry-After for a rejected daily quota is seconds-until-midnight in the configured timezone (not yet
  specified by the plan; ADR-006 defines the day boundary this derives from).
- P7: added `TargetPatch` to `model` (pointer fields for the P11 PATCH endpoint) and a `Service.location()`/
  `today()` helper pair that resolves `ratelimit.timezone` (falling back to UTC) — used by `ResetCounters`
  and retrofitted into P6's cleanup-ticker cutoff calculation, which had hard-coded UTC. Not a plan
  deviation, just completing ADR-006's "timezone is a config knob" for both call sites that compute "today".
- P6: `EffectiveRule` was placed in `model` (not declared inline in `service`) so both `service` (P6) and
  the future `middleware` package (P9) can reference the identical type for `RuleProvider.Effective` without
  an import cycle. `middleware.RuleProvider` itself is still to be declared in P9 — `ruleCache`'s
  `Effective`/`Reload` methods already match the shape the plan specifies and will satisfy it structurally.
- P1: verified `db:migrate up` → `down --steps 1` → `up` clean on **SQLite** only at the time (scratch DB,
  schema diffed via `sqlite_master`, matches migration exactly). Postgres wasn't reachable in the build
  sandbox then — **resolved**, see the Postgres verification checklist above.
- P4: same sandbox limitation at the time — `RuleRepository` integration tests (Seed idempotency,
  List/Get/Update/ResetToDefault) only ran against SQLite (`:memory:`); only `?`→`$N` in `Get` and
  `ON CONFLICT DO NOTHING` in `Seed` differ from the SQLite twin, `Update`/`ResetToDefault` use identical
  named-parameter SQL. **Resolved** — see the Postgres verification checklist above (verified via the full
  admin API against a live Postgres rather than a repository-level `_test.go`, per that checklist's
  "at minimum" option).
- P5: same sandbox limitation at the time for `CounterRepository.IncrAndGet` — the 50-goroutine `-race`
  concurrency test only ran against SQLite; the Postgres version differs from SQLite in the
  `rate_limit_counters.count` qualifier Postgres requires in `DO UPDATE SET` (unqualified `count` is
  ambiguous there), not just placeholder style. **Resolved** — see the Postgres verification checklist above
  (30-concurrent-request burst against a live Postgres, verified via the persisted counter value rather than
  a repository-level test).

