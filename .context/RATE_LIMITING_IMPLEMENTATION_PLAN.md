# Rate Limiting App-Feature — Implementation Plan

> **Status tracker:** [RATE_LIMITING_PROGRESS.md](RATE_LIMITING_PROGRESS.md) (live phase-by-phase status)
> **Design rationale:** [docs/adr/RATE_LIMITING_ADR.md](../docs/adr/RATE_LIMITING_ADR.md) (the *why* behind each decision)
> This is the git-tracked, portable copy of the approved plan (the plan-mode copy lives at
> `~/.claude/plans/`, which is machine-local — do not rely on it for resume).

This plan is deliberately sliced into **18 small phases**. Each phase (a) touches a handful of files,
(b) **leaves the tree green** — `go build ./...` passes and its own tests pass — and (c) is independently
committable. A fresh chat session can resume at any phase boundary by reading just that phase's section plus
[RATE_LIMITING_PROGRESS.md](RATE_LIMITING_PROGRESS.md). Commit after each phase (suggested message in the
phase's DoD).

```
Foundations   P1 schema        P2 models+registry     P3 config          (P1–P3 independent; any order)
Repository    P4 rule-repo  →  P5 counter-repo                           (need P1 schema, P2 models)
Service       P6 service-core → P7 service-admin-ops                     (need P4,P5,P3)
Middleware    P8 memStore    →  P9 limiter                               (need P2 registry, P3; P9 needs P8)
Handler       P10 read-API   →  P11 write-API                           (need P6,P7)
Wiring        P12 mount-admin → P13 enforce-public → P14 enforce-gated+validate   (need P9,P10,P11)
Frontend      P15 client+nav →  P16 list+toggle → P17 edit+reset         (need P10,P11)
Closeout      P18 metrics-doc + full verification (SQLite+Postgres) + tracker closeout
```

## Context

The AIO app is deployed publicly and freely accessible. Before a wider announcement, the homelab (limited
resources, single instance) needs to cap traffic to selected endpoints so it is not flooded by garbage/DDOS
traffic. Concretely: sign-ups/day (per IP), records/user/day (per user), login attempts (per IP), and future
endpoints — all admin-configurable at runtime with a per-endpoint on/off toggle.

This is a **greenfield admin-only app-feature** modeled on the existing `internal/rbac` (access-management)
app-feature and reusing the in-memory limiter algorithm from `internal/shortener/middleware/ratelimit.go`. It
reuses established module conventions: repository interface + factory + sqlite/postgres impls, per-subrouter
middleware, idempotent seed-on-boot, dual-DB migrations, per-package OTel metrics.

### Locked decisions (confirmed with user — do not re-litigate; see ADR for full rationale)

1. **Hybrid counters** — burst throttles counted in-memory (fast, DB off the hot path); per-day quotas
   counted in the DB (survive restarts). *(ADR-002)*
2. **Named target registry** synced to DB rules (mirrors `rbac.Registry`); admin toggles/tunes each; new
   endpoint = add a `TargetDef` + ensure its subrouter carries the middleware. *(ADR-003)*
3. **Per-rule natural key** — one global config; each rule counts per IP / user / global by its `Scope`.
   *(ADR-005)*
4. **Enforcement via route-resolving middleware** (not call-site `Guard`), with boot-time route-binding
   validation. *(ADR-004)*
5. **UTC calendar-day buckets, count-on-entry**; **fail-open** on counter errors. *(ADR-006, ADR-007)*
6. **Rules cached in memory, reloaded on write** (runtime effect); master switch is boot-time config.
   *(ADR-008)*
7. **Client IP is opt-in proxy-aware** (`trust_proxy_headers`, default false). *(ADR-009)*
8. **Admins are not exempt** in v1. *(ADR-010)*
9. **Full-stack** — backend + admin REST API + Svelte admin page at `/admin/ratelimit`.

> Reserved-word caution: never name DB columns `limit`/`window`; use `limit_count`, `window_value`,
> `window_unit`. Squirrel is NOT used anywhere — write hand-parameterized `sqlx` SQL.

### Initial registry targets (all runtime-editable after seed)

| Key | Scope | Kind | Route | Default |
|---|---|---|---|---|
| `auth.login` | ip | throttle | POST /api/v1/sessions | 10 / minute |
| `auth.signup.ip` | ip | daily_quota | POST /api/v1/users | 20 / day |
| `listing.item.create` | user | daily_quota | POST /api/v1/topics/{topic_id}/items | 500 / day |
| `listing.topic.create` | user | daily_quota | POST /api/v1/topics | 100 / day |
| `chat.session.create` | user | daily_quota | POST /api/v1/chats | 100 / day |
| `chat.message.send` | user | throttle | POST /api/v1/chats/{id}/messages | 60 / minute |

---

# Foundations

## Phase 1 — Migration `08` (schema only)
**Goal:** the two tables exist in both backends; zero behavior change. **Depends on:** nothing.
**Files:** `db/migrations/sqlite3/08_add_rate_limit_tables.{up,down}.sql`,
`db/migrations/postgres/08_add_rate_limit_tables.{up,down}.sql`. (Next number is **08**; migrations auto-run
on server start and in `db:seed`.)

```sql
-- up (sqlite3; postgres twin: enabled BOOLEAN NOT NULL DEFAULT TRUE)
CREATE TABLE IF NOT EXISTS rate_limit_rules (
    target_key   TEXT PRIMARY KEY,
    enabled      INTEGER NOT NULL DEFAULT 1,
    limit_count  INTEGER NOT NULL,
    window_value INTEGER NOT NULL,
    window_unit  TEXT    NOT NULL,             -- 'second'|'minute'|'hour'|'day'
    updated_at   TIMESTAMP NOT NULL,
    updated_by   TEXT
);
CREATE TABLE IF NOT EXISTS rate_limit_counters (
    target_key TEXT NOT NULL,
    bucket_key TEXT NOT NULL,   -- 'ip:<addr>' | 'user:<id>' | 'global'
    day        TEXT NOT NULL,   -- 'YYYY-MM-DD' in configured tz (default UTC)
    count      INTEGER NOT NULL DEFAULT 0,
    updated_at TIMESTAMP NOT NULL,
    PRIMARY KEY (target_key, bucket_key, day)
);
CREATE INDEX idx_rate_limit_counters_day ON rate_limit_counters(day);
```
Down: drop index, then `rate_limit_counters`, then `rate_limit_rules`.
**DoD:** `db:migrate up` then `down --steps 1` clean on **both** SQLite and Postgres; `go build ./...`.
Commit: `feat(ratelimit): add migration 08 for rate limit tables`.

## Phase 2 — Models + errors + registry
**Goal:** the top-level `ratelimit` package compiles and registry lookups are unit-tested. **Depends on:** none.
**Files:** `internal/ratelimit/model/model.go`, `internal/ratelimit/errors.go`, `internal/ratelimit/registry.go`.
- `model`: `Scope{ip|user|global}`, `Kind{throttle|daily_quota}` string enums; `Rule`, `Counter`, `Target`
  structs (json+db tags; `Rule` db tags `limit_count/window_value/window_unit`).
- `errors.go`: `ErrUnknownTarget`, `ErrInvalidWindowUnit` (top-level pkg — avoids import cycles).
- `registry.go`: `TargetDef` struct + `var Registry` (the 6 rows above) + `init()`-built maps →
  `ByKey(key)`, `RouteBindings(method, template) []TargetDef`, `Registered()`. Mirrors `internal/rbac/features.go`.
**DoD:** `go build ./...`; unit tests for `ByKey`/`RouteBindings` (incl. a route with the `{topic_id}` var and
a >1-target route case). Commit: `feat(ratelimit): add models, errors, and target registry`.

## Phase 3 — Config
**Goal:** `RateLimitConfig` is wired into viper. **Depends on:** none (do early).
**Files:** `internal/config/config.go`, `config/config.yml`.
```go
// in Config: RateLimit RateLimitConfig `mapstructure:"ratelimit"`
type RateLimitConfig struct {
    Enabled              bool          `mapstructure:"enabled"`                // master kill switch (boot-time)
    CacheRefreshInterval time.Duration `mapstructure:"cache_refresh_interval"` // 30s
    CleanupInterval      time.Duration `mapstructure:"cleanup_interval"`       // 1h
    CounterRetentionDays int           `mapstructure:"counter_retention_days"` // 3
    Timezone             string        `mapstructure:"timezone"`               // "UTC"
    TrustProxyHeaders    bool          `mapstructure:"trust_proxy_headers"`    // false — SEE ATTENTION #1
}
```
`viper.SetDefault` for each + an explicit `viper.BindEnv("ratelimit.<k>", "ALLINONE_RATELIMIT_<K>")` for each
(underscore-key gotcha at `config.go:163-186`); add a `ratelimit:` block to `config/config.yml`.
**DoD:** server boots with defaults; `ALLINONE_RATELIMIT_ENABLED=false` override verified; `go build ./...`.
Commit: `feat(ratelimit): add rate limit configuration`.

---

# Repository

## Phase 4 — Rule repository (dual backend)
**Goal:** `RuleRepository` fully implemented + tested. **Depends on:** P1, P2.
**Files:** `repository/{interface.go, factory.go, adapter.go}`, `repository/sqlite/{storage.go, helpers.go,
rule_repository.go}`, `repository/postgres/{storage.go, helpers.go, rule_repository.go}`, tests, `.mockery.yaml`.
```go
type RuleRepository interface {
    List(ctx) ([]model.Rule, error)
    Get(ctx, targetKey string) (model.Rule, error)
    Seed(ctx, r model.Rule, opts ...query.QueryOptions) error   // insert-if-absent (no clobber)
    Update(ctx, r model.Rule, opts ...query.QueryOptions) error
    ResetToDefault(ctx, r model.Rule, opts ...query.QueryOptions) error
}
type Storage interface { RuleRepo() RuleRepository; CreateTrx(ctx) (query.QueryOptions, error); Close() error }
```
Copy `helpers.go` (`queryOptions{trx}`, `getExecCtx`, `createTrx`) verbatim from `internal/rbac/repository/*`
(only `?`↔`$N` differ). `Storage.Close()` is a **no-op** (top-level storage owns the `*sqlx.DB`). `Seed` =
`INSERT OR IGNORE` / `ON CONFLICT DO NOTHING`.
**DoD:** SQLite integration tests: `Seed` idempotency (no clobber of an edited row), `List`/`Get`/`Update`/
`ResetToDefault`; mocks regenerated; `go build ./... && go vet ./...`.
Commit: `feat(ratelimit): add rule repository (sqlite+postgres)`.

## Phase 5 — Counter repository (dual backend)
**Goal:** `CounterRepository` with race-safe atomic increment, tested. **Depends on:** P4.
**Files:** extend `interface.go`/`adapter.go` (add `CounterRepo()` to `Storage`), add
`repository/{sqlite,postgres}/counter_repository.go`, tests, `.mockery.yaml`.
```go
type CounterRepository interface {
    IncrAndGet(ctx, targetKey, bucketKey, day string) (int, error)  // atomic upsert RETURNING count
    DeleteForTargetDay(ctx, targetKey, day string) error            // admin "reset"
    DeleteOlderThan(ctx, day string) (int64, error)                 // retention cleanup
}
```
```sql
INSERT INTO rate_limit_counters (target_key,bucket_key,day,count,updated_at) VALUES (?,?,?,1,?)
ON CONFLICT (target_key,bucket_key,day) DO UPDATE SET count = count + 1, updated_at = ?
RETURNING count;   -- postgres: rate_limit_counters.count + 1, $N placeholders
```
Read via `getExecCtx(db, opts...).QueryRowxContext(...).Scan(&n)`.
**DoD:** `IncrAndGet` returns incremented value; **concurrency test** (N goroutines on one `(target,bucket,day)`
→ final count == N); `DeleteForTargetDay`/`DeleteOlderThan`; mocks regenerated; build+vet.
Commit: `feat(ratelimit): add counter repository with atomic increment`.

---

# Service

## Phase 6 — Service core (compose + seed + cache + lifecycle)
**Goal:** `NewService` builds storage, seeds rules, exposes the rule cache, runs tickers, closes cleanly.
Not wired into the server. **Depends on:** P4, P5, P3.
**Files:** `service/service.go` (+ `service/mocks` from P4/P5).
- `NewService(ctx, db, config, log) (*Service, error)` — standard signature; `repository.NewRepo`; seed one
  `rate_limit_rules` row per `Registry` target; warm `ruleCache` synchronously; start refresh + cleanup
  tickers; construct `Handler` later (P10) — for now expose the cache.
- `ruleCache` implements `middleware.RuleProvider` (`Effective(key) (EffectiveRule, bool)`, `Reload(ctx)`):
  `SELECT *` + merge registry `Scope/Kind` with DB `enabled/limit/window`; window-unit→`time.Duration`.
- `Close()` stops both tickers (mirrors `csvc.Close()`); `LimiterMiddleware()` stub returns the mux
  middleware once P9 lands (or add in P9).
**DoD:** tests (mocked repos): seed→cache merge; window-unit→duration; ticker start/stop on `Close()`;
build+vet; nothing outside `internal/ratelimit` changed. Commit: `feat(ratelimit): add service core with seed and rule cache`.

## Phase 7 — Service admin operations
**Goal:** the service methods the admin API will call. **Depends on:** P6.
**Files:** `service/service.go` (extend).
- `ListTargets(ctx) ([]model.Target, error)` — merge registry meta + effective rule + defaults.
- `UpdateTarget(ctx, key string, patch)` — validate; persist; then `ruleCache.Reload`. Unknown key → `ErrUnknownTarget`.
- `ResetCounters(ctx, key)` — `CounterRepo().DeleteForTargetDay(today)` (+ best-effort in-memory clear once P9 exists).
- `ResetDefaults(ctx, key)` — `RuleRepo().ResetToDefault(registryDefault)`; then `Reload`.
**DoD:** method tests (mocked repos) incl. unknown-key error + reload-after-write; build+vet.
Commit: `feat(ratelimit): add service admin operations`.

---

# Middleware

## Phase 8 — In-memory store (`memStore`)
**Goal:** the fixed-window in-memory limiter core, standalone + tested. **Depends on:** none (pure).
**Files:** `middleware/limiter.go` (memStore portion) or `middleware/memstore.go`.
- `memStore.allow(key string, limit int, window time.Duration, now time.Time) (bool, time.Duration)` — the
  shortener's fixed-window design but with **limit/window passed per call** (runtime admin edits); RWMutex;
  cleanup goroutine on a ticker; `Stop()`.
**DoD:** table tests: allow up to `limit`, then reject with correct retry-after; window reset; eviction of
expired buckets; injectable `now`. Commit: `feat(ratelimit): add in-memory fixed-window store`.

## Phase 9 — Limiter middleware + clientIP + metrics
**Goal:** the hybrid `Limiter` + `mux.MiddlewareFunc`, tested in isolation. **Depends on:** P2, P3, P8
(and the `RuleProvider`/`CounterStore` interfaces satisfied by P6/P5).
**Files:** `middleware/limiter.go`, `middleware/metrics.go`.
- `RuleProvider`/`CounterStore` are **1-method interfaces declared here** (service & repo satisfy them →
  no import cycle).
- `Middleware()`: `if !cfg.Enabled → next`; `RouteBindings(method, CurrentRoute.GetPathTemplate())`; empty →
  next; per def (throttles before quotas): `Effective(key)`; disabled → skip; bucket key via
  `clientIP`/`user:`/`global`; throttle → `mem.allow(...)`, daily → `counters.IncrAndGet(...)` (error →
  `aio.ratelimit.errors` + **fail-open**); reject → `429` + `Retry-After` + `SendError`.
- `clientIP(r, cfg)` — left-most `X-Forwarded-For`/`X-Real-IP` when `trust_proxy_headers`, else
  `net.SplitHostPort(r.RemoteAddr)`.
- Metrics (`observability.Meter("ratelimit")`): `aio.ratelimit.rejected{target,scope,kind}`,
  `aio.ratelimit.errors{target}`.
**DoD:** middleware tests on a real `mux.Router` + `httptest` (validates `GetPathTemplate()` full template):
throttle/daily allow-then-429 (+`Retry-After`); disabled→pass; unknown route→pass; master off→pass; counter
error→fail-open+metric; ip/user/global keying; multi-target ordering; XFF on/off. Commit: `feat(ratelimit): add limiter middleware`.

---

# Handler (admin API)

## Phase 10 — Read API (`GET /ratelimit/targets`)
**Goal:** handler skeleton + list endpoint mounted-ready. **Depends on:** P6/P7.
**Files:** `handler/{handler.go, targets.go, metrics.go}`, `handler/mocks`.
- `Handler` + locally-declared `Service` interface + `RegisterAdminRoutes(router)` (relative paths).
- `GET /ratelimit/targets` → `ListTargets`; standard envelope; swagger godoc (`@Tags rate-limiting`,
  `@Security BearerAuth`). Wire `Handler` construction into `service.NewService`.
**DoD:** handler test (mocked service) for list happy-path + error mapping; mocks regenerated; swagger renders.
Commit: `feat(ratelimit): add admin read API for targets`.

## Phase 11 — Write API (PATCH / reset / reset-defaults)
**Goal:** the mutating admin endpoints. **Depends on:** P10, P7.
**Files:** `handler/targets.go` (extend), `handler/metrics.go`.
- `PATCH /ratelimit/targets/{key}` — pointer fields (`*bool/*int/*string`) so omitted≠zero; validate
  `window_unit ∈ {second,minute,hour,day}`, `limit ≥ 1`; `updated_by = claims.Username`; unknown key → 404.
- `POST /ratelimit/targets/{key}/reset` and `/reset-defaults`.
- Metric `aio.ratelimit.config.changed{target,action}`.
**DoD:** handler tests (mocked service): partial-PATCH pointer semantics, validation errors, unknown→404,
reset/reset-defaults, envelope. Commit: `feat(ratelimit): add admin write API for targets`.

---

# Wiring (incremental, de-risked)

## Phase 12 — Construct service + mount admin API (no enforcement yet)
**Goal:** the admin API is live; nothing is enforced. Safe, small, reversible. **Depends on:** P6, P7, P10, P11.
**Files:** `cmd/all-in-one/server/server.go`.
- Construct `rlsvc, err := ratelimitSvc.NewService(ctx, db, s.config, s.log)` after `rsvc`; `defer rlsvc.Close()`.
- `rlsvc.RegisterAdminRoutes(adminRoutes)` (already `JWTAuth + RequireAdmin`).
**DoD:** server boots; `GET/PATCH /api/v1/ratelimit/targets` reachable as admin, 403 for non-admin; **no**
endpoint is rate-limited yet. Commit: `feat(ratelimit): mount admin API (enforcement not yet wired)`.

## Phase 13 — Enforce on public routes (login / sign-up)
**Goal:** IP-keyed limits go live on `publicRoutes`. **Depends on:** P9, P12.
**Files:** `cmd/all-in-one/server/server.go`.
- `rlMw := rlsvc.LimiterMiddleware()`; `publicRoutes.Use(rlMw)`.
**DoD:** loop `POST /api/v1/sessions` → 429 + `Retry-After` after `auth.login` limit; `POST /api/v1/users` →
429 after `auth.signup.ip` limit; normal single requests unaffected. Commit: `feat(ratelimit): enforce limits on public routes`.

## Phase 14 — Enforce on gated routes + boot-time validation
**Goal:** user-keyed limits go live; drift protection added. **Highest-risk phase.** **Depends on:** P13.
**Files:** `cmd/all-in-one/server/server.go`.
- Inside `mkGated`: add `sr.Use(rlMw)` **after** `jwtMiddleware.JWTAuth` (so `auth.GetUserFromContext` is populated).
- Boot-time `r.Walk(...)` collecting registered `(method, template)`; assert every `Registry` binding matches;
  log-fatal/loud-warn on drift.
**DoD:** authed `POST /api/v1/topics/{id}/items` → 429 after quota; **regression check** — normal
listing/chat/shortener authed traffic works; boot log shows "all N targets bound" (and fails loudly on a
deliberate mismatch). Commit: `feat(ratelimit): enforce gated-route limits + boot-time validation`.

---

# Frontend (incremental)

## Phase 15 — API client + sidebar entry
**Goal:** typed client + nav link. **Depends on:** P10/P11.
**Files:** `web/src/lib/ratelimit-api.ts` (new), `web/src/components/app-sidebar.svelte`.
- `ratelimit-api.ts` (mirror `rbac-api.ts`): `BASE='/api/v1/ratelimit'`, `unwrap<T>`, `RateLimitTarget` +
  `TargetPatch` interfaces, `listTargets()`, `updateTarget(key, patch)`, `resetCounters(key)`, `resetDefaults(key)`.
- Sidebar: add `{ title: "Rate Limits", url: "/admin/ratelimit", icon: Gauge }` to `adminItems` (import
  `Gauge` from `@lucide/svelte/icons`).
**DoD:** `npm run check` + `npm run build` pass; the link shows in the Admin group for an admin. Commit: `feat(ratelimit): add web api client and sidebar entry`.

## Phase 16 — Admin page: list + toggle
**Goal:** the page renders targets and toggles enable/disable. **Depends on:** P15.
**Files:** `web/src/routes/admin/ratelimit/+page.svelte` (new).
- `onMount(load)`; `Table.Root` columns: Name (+ mono key), Scope/Kind badges (hand-rolled spans), Limit,
  Window, Enabled `Switch` (optimistic, `toggling` record, revert + `toast.error` on failure — mirror
  `admin/shortener`). Auto-guarded by `web/src/routes/admin/+layout.ts`.
**DoD:** browser check (frontend-browser-testing skill or `npm run dev`): table lists targets; flipping a
Switch persists after reload. Commit: `feat(ratelimit): add admin page list + enable toggle`.

## Phase 17 — Admin page: edit dialog + reset
**Goal:** edit limit/window and reset counters from the UI. **Depends on:** P16.
**Files:** `web/src/routes/admin/ratelimit/+page.svelte` (extend).
- Edit `Dialog`: number `Input` for limit + `Input` for window value + `Select` for unit → `updateTarget`;
  replace row on success. `AlertDialog` → `resetCounters`. Optional `reset-defaults` action.
**DoD:** browser check: edit a limit/window (persists after reload); reset a counter; toasts fire. Commit: `feat(ratelimit): add admin page edit dialog + reset`.

---

# Closeout

## Phase 18 — Metrics doc + full verification + tracker closeout
**Goal:** docs match reality; end-to-end verified on both backends. **Depends on:** all prior.
**Files:** `docs/metrics.md`, `RATE_LIMITING_PROGRESS.md`, (ADR only if an implementation deviated).
```bash
go build ./... && go test ./... && (cd web && npm run check && npm run build)
mockery
go run main.go all-in-one server     # migration 08 applies; boot-time target validation logs
```
Curl walkthrough (login throttle → 429; signup/day → 429; records/user/day → 429; then
`PATCH .../targets/auth.login {"enabled":false}` → login passes again **without restart**). Repeat with
`storage.type: postgres`.
**DoD:** full `go test ./...` green; curl walkthrough passes on SQLite **and** Postgres; `docs/metrics.md`
has a "Rate Limiting" section (`aio.ratelimit.rejected`/`.errors`/`.config.changed`, label values +
cardinality note); tracker fully checked → status done. Commit: `docs(ratelimit): metrics + verification closeout`.

---

## ⚠️ Attention (operator decisions surfaced during planning)

1. **Reverse proxy / X-Forwarded-For (important for the public deployment):** IP is keyed off `r.RemoteAddr`
   unless `ratelimit.trust_proxy_headers: true`. If the public AIO sits behind a proxy/ingress, **set it
   true** (and ensure the proxy sets `X-Forwarded-For`), else per-IP limits (login, sign-up) collapse into a
   single global bucket. Default is false for spoofing safety. *(ADR-009)*
2. **SQLite write reliability (optional, NOT included by default):** DB daily counters add per-request writes;
   the shared SQLite DSN in `internal/storage/sqlite.go` has no WAL/`busy_timeout`, so heavy concurrency can
   yield `SQLITE_BUSY` (which fail-open silently ignores). A one-line DSN change
   (`&_journal_mode=WAL&_busy_timeout=5000&_txlock=immediate`) fixes it but changes shared DB behavior. Left
   out by default. *(ADR-007)*
3. **Count-on-entry:** a rejected/failed create still consumes a daily unit; mitigated by generous defaults.
   *(ADR-006)*
4. **Admins are not exempt** in v1 (use master/per-target toggles or generous limits). *(ADR-010)*
5. **Master switch is boot-time** (`ratelimit.enabled`); per-target toggles are runtime (no restart). *(ADR-008)*

## Appendix A: Cross-cutting gotchas

1. **Reserved words** — never `limit`/`window` columns; use `limit_count`/`window_value`/`window_unit`.
2. **`GetPathTemplate()` returns the full template** incl. `/api/v1` + path vars; registry `Path` strings
   must match exactly. Boot-time `r.Walk` validation catches drift *(P14)*.
3. **Middleware order** — `rlMw` must be added AFTER `JWTAuth` on gated subrouters so user-scoped keying sees
   the user *(P14)*.
4. **Import-cycle avoidance** — sentinel errors in the top-level `ratelimit` package; handler declares its own
   `Service` interface; limiter declares its own `RuleProvider`/`CounterStore` interfaces.
5. **Atomic counter** — single `INSERT … ON CONFLICT DO UPDATE … RETURNING count`; never check-then-increment.
6. **Fail-open** — counter/store error or cache miss allows the request + emits `aio.ratelimit.errors` *(P9)*.
7. **`helpers.go` copy** — the sqlite/postgres `helpers.go` are identical except `?`↔`$N`.
8. **Squirrel is NOT used** — hand-parameterized `sqlx` SQL only (CLAUDE.md mentions it; codebase doesn't).
9. **Mock regen** — after P4/P5 (repo interfaces) and P10 (handler `Service`).
10. **Onboarding a new limited endpoint** *(future maintenance)* — add a `TargetDef` to `registry.go` with its
    `(method, path)`; ensure the owning subrouter has `rlMw`; the rule row seeds on next boot.
