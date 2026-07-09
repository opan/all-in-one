# Rate Limiting App-Feature — Implementation Plan

> **Status tracker:** [RATE_LIMITING_PROGRESS.md](RATE_LIMITING_PROGRESS.md) (live phase-by-phase status)
> **Design rationale:** [docs/adr/RATE_LIMITING_ADR.md](../docs/adr/RATE_LIMITING_ADR.md) (the *why* behind each decision)
> This is the git-tracked, portable copy of the approved plan (the plan-mode copy lives at
> `~/.claude/plans/`, which is machine-local — do not rely on it for resume).

This plan is organized into **8 sequential phases**, each with its own goal, file list, and Definition of
Done so it can be implemented, tested, and committed independently. A new chat session can resume at any
phase boundary by reading just that phase's section plus [RATE_LIMITING_PROGRESS.md](RATE_LIMITING_PROGRESS.md).

```
Phase 1 (schema + registry) → Phase 2 (repository) → Phase 3 (config)
    → Phase 4 (service + cache) → Phase 5 (limiter middleware) → Phase 6 (admin API)
    → Phase 7 (server wiring + enforcement) → Phase 8 (frontend) → verification + docs sweep
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

## Phase 1 — Schema + top-level package skeleton

**Goal:** migration `08` exists in both backends and the top-level `ratelimit` package (registry, errors,
models) compiles. Zero behavior change.
**Depends on:** nothing

**Files:**
- `db/migrations/sqlite3/08_add_rate_limit_tables.up.sql` / `.down.sql`
- `db/migrations/postgres/08_add_rate_limit_tables.up.sql` / `.down.sql`
- `internal/ratelimit/model/model.go`, `internal/ratelimit/errors.go`, `internal/ratelimit/registry.go`

Migrations auto-run on server start (`server.go`) and in `db:seed` — no runner change. Next number is **08**
(highest existing is 07).

### `db/migrations/sqlite3/08_add_rate_limit_tables.up.sql`
```sql
CREATE TABLE IF NOT EXISTS rate_limit_rules (
    target_key   TEXT PRIMARY KEY,
    enabled      INTEGER NOT NULL DEFAULT 1,   -- postgres: BOOLEAN NOT NULL DEFAULT TRUE
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
Down: `DROP INDEX idx_rate_limit_counters_day; DROP TABLE rate_limit_counters; DROP TABLE rate_limit_rules;`
**Postgres twin:** identical except `enabled BOOLEAN NOT NULL DEFAULT TRUE`.

### `registry.go` (package `ratelimit`)
`TargetDef{Key, Name, Description, Scope model.Scope, Kind model.Kind, Method, Path, DefaultEnabled,
DefaultLimit, DefaultWindowValue, DefaultWindowUnit}` + `var Registry []TargetDef` (the 6 rows above) + maps
built in `init()`: `ByKey(key)`, `RouteBindings(method, template) []TargetDef` (slice — a route may carry >1
target), `Registered()`. Mirrors `internal/rbac/features.go`.

### `model/model.go`
`Scope{ip|user|global}`, `Kind{throttle|daily_quota}` string enums; `Rule` (db row), `Counter` (db row),
`Target` (merged API view: registry meta + effective rule + defaults). All with `json`+`db` tags; `Rule`
uses `limit_count/window_value/window_unit` db tags with `json:"limit"/"window_value"/"window_unit"`.

### Definition of Done
- [ ] `go run main.go all-in-one db:migrate up` applies cleanly on a fresh SQLite DB; `db:migrate down --steps 1` reverts
- [ ] Same up/down cycle verified against Postgres
- [ ] `go build ./...` succeeds

---

## Phase 2 — Repository (dual backend)

**Goal:** `internal/ratelimit/repository` fully implemented + tested for both backends. Nothing wired yet.
**Depends on:** Phase 1

**Files:** `repository/{interface.go, factory.go, adapter.go}`,
`repository/sqlite/{storage.go, helpers.go, rule_repository.go, counter_repository.go}`,
`repository/postgres/{…same…}` (helpers identical except `?`↔`$N`).

**Interfaces:**
```go
type RuleRepository interface {
    List(ctx) ([]model.Rule, error)
    Get(ctx, targetKey string) (model.Rule, error)
    Seed(ctx, r model.Rule, opts ...query.QueryOptions) error   // insert-if-absent (no clobber)
    Update(ctx, r model.Rule, opts ...query.QueryOptions) error
    ResetToDefault(ctx, r model.Rule, opts ...query.QueryOptions) error
}
type CounterRepository interface {
    IncrAndGet(ctx, targetKey, bucketKey, day string) (int, error)  // atomic upsert RETURNING count
    DeleteForTargetDay(ctx, targetKey, day string) error            // admin "reset"
    DeleteOlderThan(ctx, day string) (int64, error)                 // retention cleanup
}
type Storage interface { RuleRepo() RuleRepository; CounterRepo() CounterRepository; CreateTrx(ctx) (query.QueryOptions, error); Close() error }
```
`Storage.Close()` is a **no-op** (top-level storage owns the shared `*sqlx.DB`). Copy the `helpers.go`
(`queryOptions{trx}`, `getExecCtx`, `createTrx`) verbatim from an existing package (e.g.
`internal/rbac/repository/sqlite/helpers.go`), changing only placeholders.

**Atomic increment** (`counter_repository.go`):
```sql
INSERT INTO rate_limit_counters (target_key, bucket_key, day, count, updated_at)
VALUES (?, ?, ?, 1, ?)
ON CONFLICT (target_key, bucket_key, day)
DO UPDATE SET count = count + 1, updated_at = ?    -- postgres: rate_limit_counters.count + 1, $N placeholders
RETURNING count;
```
Read via `getExecCtx(db, opts...).QueryRowxContext(...).Scan(&n)`. Allowed iff `n <= limit`.

### Definition of Done
- [ ] Repo integration tests against a temp SQLite DB: `Seed` idempotency (no clobber), `List`/`Get`/`Update`, `ResetToDefault`
- [ ] `IncrAndGet` returns the incremented value; **concurrency test** (N goroutines on one `(target,bucket,day)` → final count == N)
- [ ] `DeleteForTargetDay`, `DeleteOlderThan`
- [ ] `.mockery.yaml` updated (`internal/ratelimit/repository` → `internal/ratelimit/service/mocks`); mocks regenerated
- [ ] `go build ./... && go vet ./...` pass

---

## Phase 3 — Config

**Goal:** `RateLimitConfig` wired into viper with defaults + env binds + yml block.
**Depends on:** nothing (do early; Phase 4/5 consume it)

**Files:** `internal/config/config.go`, `config/config.yml`

```go
// in Config: RateLimit RateLimitConfig `mapstructure:"ratelimit"`
type RateLimitConfig struct {
    Enabled              bool          `mapstructure:"enabled"`                // master kill switch (boot-time)
    CacheRefreshInterval time.Duration `mapstructure:"cache_refresh_interval"` // default 30s
    CleanupInterval      time.Duration `mapstructure:"cleanup_interval"`       // default 1h
    CounterRetentionDays int           `mapstructure:"counter_retention_days"` // default 3
    Timezone             string        `mapstructure:"timezone"`               // default "UTC"
    TrustProxyHeaders    bool          `mapstructure:"trust_proxy_headers"`    // default false — SEE ATTENTION #1
}
```
Add a `viper.SetDefault` for each, and — because every key contains underscores — an explicit
`viper.BindEnv("ratelimit.<key>", "ALLINONE_RATELIMIT_<KEY>")` for each (the documented gotcha at
`config.go:163-186`). Add a `ratelimit:` block to `config/config.yml` with inline docs.

### Definition of Done
- [ ] Server boots with defaults; `ALLINONE_RATELIMIT_ENABLED=false` env override verified taking effect
- [ ] `go build ./...` passes

---

## Phase 4 — Service + rule cache + seed + tickers

**Goal:** `internal/ratelimit/service` composes storage, seeds rules, exposes the rule cache
(`RuleProvider`), runs cleanup/refresh tickers, and constructs the handler. Not wired into the server yet.
**Depends on:** Phase 2, Phase 3

**Files:** `service/service.go` (+ `service/mocks` from Phase 2)

- `NewService(ctx, db, config, log) (*Service, error)` — the standard app-feature signature. Builds
  `repository.NewRepo`, seeds one `rate_limit_rules` row per `Registry` target (insert-if-absent), warms the
  `ruleCache` synchronously, starts a `cache_refresh_interval` reload ticker and a `cleanup_interval`
  retention ticker, constructs the `Handler` (passing itself as a locally-declared `Service` interface to
  avoid an import cycle).
- `ruleCache` implements `middleware.RuleProvider` (`Effective(key) (EffectiveRule, bool)`): `RLock` map
  read; `Reload(ctx)` does `SELECT *` + merges registry `Scope/Kind` with the DB row's
  `enabled/limit/window` → swaps the map under `Lock`. Window unit → `time.Duration` for throttles.
- Admin-facing methods: `ListTargets`, `UpdateTarget` (persist then `Reload`), `ResetCounters`,
  `ResetDefaults`. Unknown key → `ErrUnknownTarget`.
- `Close()` stops both tickers (mirrors `csvc.Close()`/`ssvc.Close()`); `RegisterAdminRoutes(router)`
  delegates to the handler; `LimiterMiddleware()` returns the mux middleware (Phase 5).

### Definition of Done
- [ ] Service tests (mockery repo mocks): seed→cache merge; window-unit→duration; `UpdateTarget` persists then reloads; unknown key → `ErrUnknownTarget`
- [ ] Ticker goroutines start and stop cleanly on `Close()`
- [ ] `go build ./...` passes; nothing outside `internal/ratelimit` changed

---

## Phase 5 — Limiter middleware

**Goal:** the hybrid `Limiter` and its `mux.MiddlewareFunc` + OTel metrics + `clientIP` exist and are tested
in isolation.
**Depends on:** Phase 1 (registry/models), Phase 3 (config). Uses interfaces only — no `service` import.

**Files:** `middleware/limiter.go`, `middleware/metrics.go`

- `Limiter{rules RuleProvider, counters CounterStore, mem *memStore, cfg, metrics, now func() time.Time, loc *time.Location}`.
  `RuleProvider`/`CounterStore` are **1-method interfaces declared in this package** (service and repository
  satisfy them; keeps the package import-cycle-free).
- `memStore.allow(key, limit int, window time.Duration, now) (bool, time.Duration)` — the shortener's
  fixed-window design but with limit/window **passed per call** (admin edits take effect at runtime), plus a
  cleanup goroutine + `Stop()`.
- `Middleware()` flow: `if !cfg.Enabled → next`; `defs := ratelimit.RouteBindings(method, CurrentRoute.GetPathTemplate())`;
  empty → next; for each def (throttles before quotas): `Effective(key)`; disabled → skip; compute bucket key
  (`clientIP`/`user:`/`global`); throttle → `mem.allow(...)`, daily → `counters.IncrAndGet(...)` (on error →
  `aio.ratelimit.errors` + **fail-open**); on reject → `429` + `Retry-After` + `SendError`.
- `clientIP(r, cfg)` — XFF/X-Real-IP left-most when `trust_proxy_headers`, else `net.SplitHostPort(r.RemoteAddr)`.
- Metrics (`observability.Meter("ratelimit")`): `aio.ratelimit.rejected{target,scope,kind}`,
  `aio.ratelimit.errors{target}`, optional `aio.ratelimit.allowed{target}`.

### Definition of Done
- [ ] Middleware tests on a real `mux.Router` + `httptest` (also validates `GetPathTemplate()` returns the full template):
      throttle/daily allow-then-429 (+`Retry-After`); disabled→pass; unknown route→pass; master off→pass;
      counter error→fail-open + error metric; IP/user/global keying; multi-target ordering; XFF on/off (injectable `now`)
- [ ] `go build ./...` passes

---

## Phase 6 — Admin REST API (handler)

**Goal:** the admin management API exists and is testable (handler + mocked service). Not mounted yet.
**Depends on:** Phase 4 (Service contract)

**Files:** `handler/{handler.go, targets.go, metrics.go}` (+ `handler/mocks` for the `Service` interface)

`Handler` with a locally-declared `Service` interface + `RegisterAdminRoutes(router)` using **relative**
paths (the `/api/v1` prefix + admin gating come from the parent subrouter):

| Method | Relative path | Full path | Purpose |
|---|---|---|---|
| GET | `/ratelimit/targets` | `/api/v1/ratelimit/targets` | list targets (registry meta + effective rule + defaults) |
| PATCH | `/ratelimit/targets/{key}` | `.../{key}` | partial update `enabled`/`limit`/`window_value`/`window_unit` → `cache.Reload` |
| POST | `/ratelimit/targets/{key}/reset` | `.../{key}/reset` | clear today's counters (+ best-effort in-memory clear) |
| POST | `/ratelimit/targets/{key}/reset-defaults` | `.../{key}/reset-defaults` | reset rule row to code defaults |

Handlers: `ctx`+`logging.GetLoggerFromContext`; PATCH body uses pointer fields (`*bool/*int/*string`) so
omitted ≠ zero; validate `window_unit ∈ {second,minute,hour,day}` and `limit ≥ 1`; unknown `{key}` → 404
(`ErrUnknownTarget`); `updated_by = claims.Username`; standard `httpHelper.Response` envelope; swagger godoc
(`@Tags rate-limiting`, `@Security BearerAuth`). Metric `aio.ratelimit.config.changed{target,action}`.

### Definition of Done
- [ ] Handler tests (mocked service): list/patch/reset/reset-defaults, partial-PATCH pointer semantics, error→status mapping, envelope
- [ ] `.mockery.yaml` updated (`internal/ratelimit/handler` `Service` → `internal/ratelimit/handler/mocks`); mocks regenerated
- [ ] Swagger regenerated and renders the new endpoints

---

## Phase 7 — Server wiring + enforcement ("flip the switch")

**Goal:** limits are enforced end-to-end and the admin API is live. **Highest-risk phase.**
**Depends on:** Phases 4, 5, 6

**Files:** `cmd/all-in-one/server/server.go` (+ `cmd/all-in-one/db/seed.go` if a bootstrap/seed call is added)

- Construct `rlsvc, err := ratelimitSvc.NewService(ctx, db, s.config, s.log)` after `rsvc`; `defer rlsvc.Close()`.
- `rlMw := rlsvc.LimiterMiddleware()`.
- `publicRoutes.Use(rlMw)` (login/sign-up, IP-keyed — no user in context).
- Inside `mkGated`: add `sr.Use(rlMw)` **after** `jwtMiddleware.JWTAuth` so `auth.GetUserFromContext` is
  populated for user-scoped targets (gorilla runs `.Use` middleware in add-order, outermost first).
- `rlsvc.RegisterAdminRoutes(adminRoutes)` (already `JWTAuth + RequireAdmin`).
- **Boot-time route-binding validation:** after routes are registered, `r.Walk(...)` collecting registered
  `(method, template)` pairs; assert every `Registry` binding is present; log-fatal (or loud warn) on drift.

Shortener keeps its own limiter; no shortener targets in v1 → `rlMw` is a no-op there. Unregistered routes and
non-target methods pass through on a cheap map lookup.

### Definition of Done
- [ ] Server boots; boot-time validation logs "all N rate-limit targets bound" (or fails loudly on a deliberate mismatch)
- [ ] Regression check: normal login, sign-up, item/topic/chat create all work under the limits
- [ ] `go build ./...` && `go test ./internal/ratelimit/...` green

---

## Phase 8 — Frontend

**Goal:** admins manage limits at `/admin/ratelimit`.
**Depends on:** Phase 6 (API contract)

**Files:** `web/src/lib/ratelimit-api.ts` (new), `web/src/routes/admin/ratelimit/+page.svelte` (new),
`web/src/components/app-sidebar.svelte` (add `adminItems` entry)

- **`ratelimit-api.ts`** (mirror `rbac-api.ts`): `const BASE='/api/v1/ratelimit'`, `unwrap<T>`, TS
  `RateLimitTarget` + `TargetPatch` interfaces (commented as mirroring the Go model), `listTargets()`,
  `updateTarget(key, patch)` (apiPatch), `resetCounters(key)` (apiPost).
- **`/admin/ratelimit/+page.svelte`** (mirror `admin/shortener/+page.svelte`): `onMount(load)`; `Table.Root`
  with columns Name (+ mono key), Scope/Kind badges (hand-rolled spans), Limit, Window, an **Enabled
  `Switch`** per row with a `toggling` record + optimistic update + revert-on-error; edit `Dialog` (number
  `Input` + unit `Select`) → `updateTarget`; reset via `AlertDialog`. `toast` from `svelte-sonner`.
  Auto-guarded by existing `web/src/routes/admin/+layout.ts`.
- **Sidebar:** add `{ title: "Rate Limits", url: "/admin/ratelimit", icon: Gauge }` to `adminItems` (import
  `Gauge` from `@lucide/svelte/icons`). Admin group already renders only when `$auth?.is_admin === true`.

All needed shadcn-svelte components already exist under `web/src/lib/components/ui/` (table, switch, input,
button, dialog, select, alert-dialog); badges are hand-rolled spans per existing convention.

### Definition of Done
- [ ] `cd web && npm run check` and `npm run build` pass, no type errors
- [ ] Browser walkthrough (frontend-browser-testing skill or `npm run dev`): as admin, see the target table,
      flip a Switch (persists after reload), edit a limit/window in the dialog, reset a counter

---

## Verification & docs sweep (after Phase 8)

```bash
go build ./... && go test ./internal/ratelimit/...
mockery
cd web && npm run check && npm run build && cd ..
go run main.go all-in-one server     # migration 08 applies; boot-time target validation logs
```
Then exercise (see PROGRESS for the full curl script):
```bash
# login throttle
for i in $(seq 1 12); do curl -s -o /dev/null -w "%{http_code}\n" -X POST localhost:8080/api/v1/sessions \
  -H 'content-type: application/json' -d '{"username":"x","password":"y"}'; done   # 11th → 429 + Retry-After
# signup/day → 429 after 20;  records/user/day → 429 after 500 (authed cookie)
# runtime toggle (no restart):
curl -X PATCH localhost:8080/api/v1/ratelimit/targets/auth.login -d '{"enabled":false}'   # then login passes again
```

- [ ] Full backend suite green: `go build ./... && go test ./...`
- [ ] curl walkthrough passes on **both** SQLite and Postgres
- [ ] `docs/metrics.md` updated: new "Rate Limiting" section documenting `aio.ratelimit.rejected` /
      `.errors` / `.config.changed` / (optional `.allowed`) with label values + a cardinality note
- [ ] ADR (`docs/adr/RATE_LIMITING_ADR.md`) amended only if an implementation deviated from a locked decision
- [ ] [RATE_LIMITING_PROGRESS.md](RATE_LIMITING_PROGRESS.md) fully checked off; status → done

---

## ⚠️ Attention (operator decisions surfaced during planning)

1. **Reverse proxy / X-Forwarded-For (important for the public deployment):** IP is keyed off `r.RemoteAddr`
   unless `ratelimit.trust_proxy_headers: true`. If the public AIO sits behind a proxy/ingress, **set it
   true** (and ensure the proxy sets `X-Forwarded-For`), else per-IP limits (login, sign-up) collapse into a
   single global bucket. Default is false for spoofing safety. *(ADR-009)*
2. **SQLite write reliability (optional, NOT included by default):** DB daily counters add per-request writes;
   the shared SQLite DSN in `internal/storage/sqlite.go` has no WAL/`busy_timeout`, so heavy concurrency can
   yield `SQLITE_BUSY` (which fail-open silently ignores). A one-line DSN change
   (`&_journal_mode=WAL&_busy_timeout=5000&_txlock=immediate`) fixes it but changes shared DB behavior
   (adds `-wal`/`-shm` sidecar files). Left out by default. *(ADR-007)*
3. **Count-on-entry:** a rejected/failed create still consumes a daily unit; mitigated by generous defaults.
   *(ADR-006)*
4. **Admins are not exempt** in v1 (use master/per-target toggles or generous limits). *(ADR-010)*
5. **Master switch is boot-time** (`ratelimit.enabled`); per-target toggles are runtime (no restart). *(ADR-008)*

## Appendix A: Cross-cutting gotchas

1. **Reserved words** — never `limit`/`window` columns; use `limit_count`/`window_value`/`window_unit`.
2. **`GetPathTemplate()` returns the full template** incl. `/api/v1` + path vars; registry `Path` strings
   must match exactly. Boot-time `r.Walk` validation catches drift *(Phase 7)*.
3. **Middleware order** — `rlMw` must be added AFTER `JWTAuth` on gated subrouters so user-scoped keying sees
   the user *(Phase 7)*.
4. **Import-cycle avoidance** — sentinel errors in the top-level `ratelimit` package; handler declares its own
   `Service` interface; limiter declares its own `RuleProvider`/`CounterStore` interfaces.
5. **Atomic counter** — single `INSERT … ON CONFLICT DO UPDATE … RETURNING count`; never check-then-increment.
6. **Fail-open** — counter/store error or cache miss allows the request + emits `aio.ratelimit.errors` *(Phase 5)*.
7. **`helpers.go` copy** — the sqlite/postgres `helpers.go` are identical except `?`↔`$N`.
8. **Squirrel is NOT used** — hand-parameterized `sqlx` SQL only (CLAUDE.md mentions it; codebase doesn't).
9. **Mock regen** — after Phase 2 (repo interfaces) and Phase 6 (handler `Service`).
10. **Onboarding a new limited endpoint** *(future maintenance)* — add a `TargetDef` to `registry.go` with its
    `(method, path)`; ensure the owning subrouter has `rlMw`; the rule row seeds on next boot.
