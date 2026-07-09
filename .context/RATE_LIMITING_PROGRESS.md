# Rate Limiting App-Feature — Progress Tracker

> **Plan:** [RATE_LIMITING_IMPLEMENTATION_PLAN.md](RATE_LIMITING_IMPLEMENTATION_PLAN.md) · **Decisions:** [docs/adr/RATE_LIMITING_ADR.md](../docs/adr/RATE_LIMITING_ADR.md)
> Live status of the build (18 small phases). Tick each box, update **Resume here**, and commit after each phase.

**Overall status:** 🟡 Phase 16 done — resuming at Phase 17.

**Resume here:** Phase 17 (admin page: edit dialog + reset).

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
- ⬜ **P17 — Admin page: edit + reset** · edit `Dialog` (limit/window) + reset `AlertDialog`

**Closeout**
- ⬜ **P18 — Metrics doc + verification** · `docs/metrics.md` · curl walkthrough on SQLite **and** Postgres · tracker → done

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

## Open items needing operator action (carried from plan)

- ⬜ Confirm whether the public deployment is behind a reverse proxy → if yes, set
     `ratelimit.trust_proxy_headers: true` (else per-IP limits collapse). *(ADR-009)*
- ⬜ Decide whether to include the optional SQLite WAL/`busy_timeout` DSN hardening. *(ADR-007)*

## Notes / deviations

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
- P1: verified `db:migrate up` → `down --steps 1` → `up` clean on **SQLite** only (scratch DB, schema
  diffed via `sqlite_master`, matches migration exactly). Postgres was not reachable in the build sandbox
  (no docker/psql) — the Postgres migration mirrors the SQLite one exactly (only `BOOLEAN`/`TRUE` vs
  `INTEGER`/`1`, same pattern as migration 06's twin files) but has not been live-tested. Run
  `make db-migrate-up` / `db-migrate-down` against `storage.type: postgres` before this phase is considered
  fully closed.
- P4: same sandbox limitation — `RuleRepository` integration tests (Seed idempotency, List/Get/Update/
  ResetToDefault) only ran against SQLite (`:memory:`). The Postgres implementation was written and
  reviewed against the same test cases' intent but not executed (no reachable Postgres in this sandbox);
  only its `?`→`$N` positional placeholder in `Get` and `ON CONFLICT DO NOTHING` in `Seed` differ from the
  SQLite twin, `Update`/`ResetToDefault` use identical named-parameter SQL. Recommend running the
  equivalent tests against `storage.type: postgres` (or at minimum a manual boot + admin-API smoke test
  once P10–P12 land) before considering the repository layer fully verified on that backend.
- P5: same sandbox limitation applies to `CounterRepository.IncrAndGet` — verified the
  `INSERT ... ON CONFLICT ... RETURNING` atomic upsert (incl. a 50-goroutine `-race` concurrency test) only
  against SQLite. The Postgres version differs only in placeholder style and the `rate_limit_counters.count`
  qualifier Postgres requires in the `DO UPDATE SET` clause (unqualified `count` is ambiguous there);
  not live-tested.

