# External Rate Limiting (aio as a rate-limit service) — Implementation Plan

> **Status tracker:** [EXTERNAL_RATE_LIMIT_PROGRESS.md](EXTERNAL_RATE_LIMIT_PROGRESS.md) (live phase-by-phase status)
> **Builds on:** [RATE_LIMITING_IMPLEMENTATION_PLAN.md](RATE_LIMITING_IMPLEMENTATION_PLAN.md) · [docs/adr/RATE_LIMITING_ADR.md](../docs/adr/RATE_LIMITING_ADR.md)
> **ADR:** to be written at closeout as `docs/adr/EXTERNAL_RATE_LIMIT_ADR.md` (per CLAUDE.md convention).

Extends the existing `internal/ratelimit` app-feature so **other services** (first consumer:
[cashflow](https://github.com/opan/cashflow), a separate Go repo with its own Postgres and cookie sessions)
can use aio's limiter instead of each app growing its own. Cashflow calls one HTTP endpoint; aio owns the
rules, the counters, the admin UI, and the tuning.

Sliced into **18 phases**. Each phase (a) touches a handful of files, (b) **leaves the tree green** —
`go build ./...` passes and its own tests pass — and (c) is independently committable. A fresh chat session
can resume at any phase boundary by reading that phase's section plus the tracker.

```
Foundations   P1 migration-10   P2 models+errors   P3 config          (P1–P3 independent; any order)
Repository    P4 token-repo  →  P5 rule-repo-external                 (need P1, P2)
Service       P6 check-core  →  P7 cache-union  →  P8 token-svc  →  P9 external-target-ops
Middleware    P10 app-token auth middleware                           (need P8)
Handler       P11 check-API  →  P12 token+external-target admin API   (need P6,P9,P10)
Wiring        P13 mount routes + self-protection target               (need P10,P11,P12)
CLI           P14 token:create / token:list / token:revoke            (need P8)
Frontend      P15 tokens page  →  P16 external targets on /admin/ratelimit
Closeout      P17 metrics+swagger+verification (SQLite+Postgres) + ADR
Consumer      P18 cashflow client reference (separate repo)
```

## Context

`internal/ratelimit` today enforces limits on **aio's own** routes: targets are compile-time entries in
`ratelimit.Registry`, resolved per request from the matched mux route, with the bucket key derived by aio
from its own JWT claims or the client IP. That model cannot serve a second service — its routes aren't in
aio's mux and its users aren't in aio's JWT.

Rather than duplicate the feature per app (or extract a library that would still leave each app with its own
rules table and its own admin UI), aio becomes the **rate-limit decision service**: cashflow POSTs a target
key and a bucket key, aio counts and answers.

### The two-mode model (the core framing of this feature)

| | Internal (exists today) | External (this plan) |
|---|---|---|
| Target defined in | compile-time `ratelimit.Registry` | DB row, admin-created at runtime |
| Triggered by | mux middleware on the matched route | explicit `POST /ratelimit/check` |
| Bucket key | derived by aio from JWT/IP | **supplied by the caller** |
| Auth | the end user's JWT session | app token, prefix-scoped |
| Drift protection | boot-time route-binding validation | n/a — no route to bind |
| Counted when | before the handler runs | the caller's choice |

Everything above that line diverges; the counting core below it is shared. **The single keystone refactor is
P6** — pulling the decision core out of `Limiter.enforce` so both modes call it.

### Locked decisions (confirmed with user — do not re-litigate)

1. **aio owns the whole process.** Cashflow only calls the API. No rules, no counters, no admin UI on the
   consumer side.
2. **External targets are DB rows, not `Registry` entries.** The Registry's value is boot-time route-binding
   drift protection, which external targets cannot have by definition — so the compile-time anchor buys them
   nothing, while costing an aio redeploy for every consumer-side limit change.
3. **App tokens are admin-issued** (aio admin), never user-issued.
4. **Tokens are scoped to a target-key prefix** (`cashflow.`). The risk isn't the issuer, it's the holder:
   the token lives in cashflow's deployment, so a leak must not be able to exhaust aio's own quotas.
5. **Fail-open stays the stance** (ADR-007). aio answers `allowed: true` on internal counter errors;
   cashflow fails open on timeout/error.
6. **`Scope` is advisory for external targets** — aio cannot verify a caller's bucket key really came from a
   session. It validates only that the prefix matches the declared scope. This is a bug-catcher, **not** a
   security control, and must be documented as such.
7. **Login throttling for cashflow is handled at the edge (Cloudflare)**, not here. A fail-open quota
   service is right for business quotas and weak for a security control that disappears exactly when aio is
   down. See "Deliberately out of scope".

### Initial external targets (seeded via admin UI/CLI, not code)

| Key | Scope | Kind | Suggested default | Counted by cashflow |
|---|---|---|---|---|
| `cashflow.plan.create` | user | daily_quota | 50 / day | on successful create |
| `cashflow.entry.create` | user | daily_quota | 1000 / day | on successful create |
| `cashflow.share.view.ip` | ip | throttle | 120 / minute | on each public slug view |

> These are examples to validate the flow end-to-end; final targets are an operator decision and need no
> code change to add.

---

## ATTENTION — three things that will bite

**#1 — Do NOT hash app tokens with bcrypt.** `internal/auth/crypto.go` uses bcrypt for passwords, and copying
that here would be a serious mistake: bcrypt is deliberately ~60–100ms, and this runs on cashflow's request
hot path, per check. Use **SHA-256** over a 256-bit random token. That is safe *because* the token is
high-entropy — the dictionary/brute-force threat bcrypt defends against does not apply. Look the token up by
a unique index on the hash (one indexed SELECT, constant time).

**#2 — `ruleCache.Reload` silently drops unknown rows.** It iterates `ratelimit.Registered()`
([rule_cache.go:57](../internal/ratelimit/service/rule_cache.go#L57)) and skips any DB rule without a Registry
entry. Left as-is, every external target would appear fully configured in the admin UI and **never enforce**.
P7 exists solely to fix this, and its regression test is the most important test in this plan.

**#3 — `validateRateLimitBindings` is a `log.Fatal`.** ([server.go:238](../cmd/all-in-one/server/server.go#L238))
It walks `ratelimit.Registered()`, so DB-only external targets are never reached and the Fatal cannot fire
for them — *provided external targets never enter `Registry`*. This is a load-bearing consequence of locked
decision #2: if a future change adds external targets to the Registry as a convenience, the server will
refuse to boot. Add a comment at the Registry saying so.

---

# Foundations

## Phase 1 — Migration 10 (schema only)
**Goal:** tables exist on both backends. **Depends on:** none.
**Files:** `db/migrations/{sqlite3,postgres}/10_add_app_tokens_and_external_targets.{up,down}.sql`.

```sql
-- New: service credentials for cross-app calls.
CREATE TABLE IF NOT EXISTS app_tokens (
    id           TEXT PRIMARY KEY,
    app          TEXT NOT NULL,             -- 'cashflow'
    name         TEXT NOT NULL,             -- human label, e.g. 'cashflow prod'
    token_hash   TEXT NOT NULL,             -- sha256 hex (NOT bcrypt — see ATTENTION #1)
    token_prefix TEXT NOT NULL,             -- first 12 chars, display only
    scope_prefix TEXT NOT NULL,             -- 'cashflow.' — target keys this token may touch
    created_at   TIMESTAMP NOT NULL,
    created_by   TEXT,
    last_used_at TIMESTAMP,
    revoked_at   TIMESTAMP
);
CREATE UNIQUE INDEX idx_app_tokens_hash ON app_tokens(token_hash);
CREATE INDEX idx_app_tokens_app ON app_tokens(app);

-- Extend existing rules so a row can stand alone without a Registry entry.
ALTER TABLE rate_limit_rules ADD COLUMN app         TEXT NOT NULL DEFAULT 'all-in-one';
ALTER TABLE rate_limit_rules ADD COLUMN is_external BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE rate_limit_rules ADD COLUMN name        TEXT;   -- NULL for internal (Registry wins)
ALTER TABLE rate_limit_rules ADD COLUMN description TEXT;
ALTER TABLE rate_limit_rules ADD COLUMN scope       TEXT;   -- NULL for internal (Registry wins)
ALTER TABLE rate_limit_rules ADD COLUMN kind        TEXT;
```

Use `BOOLEAN ... DEFAULT FALSE` on Postgres and `DEFAULT 0` on SQLite (the existing `08` migration is the
reference for per-backend differences). Down: drop the two indexes and `app_tokens`; for SQLite, dropping
columns needs the table-rebuild dance already used elsewhere, or accept leaving the columns (document which).

**DoD:** `db:migrate up` then `down --steps 1` clean on **both** SQLite and Postgres; existing rows keep
their defaults; `go build ./...`.
Commit: `feat(ratelimit): add migration 10 for app tokens and external targets`.

## Phase 2 — Models + errors
**Goal:** types compile; no behavior yet. **Depends on:** none.
**Files:** `internal/ratelimit/model/model.go`, `internal/ratelimit/errors.go`.

- `AppToken` struct (db+json tags; **never** serialize `token_hash` — `json:"-"`).
- Extend `model.Rule` with `App`, `IsExternal`, `Name`, `Description`, `Scope`, `Kind` (nullable pointers or
  `sql.Null*` for the four that are NULL on internal rows).
- Extend `model.Target` (the admin read view) with `App` and `IsExternal` so the UI can group and gate.
- `CheckRequest{TargetKey, BucketKey string}` / `CheckResponse{Allowed bool, RetryAfterSeconds int, Limit,
  Remaining int}`. Returning `Limit`/`Remaining` costs nothing and makes consumer-side debugging far easier.
- errors: `ErrTokenNotFound`, `ErrTokenRevoked`, `ErrTokenScopeViolation`, `ErrExternalTargetExists`,
  `ErrNotSupportedForExternal`.

**DoD:** `go build ./...`; existing ratelimit tests still pass.
Commit: `feat(ratelimit): add app token and external target models`.

## Phase 3 — Config
**Goal:** the external API has its own master switch. **Depends on:** none (do early).
**Files:** `internal/config/config.go`, `config/config.yml`.

```go
// nested inside RateLimitConfig
type ExternalConfig struct {
    Enabled        bool          `mapstructure:"enabled"`          // false — opt in explicitly
    TokenCacheTTL  time.Duration `mapstructure:"token_cache_ttl"`  // 60s; 0 disables the cache
}
```

`ratelimit.enabled` stays the platform-wide kill switch; `ratelimit.external.enabled` gates only the
`/check` endpoint and token auth, so the external API can be turned off without disabling aio's own limits.
Add `viper.SetDefault` + an explicit `viper.BindEnv("ratelimit.external.<k>", "ALLINONE_RATELIMIT_EXTERNAL_<K>")`
per key (the underscore-key gotcha at [config.go:191](../internal/config/config.go#L191)), plus the
`config.yml` block.

**DoD:** server boots with defaults; `ALLINONE_RATELIMIT_EXTERNAL_ENABLED=true` override verified.
Commit: `feat(ratelimit): add external rate limit configuration`.

---

# Repository

## Phase 4 — Token repository (dual backend)
**Goal:** `TokenRepository` implemented + tested. **Depends on:** P1, P2.
**Files:** `internal/ratelimit/repository/{interface.go,factory.go,adapter.go}`,
`repository/{sqlite,postgres}/token_repository.go`, tests, `.mockery.yaml`.

```go
type TokenRepository interface {
    Create(ctx context.Context, t model.AppToken, opts ...query.QueryOptions) error
    // GetByHash is the hot path — one indexed lookup, no logging per call
    // (same reasoning as CounterRepository.IncrAndGet).
    GetByHash(ctx context.Context, tokenHash string) (model.AppToken, error)
    List(ctx context.Context) ([]model.AppToken, error)
    Revoke(ctx context.Context, id string, opts ...query.QueryOptions) error
    TouchLastUsed(ctx context.Context, id string, at time.Time) error
}
```

Add `TokenRepo()` to `Storage` and to `storeAdapter`. `GetByHash` must exclude revoked tokens at the SQL
level (`WHERE token_hash = ? AND revoked_at IS NULL`) so a revocation takes effect without any cache flush
reasoning. `TouchLastUsed` is best-effort — never let its error fail a check.

**DoD:** SQLite integration tests for create/get/list/revoke and "revoked token is not returned by
GetByHash"; mocks regenerated; `go build ./... && go vet ./...`.
Commit: `feat(ratelimit): add app token repository (sqlite+postgres)`.

## Phase 5 — Rule repository: external target ops
**Goal:** external rules can be created, listed and deleted. **Depends on:** P4.
**Files:** `repository/interface.go`, `repository/{sqlite,postgres}/rule_repository.go`, tests.

```go
// added to RuleRepository
CreateExternal(ctx context.Context, r model.Rule, opts ...query.QueryOptions) error
Delete(ctx context.Context, targetKey string, opts ...query.QueryOptions) error
```

`List` must now also select the six new columns. `Delete` is external-only — enforce that in the service
(P9), not here, but add a `WHERE target_key = ? AND is_external = TRUE` guard as defence in depth so no code
path can delete an internal rule row. `CreateExternal` fails on duplicate key → `ErrExternalTargetExists`.

**DoD:** tests for create/list/delete round-trip, duplicate rejection, and that `Delete` refuses an internal
row; existing rule-repo tests still pass.
Commit: `feat(ratelimit): add external target CRUD to rule repository`.

---

# Service

## Phase 6 — Extract the decision core (keystone refactor)
**Goal:** the counting logic is callable without `http.ResponseWriter` or mux. **Pure refactor — no behavior
change.** **Depends on:** none (can run in parallel with P4/P5).
**Files:** `internal/ratelimit/middleware/limiter.go`, `limiter_test.go`.

Split [`Limiter.enforce`](../internal/ratelimit/middleware/limiter.go#L110) into:

```go
// Check evaluates one rule against one bucket key and reports the decision.
// It never writes a response. A counter-store error returns allowed=true
// (fail-open, ADR-007) alongside the error, for the caller to log/meter.
func (l *Limiter) Check(ctx context.Context, rule model.EffectiveRule, bucketKey string) (allowed bool, retryAfter time.Duration, err error)
```

`enforce` keeps ownership of everything HTTP: rule lookup, `bucketKey(r, scope)` derivation, the 429 write,
and the metrics. It becomes a thin wrapper over `Check`. The external handler (P11) will call `Check`
directly with a caller-supplied bucket key.

Keep `Check` on `*Limiter` (it needs `l.mem`, `l.counters` and `l.today()`); expose it from the service via
a passthrough in P9 rather than reaching into the middleware package from the handler.

**DoD:** **every existing limiter test passes unchanged** — that is the whole point of this phase. Add direct
unit tests for `Check` covering throttle-exhausted, quota-exhausted, disabled rule, and counter-error
fail-open. `go build ./...`.
Commit: `refactor(ratelimit): extract Check decision core from enforce`.

## Phase 7 — Rule cache union (see ATTENTION #2)
**Goal:** external DB targets become visible to the limiter. **Depends on:** P2, P5.
**Files:** `internal/ratelimit/service/rule_cache.go`, `rule_cache_test.go`.

Invert `Reload`: iterate the **DB rules**, not `ratelimit.Registered()`.

- rule has a Registry entry → merge, Registry supplies `Scope`/`Kind` (code wins; today's behavior).
- rule has `is_external = true` → build `EffectiveRule` from the row's own `scope`/`kind` columns.
- neither (orphan, e.g. a Registry entry deleted in code) → skip **and log a warning**. Today this is silent;
  make it visible now that "not in the Registry" is no longer automatically an error.

`model.EffectiveRule` already carries everything both modes need (`Key`, `Scope`, `Kind`, `Enabled`,
`LimitCount`, `Window`) — no new type, just a second way to populate it.

**DoD:** regression test proving an external DB row lands in the cache with the right scope/kind and window,
plus a test that an orphan row is skipped and warned. Existing cache tests unchanged.
Commit: `feat(ratelimit): load external targets into the rule cache`.

## Phase 8 — Token service
**Goal:** mint, hash, verify, scope-check. **Depends on:** P4, P3.
**Files:** `internal/ratelimit/service/token.go`, `token_test.go`.

```go
func (s *Service) CreateToken(ctx, app, name, scopePrefix, createdBy string) (model.AppToken, string, error) // returns the plaintext ONCE
func (s *Service) ListTokens(ctx) ([]model.AppToken, error)
func (s *Service) RevokeToken(ctx, id string) error
func (s *Service) VerifyToken(ctx, plaintext string) (model.AppToken, error)
```

- Token format `aio_<app>_<base64url(32 random bytes)>`, from `crypto/rand`. The app segment is a
  readability affordance only — **authorization comes from the DB row, never from parsing the string.**
- Hash with SHA-256 (ATTENTION #1). Plaintext is returned exactly once, at creation, and never stored.
- `scopePrefix` defaults to `<app>.` and is validated non-empty — a token with an empty prefix would match
  every target, defeating locked decision #4.
- Optional in-memory token cache keyed by hash with `TokenCacheTTL`, mirroring `ruleCache`. Revocation is
  enforced at the SQL level (P4), so cache TTL bounds how long a revoked token keeps working — note that in
  the ADR and keep the default low (60s). Skip the cache entirely if it complicates P8; one indexed SELECT
  per check is acceptable.

**DoD:** unit tests for round-trip verify, unknown token, revoked token, scope-prefix validation, and that
the plaintext never appears in `List` output or in the struct's JSON.
Commit: `feat(ratelimit): add app token service`.

## Phase 9 — External target admin ops
**Goal:** admin CRUD for external targets + a `Check` passthrough. **Depends on:** P5, P6, P7.
**Files:** `internal/ratelimit/service/service.go`, `service_test.go`.

```go
func (s *Service) CreateExternalTarget(ctx, t model.Target, createdBy string) (model.Target, error)
func (s *Service) DeleteExternalTarget(ctx, key string) error
func (s *Service) CheckExternal(ctx, targetKey, bucketKey string) (model.CheckResponse, error)
```

Three existing call sites gate on Registry membership via `ratelimit.ByKey` —
[service.go:213](../internal/ratelimit/service/service.go#L213) (`UpdateTarget`),
[:260](../internal/ratelimit/service/service.go#L260) (`ResetCounters`) and
[:270](../internal/ratelimit/service/service.go#L270) (`ResetDefaults`). Each needs a DB fallback for
external keys. Note the asymmetry:

- `UpdateTarget` / `ResetCounters` → work for external targets.
- `ResetDefaults` → **return `ErrNotSupportedForExternal`**. An external target has no code-defined default
  to reset to. (The alternative — storing creation-time values as the "default" — is more machinery than the
  operator value justifies; revisit if it's ever actually missed.)

`CreateExternalTarget` validates the key is not already a Registry key, reloads the cache on write (the
existing reload-on-write path), and rejects an unknown scope/kind. `CheckExternal` looks the effective rule
up in the cache, calls `Check` (P6), and translates a cache miss into fail-open + a warning.

**DoD:** table-driven tests for create/delete/update-external, `ResetDefaults` rejection, Registry-key
collision rejection, and cache reload on write.
Commit: `feat(ratelimit): add external target admin operations`.

---

# Middleware & handlers

## Phase 10 — App-token auth middleware
**Goal:** `X-API-Key` → a verified token in the request context. **Depends on:** P8.
**Files:** `internal/ratelimit/middleware/apptoken.go`, `apptoken_test.go`.

```go
func (m *AppTokenAuth) Middleware() mux.MiddlewareFunc  // 401 on missing/invalid/revoked
```

- Reads `X-API-Key`; compares via the hash lookup, not string equality against a stored secret.
- Puts `model.AppToken` in the request context for the handler's scope check.
- **Never log the token**, even truncated, on the failure path — a near-miss in logs is still a near-miss in
  logs. Log `token_prefix` only on success paths.
- 401 responses use the standard envelope via `httpHelper.SendError`.
- Fires `aio.ratelimit.token_auth_failures` (P17) with a `reason` label.

**DoD:** httptest tests for valid, missing, malformed, unknown and revoked tokens; verify no token material
reaches the log output.
Commit: `feat(ratelimit): add app token auth middleware`.

## Phase 11 — Check API
**Goal:** `POST /api/v1/ratelimit/check` works end to end. **Depends on:** P6, P9, P10.
**Files:** `internal/ratelimit/handler/check.go`, `check_test.go`.

```
POST /api/v1/ratelimit/check
X-API-Key: aio_cashflow_...
{"target_key": "cashflow.entry.create", "bucket_key": "user:42"}

200 {"success":true,"data":{"allowed":true,"limit":1000,"remaining":941,"retry_after_seconds":0}}
200 {"success":true,"data":{"allowed":false,"limit":1000,"remaining":0,"retry_after_seconds":47320}}
```

Handler order, and each step matters:

1. `ratelimit.external.enabled` off → 503 (so a kill switch is unambiguous to the caller, not a silent allow).
2. Token from context (P10) → **scope check**: `strings.HasPrefix(targetKey, token.ScopePrefix)` else 403.
   This is the enforcement point for locked decision #4.
3. Target must exist and be `is_external` → 404. An internal target is **not** callable through this API;
   allowing it would let a consumer burn aio's own quotas.
4. Bucket-key prefix must match the declared scope (`user:` / `ip:` / `global`) → 400. **Advisory only** —
   say so in the code comment so no future reader mistakes it for a security control (locked decision #6).
5. Call `CheckExternal`; on a counter-store error return **200 with `allowed: true`** (ADR-007) and meter it.

**Rejection is `200 + allowed:false`, not 429.** The caller is asking a question, not being rate-limited
itself; conflating the two makes cashflow unable to distinguish "you're limited" from "aio limited *me*".

**DoD:** httptest tests for each numbered branch above, plus a success path asserting the counter actually
incremented. Swagger annotations added.
Commit: `feat(ratelimit): add external check API`.

## Phase 12 — Token + external target admin API
**Goal:** admin CRUD over HTTP. **Depends on:** P8, P9.
**Files:** `internal/ratelimit/handler/tokens.go`, `handler/targets.go`, tests.

```
GET    /api/v1/ratelimit/tokens
POST   /api/v1/ratelimit/tokens              -> 201, plaintext token in the body ONCE
DELETE /api/v1/ratelimit/tokens/{id}         -> revoke (soft, keeps the audit row)
POST   /api/v1/ratelimit/targets/external    -> 201
DELETE /api/v1/ratelimit/targets/{key}       -> external only, 400 otherwise
```

All register on the existing `RegisterAdminRoutes`, so they inherit `JWTAuth` + `RequireAdmin` from
[server.go:218-220](../cmd/all-in-one/server/server.go#L218-L220) with no new wiring. Extend the existing
`aio.ratelimit.config_changed` metric with `token.create` / `token.revoke` / `external.create` /
`external.delete` actions rather than adding a new counter.

The create response must state plainly that the token is shown once. Revoke is soft (`revoked_at`), never a
row delete — the audit trail of what existed is the point.

**DoD:** handler tests incl. "plaintext appears in the create response and in no other response"; mocks
regenerated; swagger updated.
Commit: `feat(ratelimit): add token and external target admin API`.

## Phase 13 — Wiring + self-protection
**Goal:** routes are mounted and `/check` cannot itself be used as a flood vector. **Depends on:** P10–P12.
**Files:** `cmd/all-in-one/server/server.go`, `internal/ratelimit/registry.go`, `internal/ratelimit/service/service.go`.

- New subrouter for `/check` carrying `AppTokenAuth` (**not** `JWTAuth`, **not** `RequireAdmin`).
- Add **one internal** Registry target `ratelimit.check.ip` (ip / throttle, `POST /api/v1/ratelimit/check`,
  default 6000/minute) and put `rlMw` on that subrouter. No recursion: the middleware enforces the internal
  target, the handler enforces the external one — different keys, different paths.
- Add the comment at `Registry` recording ATTENTION #3 (external targets must never be added here).
- Register before `validateRateLimitBindings` runs so the new target's binding is seen.

**DoD:** server boots with the boot-validation log line reporting the new target count; a curl walkthrough —
mint a token via CLI, create an external target, check until rejected, verify the counter row, revoke the
token, verify 401.
Commit: `feat(ratelimit): mount external check API with self-protection`.

---

# CLI

## Phase 14 — Token CLI
**Goal:** issue tokens without the admin UI existing yet. **Depends on:** P8.
**Files:** `cmd/all-in-one/command/command.go`, `cmd/all-in-one/token/`.

```
all-in-one token:create --app cashflow --name "cashflow prod" [--scope cashflow.]
all-in-one token:list
all-in-one token:revoke <id>
```

Mirrors the existing `db:migrate` / `db:seed` command structure. `token:create` prints the plaintext once,
to stdout, with a clear "store this now, it will not be shown again" line on stderr (so `... | pbcopy`
still works cleanly).

This phase is why P15 can slip without blocking anything: the feature is fully operable from the CLI.

**DoD:** all three commands work against SQLite and Postgres; token from `token:create` authenticates
against `/check`.
Commit: `feat(ratelimit): add app token CLI commands`.

---

# Frontend

## Phase 15 — API client + tokens admin page
**Goal:** admins can mint/revoke tokens in the UI. **Depends on:** P12.
**Files:** `web/src/lib/ratelimit-api.ts`, `web/src/routes/admin/ratelimit/tokens/+page.svelte`,
`web/src/components/app-sidebar.svelte`.

Extend the existing client ([ratelimit-api.ts](../web/src/lib/ratelimit-api.ts)) with `AppToken`,
`listTokens`, `createToken`, `revokeToken`, and add `App`/`IsExternal` to `RateLimitTarget`. The sidebar
entry already exists at [app-sidebar.svelte:136](../web/src/components/app-sidebar.svelte#L136) — make
"Rate Limits" expandable with Targets/Tokens children.

The create dialog must show the plaintext **once**, with a copy button and an explicit warning; never
re-fetchable. Table shows app, name, prefix, created, last used, revoked state.

**DoD:** `npm run check` and `npm run build` clean; manual verification per the
`frontend-browser-testing` skill.
Commit: `feat(web): add app token admin page`.

## Phase 16 — External targets on the rate limits page
**Goal:** external targets are visible, creatable, deletable, grouped by app. **Depends on:** P12, P15.
**Files:** `web/src/routes/admin/ratelimit/+page.svelte`.

- Group the existing table by `app` with a section header.
- Internal rows keep today's behavior (toggle/edit/reset; scope+kind read-only, they're code-defined).
- External rows add create + delete, with scope/kind editable **at creation only**, and hide
  "reset defaults" (P9 rejects it server-side; don't offer a button that always errors).

**DoD:** `npm run check`/`build` clean; an external target created in the UI enforces on the next `/check`
without a restart (proves the P7 cache path end to end from the UI).
Commit: `feat(web): manage external rate limit targets`.

---

# Closeout

## Phase 17 — Metrics, docs, verification, ADR
**Goal:** documented and verified on both backends. **Depends on:** all.
**Files:** `docs/metrics.md`, `docs/swagger.yaml` (`swag init`), `docs/adr/EXTERNAL_RATE_LIMIT_ADR.md`.

New metrics, following the existing `aio_ratelimit_*` naming:

| Metric | Type | Labels | Description |
|---|---|---|---|
| `aio_ratelimit_check_total` | Counter | `app`, `target`, `allowed` | External check API calls |
| `aio_ratelimit_token_auth_failures_total` | Counter | `reason` | `missing`, `unknown`, `revoked`, `scope` |

Extend the existing `config_changed_total` `action` label values with the four new actions from P12, and
note in `docs/metrics.md` that `target` is no longer bounded by the Registry — external targets are
operator-created, so cardinality is now operator-controlled. That is a real monitoring caveat, not a
footnote.

Write the ADR covering: the two-mode model, DB rows vs Registry (and the Fatal consequence), SHA-256 vs
bcrypt, admin-only prefix-scoped tokens, advisory scope, `200 allowed:false` vs 429, and the deliberate
exclusion of login throttling.

**DoD:** full curl walkthrough on **SQLite and Postgres**; `docs/metrics.md` and swagger updated; ADR
written; tracker closed out.
Commit: `docs(ratelimit): document external rate limiting metrics and decisions`.

## Phase 18 — Cashflow client (separate repo)
**Goal:** cashflow enforces limits through aio. **Depends on:** P13 deployed.
**Repo:** `github.com/opan/cashflow` — **not** this one.

Roughly 100 lines, no schema change, no rules storage:

- `ratelimit.go`: a client with `http.Client{Timeout: 250ms}`, `Allow(ctx, targetKey, bucketKey) bool`.
  **Fails open** on any error, timeout, or non-200. **Never retries** — `/check` increments as a side
  effect, so a retry double-charges the quota.
- Config: `AIO_RATELIMIT_URL`, `AIO_RATELIMIT_TOKEN`, `AIO_RATELIMIT_ENABLED`. The token is a server-side
  secret and must never reach a template or the browser.
- Its own client-IP resolution (cashflow's equivalent of
  [`clientIP`](../internal/ratelimit/middleware/limiter.go#L263)) — aio only ever sees the string cashflow
  sends, so proxy-header handling cannot be delegated.
- Bucket key from its own session: `user:<id>`, else `ip:<addr>`.
- Call sites: **after** the outcome is known where that matters (count successful creates, not attempts) —
  the explicit-call model's main advantage over aio's own middleware.
- On `allowed:false`, render cashflow's own 429 using the returned `retry_after_seconds`.

**DoD:** cashflow rejects on quota exhaustion; killing aio leaves cashflow fully functional (fail-open
verified, not assumed); the token never appears in rendered HTML.

---

# Deliberately out of scope

- **Login throttling for cashflow.** A fail-open quota service is the wrong shape for a security control
  that would vanish precisely when aio is down. Cloudflare edge rules, or a ~30-line local in-memory
  throttle in cashflow. Not duplication of this feature — a safety net for the one limit that must survive
  aio being unavailable.
- **A shared `pkg/ratelimit` library.** Removes code duplication but not operational duplication: each app
  would still get its own rules table, counters and admin UI. That is the problem this plan exists to avoid.
- **Distributed throttle counters.** `memStore` is per-process
  ([memstore.go](../internal/ratelimit/middleware/memstore.go)). Fine while aio is single-instance; if aio
  ever scales out, throttle limits multiply by replica count. Daily quotas are DB-backed and unaffected.
  Record this in the ADR as a known ceiling.
- **Per-token rate limits on `/check` itself.** P13's ip-scoped `ratelimit.check.ip` is the v1 answer;
  per-token limits are the natural follow-up if a second consumer appears.
