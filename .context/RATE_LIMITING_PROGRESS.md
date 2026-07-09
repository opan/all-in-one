# Rate Limiting App-Feature — Progress Tracker

> **Plan:** [RATE_LIMITING_IMPLEMENTATION_PLAN.md](RATE_LIMITING_IMPLEMENTATION_PLAN.md) · **Decisions:** [docs/adr/RATE_LIMITING_ADR.md](../docs/adr/RATE_LIMITING_ADR.md)
> Live status of the build (18 small phases). Tick each box, update **Resume here**, and commit after each phase.

**Overall status:** 🟡 Phase 9 done — resuming at Phase 10.

**Resume here:** Phase 10 (admin read API: `Handler` + `GET /ratelimit/targets`).

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
- ⬜ **P10 — Read API** · `Handler` + `RegisterAdminRoutes` + `GET /ratelimit/targets` + swagger + mocks
- ⬜ **P11 — Write API** · PATCH + reset + reset-defaults + `config.changed` metric + tests

**Wiring** _(need P9, P10, P11)_
- ⬜ **P12 — Mount admin API (no enforcement)** · construct `rlsvc` + `defer Close` + `RegisterAdminRoutes`
- ⬜ **P13 — Enforce public routes** · `publicRoutes.Use(rlMw)` → login/signup 429
- ⬜ **P14 — Enforce gated routes + boot validation** · `sr.Use(rlMw)` in `mkGated` + `r.Walk` check · regression

**Frontend** _(need P10, P11)_
- ⬜ **P15 — API client + sidebar** · `ratelimit-api.ts` + `adminItems` entry · `npm run check`/`build`
- ⬜ **P16 — Admin page: list + toggle** · `Table` + `Switch` optimistic
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

