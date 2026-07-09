# ADR: Rate Limiting (app-feature)

This document records design decisions made when adding the admin-only **Rate Limiting** app-feature — a
platform-wide capability that caps traffic to selected endpoints (sign-ups/day, records/user/day, login
attempts, …) so the resource-constrained homelab is not flooded before a public announcement. Add a new
entry here for any future change that touches rate limiting.

Implementation status is tracked separately in `.context/RATE_LIMITING_PROGRESS.md`; this document records
the *decisions*, not build progress. The phased build recipe lives in
`.context/RATE_LIMITING_IMPLEMENTATION_PLAN.md`.

---

## ADR-001: Rate limiting as an admin-only app-feature with DB-backed config, not a config-file limiter

### Status
Accepted

### Context
An in-memory rate limiter already exists at `internal/shortener/middleware/ratelimit.go`, but it is scoped
to the shortener, is configured only from `config.yml` (read once at construction), and is not persisted or
runtime-editable. The requirement is a *platform-wide* capability whose limits an admin can enable/disable
and tune **at runtime** (before a public launch, the operator needs to react to traffic without a redeploy).

### Decision
Add a new `internal/ratelimit` app-feature that mirrors the `internal/rbac` (access-management) app-feature:
rate-limit *rules* are persisted in the database (admin-editable), the limiter is a middleware
dependency-injected in `cmd/all-in-one/server/server.go`, and its management API mounts on the existing
`RequireAdmin` subrouter. It is an "app-feature" (cross-cutting, admin-only, provides middleware) rather than
a user-facing "app".

### Rationale
- The `rbac` app-feature is a proven, near-identical structural template (admin-only, cross-cutting,
  provides middleware wired in `server.go`, repository→service→handler with dual SQLite/Postgres backends),
  so this reuses established conventions rather than inventing new ones.
- Persisting rules in the DB (not `config.yml`) is what makes runtime enable/disable + tuning possible; the
  master on/off switch and operational knobs stay in config (ADR-008).

### Alternatives Considered
| Option | Rejected because |
|---|---|
| Extend the existing shortener limiter | It is app-scoped, config-only, and not persisted; generalizing it in place would still leave limits un-editable at runtime and wouldn't give a management API. It is instead reused as the *algorithmic* template for the in-memory throttle (ADR-002). |
| Config-file-only limits (viper) | Every limit change would require an edit + redeploy/restart — the opposite of the "react to traffic quickly" requirement. |

### Consequences
- The shortener keeps its own existing limiter for now; no shortener targets are registered in v1, so the
  new middleware is a no-op on shortener routes. Unifying them is possible future work, not part of this.
- Onboarding a new limited endpoint is a small, centralized code change (ADR-003) plus runtime config.

### Key files (planned)
- `internal/ratelimit/**`, `cmd/all-in-one/server/server.go` (wiring)
- reused template: `internal/rbac/**`; reused algorithm: `internal/shortener/middleware/ratelimit.go`

---

## ADR-002: Hybrid counter storage — in-memory for burst throttles, DB for daily quotas

### Status
Accepted

### Context
The requirements span two very different traffic shapes: (a) burst/flood protection (login attempts,
per-IP bursts) which is high-frequency and is exactly what a DDOS drives, and (b) daily quotas (sign-ups/day,
records/user/day) which are low-frequency but must be meaningful across restarts/deploys. Where the request
*counters* live materially affects both correctness and whether the limiter protects or harms the server.

### Decision
Counter storage is chosen per target `Kind`: `throttle` targets count **in-memory** (fixed-window map,
reusing the shortener limiter's design); `daily_quota` targets count in a **DB table**
(`rate_limit_counters`). Rule *config* is always in the DB regardless (ADR-001/ADR-008).

### Rationale
- In-memory counting keeps the DB entirely off the hot path during a flood — a cheap map lookup sheds the
  attack, which is the whole point of DDOS protection. Throttle windows are short, so a restart-reset is
  acceptable.
- DB-backed daily counters survive restarts/deploys, so "20 sign-ups/day" or "500 items/user/day" cannot be
  reset by simply bouncing the server. Daily creates are low-frequency, so the per-request write is cheap.
- Throttle targets are evaluated **before** quota targets on a shared route, so an in-memory throttle sheds a
  flood before any DB counter write happens.

### Alternatives Considered
| Option | Rejected because |
|---|---|
| All counters in-memory | Daily quotas would reset on every restart/deploy — weak against a determined abuser and inconsistent with the primary stated goals (sign-ups/day, records/day). |
| All counters in the DB | A flood on a public endpoint would hammer the DB on every request, making the database the bottleneck the attacker targets — the limiter would *cause* the outage it's meant to prevent. |

### Consequences
- Two counter code paths (a `memStore` and a `CounterStore`) exist behind one `Limiter`.
- In-memory throttle state is per-process and non-durable by design; if the app ever runs multi-instance,
  throttles become per-instance (documented limitation; daily quotas remain correct as they're DB-backed).

### Key files (planned)
- `internal/ratelimit/middleware/limiter.go` (memStore + dispatch), `internal/ratelimit/repository/*/counter_repository.go`

---

## ADR-003: Named target registry synced to DB rules, not admin-authored path rules

### Status
Accepted

### Context
The admin must choose *which* endpoints are limited, and the operator anticipates "many undiscovered use
cases." Two models: a code-defined registry of named targets, or free-form admin-authored rules keyed on
HTTP method + path.

### Decision
`internal/ratelimit/registry.go` holds a Go-level `Registry` of `TargetDef` (key, name, description, scope,
kind, the route binding, and defaults) as the single source of truth — the same idiom as
`internal/rbac/features.go`. On boot, one `rate_limit_rules` row is seeded per target (insert-if-absent,
never clobbering admin edits). The admin toggles/tunes each target; onboarding a new endpoint = add a
`TargetDef` (+ ensure its subrouter carries the middleware).

### Rationale
- A rate-limit target is only meaningful if code actually enforces it at a real route — an admin-created
  rule pointing at a path with no enforcement point would be a UI that limits nothing (the same reasoning
  that kept RBAC features code-defined, ADR-007 of the access-management ADR).
- Named targets give typo-proof, correct per-scope keying (each target declares whether it counts per IP,
  per user, or globally — ADR-005) and friendly admin labels.
- Seed-on-boot means adding a target needs no data migration; it appears configured with code defaults on
  first boot of the new binary.

### Alternatives Considered
| Option | Rejected because |
|---|---|
| Admin-authored `method + path` rules in the UI | Fragile (path typos, `{path_var}` templates), per-user keying is awkward to wire generically, and a rule can silently enforce nothing. |
| Both (registry + ad-hoc path rules) | Extra surface and two enforcement paths to build/test for no v1 requirement; can be added later if a real need appears. |

### Consequences
- The admin "targets" list is registry-driven; the DB row supplies the current enabled/limit/window overlay.
- Bumping a *code default* for an already-seeded, untouched target does not auto-propagate; a per-target
  `reset-defaults` admin action pulls new code defaults on demand (mirrors RBAC's non-clobbering upsert).

### Key files (planned)
- `internal/ratelimit/registry.go`, `internal/ratelimit/service/service.go` (seed), `db/migrations/*/08_add_rate_limit_tables.*`

---

## ADR-004: Enforcement via a route-resolving middleware, not call-site `Guard` wrapping

### Status
Accepted

### Context
The endpoints to be limited live in other packages: login/sign-up in `authnz`, record creation in `listing`
and `chat`. Enforcement can attach either by wrapping each handler at its call site
(`limiter.Guard("auth.login", h.CreateSession)`) or by a subrouter middleware that resolves the target from
the matched route.

### Decision
Use one `mux.MiddlewareFunc` applied to the subrouters that carry limited routes (`publicRoutes` and the
`mkGated(...)` subrouters). Per request it looks up the target(s) bound to the matched route via
`mux.CurrentRoute(r).GetPathTemplate()` + `r.Method`; no match → pass through. A boot-time `r.Walk`
validation asserts every registry route-binding matches a registered route.

### Rationale
- This matches the codebase's existing gating idiom exactly — `server.go` already builds `mkGated(feature)`
  subrouters with `sr.Use(authz.RequireFeature(feature))`; rate limiting is the same shape.
- Blast radius is tiny: only `server.go` + the new `ratelimit` package change. `Guard` at call sites would
  change `NewHandler`/`NewService` signatures for three packages (authnz, listing, chat) and add an import
  edge from each into the limiter.
- Applying the middleware **after** `jwtMiddleware.JWTAuth` on gated subrouters means `auth.GetUserFromContext`
  is populated, so user-scoped targets (ADR-005) can key by user id.

### Alternatives Considered
| Option | Rejected because |
|---|---|
| `limiter.Guard(key, handlerFunc)` at each call site | Larger blast radius (3 packages' constructors + import edges), and diverges from the established subrouter-gating pattern. Its one advantage — no route-binding drift — is recovered by the boot-time validation below. |

### Consequences
- **Drift risk:** renaming a route without updating its `TargetDef.Path` would silently stop enforcement.
  Mitigated by the boot-time `r.Walk` validation, which turns a silent failure into a loud one at startup.
- `GetPathTemplate()` returning the full template (incl. `/api/v1` + path vars) is relied upon and is
  covered by a middleware test.

### Key files (planned)
- `cmd/all-in-one/server/server.go`, `internal/ratelimit/middleware/limiter.go`, `internal/ratelimit/registry.go`

---

## ADR-005: Per-rule natural key (IP / user / global); one global config, not per-user or multi-tenant

### Status
Accepted

### Context
The feature is described as "not scoped to users, but to the whole AIO," yet one requirement — "records each
user can create per day" — is inherently per-user. These are reconciled by separating *configuration scope*
from *counting key*.

### Decision
There is one global rule set (no per-user custom limits, not multi-tenant). Each target declares a `Scope`
that determines its counting key: `ip` (e.g. login, sign-up — keyed on client IP), `user` (e.g.
records/user/day — keyed on the authenticated user id), or `global` (a single shared counter). The bucket
key is `"ip:<addr>"`, `"user:<id>"`, or `"global"` — matching the existing shortener limiter's user→IP
fallback convention.

### Rationale
- "Config is global, counting is per-rule" is the only reading that satisfies both "whole-AIO feature" and
  "records per *user* per day" simultaneously.
- Per-scope keying is a property of the code-defined target (ADR-003), so it is correct by construction and
  not something an admin can misconfigure.

### Alternatives Considered
| Option | Rejected because |
|---|---|
| Everything a single global counter | Cannot express "per user" — one heavy user would exhaust a shared record quota for everyone, and a shared login counter would let one abuser lock out all users. |
| Per-user configurable limits / multi-tenant | Not required; adds significant config and storage surface for no stated need. |

### Consequences
- IP-scoped correctness depends on resolving the real client IP behind a proxy (ADR-009).
- User-scoped targets only apply on authenticated routes (the middleware runs after JWT auth there).

### Key files (planned)
- `internal/ratelimit/middleware/limiter.go` (bucket-key derivation), `internal/ratelimit/registry.go`

---

## ADR-006: UTC calendar-day quota buckets, counted on entry

### Status
Accepted

### Context
Daily quotas need a definition of "a day" (rolling 24h vs calendar day; which timezone) and a point at which
a request is counted (before or after the handler runs).

### Decision
A daily-quota bucket is a `'YYYY-MM-DD'` string in a configured timezone (default **UTC**), resetting at
midnight. Requests are counted **on entry** (in the middleware, before the handler), via a single race-safe
`INSERT … ON CONFLICT (target_key,bucket_key,day) DO UPDATE SET count = count + 1 RETURNING count`.

### Rationale
- UTC matches the codebase's `time.Now().UTC()` convention everywhere and avoids DST edge cases; the tz is a
  config knob for operators who want local-day semantics.
- A calendar-day bucket needs no per-key "first seen" state (unlike a rolling 24h window) — the day string
  *is* the window identity.
- Count-on-entry is correct for abuse prevention (you want to count *attempts*, including failed ones) and is
  simple; the single atomic upsert is the only race-safe form under concurrency.

### Alternatives Considered
| Option | Rejected because |
|---|---|
| Rolling 24h window | Requires tracking each key's first-hit timestamp and expiry; unnecessary state for a daily quota. |
| Count only successful (2xx) requests | Needs a post-handler status-capturing `ResponseWriter` and accepts a concurrent overshoot; count-on-entry with generous limits is simpler and more abuse-resistant. Revisit if a create endpoint needs success-only accounting. |

### Consequences
- A rejected/failed create (e.g. a 400) still consumes a daily unit — mitigated by setting generous default
  limits.
- Old counter rows are pruned by a retention ticker (`counter_retention_days`, default 3).

### Key files (planned)
- `internal/ratelimit/repository/*/counter_repository.go` (atomic upsert), `internal/ratelimit/middleware/limiter.go`

---

## ADR-007: Fail-open on counter/store errors

### Status
Accepted

### Context
DB-backed daily counters introduce a failure mode the in-memory shortener limiter never had: the counter
query can error (e.g. SQLite `database is locked` under contention, or a transient DB outage). The limiter
must decide whether an error blocks (fail-closed) or allows (fail-open) the request.

### Decision
On any counter-store error or rule-cache miss, **allow** the request (fail-open) and increment an
`aio.ratelimit.errors` counter so the condition is observable. The in-memory throttle path has no DB
dependency and is unaffected.

### Rationale
- For a homelab pre-launch, availability of legitimate traffic outranks strict quota enforcement — a
  database hiccup should not 429 real users.
- Emitting a metric keeps the fail-open visible rather than silent, so the operator can alarm on it.

### Alternatives Considered
| Option | Rejected because |
|---|---|
| Fail-closed (deny on error) | A transient DB error would reject legitimate users wholesale; worse operator experience than a briefly-unenforced quota. |

### Consequences
- A sustained DB outage disables *daily-quota* enforcement (throttles still work, being in-memory).
- An optional SQLite DSN hardening (`_journal_mode=WAL&_busy_timeout=5000&_txlock=immediate` in
  `internal/storage/sqlite.go`) would make `SQLITE_BUSY`-driven fail-opens rare; left out by default as it
  changes shared DB behavior (see IMPLEMENTATION_PLAN "attention" notes).

### Key files (planned)
- `internal/ratelimit/middleware/limiter.go`, `internal/ratelimit/middleware/metrics.go`

---

## ADR-008: DB rules + in-memory cache reloaded on write; master switch is boot-time

### Status
Accepted

### Context
Rule *config* lives in the DB (ADR-001), but reading it from the DB on every request would add DB load to
the hot path the limiter is meant to protect. Separately, admin edits must take effect without a restart.

### Decision
Effective rules are held in an in-memory cache (`RuleProvider`), seeded synchronously at boot, reloaded on
every admin write (PATCH/reset-defaults) and on a periodic ticker (`cache_refresh_interval`). So per-target
changes are effective at runtime with no restart. The master on/off switch (`ratelimit.enabled`) is a
**boot-time** config value.

### Rationale
- Only the daily *counter* must touch the DB per request; the *rule* config is read-mostly and cheap to
  cache, keeping config reads off the hot path.
- Reload-on-write gives immediate admin feedback (toggle a target off → next request passes) without the
  staleness of a redeploy.
- A master kill switch is an ops-level action where a restart is acceptable; making it boot-time keeps the
  hot-path check a single boolean with no per-request lookup.

### Alternatives Considered
| Option | Rejected because |
|---|---|
| Read rule config from DB per request | Unnecessary DB load on exactly the endpoints under flood; defeats part of ADR-002. |
| Everything (incl. per-target limits) in config.yml | Needs a redeploy for every tuning change — the problem ADR-001 exists to solve. |

### Consequences
- The cache must be invalidated on every rule write (a `Reload`), which the admin handlers call after
  persisting.
- If a runtime *global* kill (no restart) is ever needed, the cheapest addition is a bulk "disable all
  targets" admin action (no schema change); not built in v1.

### Key files (planned)
- `internal/ratelimit/service/service.go` (cache + reload), `internal/config/config.go`, `config/config.yml`

---

## ADR-009: Client-IP resolution is opt-in proxy-aware (`trust_proxy_headers`, default false)

### Status
Accepted

### Context
The AIO is deployed publicly and is likely behind a reverse proxy/ingress. Nothing in the codebase parses
`X-Forwarded-For` today (`r.RemoteAddr` is used raw). Behind a proxy, every request shares the proxy's IP, so
IP-scoped limits (login, sign-up — ADR-005) would collapse into a single global limit.

### Decision
Add a `clientIP(r, cfg)` helper: when `ratelimit.trust_proxy_headers` is true, take the left-most
`X-Forwarded-For` entry (falling back to `X-Real-IP`); otherwise use `net.SplitHostPort(r.RemoteAddr)`. The
flag defaults to **false**.

### Rationale
- Trusting client-supplied `X-Forwarded-For` unconditionally is spoofable — an attacker could rotate the
  header to dodge an IP limit. It must only be trusted when a sanitizing edge proxy is known to set it, which
  only the operator can assert — hence an explicit opt-in defaulting to off.

### Alternatives Considered
| Option | Rejected because |
|---|---|
| Always trust `X-Forwarded-For` | Trivially spoofable; would make per-IP limits worthless against a motivated abuser. |
| Never parse proxy headers | Per-IP limits silently collapse to a single bucket behind any proxy — a correctness trap for the actual deployment. |

### Consequences
- **The operator must set `ratelimit.trust_proxy_headers: true` in the public deployment** (and ensure the
  proxy sets `X-Forwarded-For`) for per-IP limits to work. Documented prominently in the implementation plan.
- IP extraction ideally belongs at server scope (request logging would benefit too); scoped to the limiter
  now, unifiable later.

### Key files (planned)
- `internal/ratelimit/middleware/limiter.go` (`clientIP`), `internal/config/config.go`

---

## ADR-010: Admins are subject to limits in v1 (no exemption)

### Status
Accepted

### Context
The operator is an admin and will create records and log in like any user. Admins could be exempted from
user-scoped quotas, but exemption requires the limiter to know admin status (a dependency on the RBAC
resolver) and makes limits harder to test.

### Decision
v1 does **not** special-case admins — everyone is subject to the same limits. The operator manages their own
experience via generous default limits, per-target runtime toggles, and the master switch.

### Rationale
- Keeps the limiter free of a dependency on the RBAC resolver (CLAUDE.md: keep dependencies minimal) and
  makes every limit testable in dev without a second account.
- Server-side seeding (`db:seed`) does not go through HTTP, so bulk data creation is not rate-limited anyway.

### Alternatives Considered
| Option | Rejected because |
|---|---|
| Exempt admins from user-scoped quotas | Adds an RBAC-resolver dependency + a per-request `IsAdmin` check; not needed given generous limits + toggles. Left as a documented future option via an injected `isExempt func(ctx, userID) bool` wired to `rsvc.Resolver.IsAdmin`. |

### Consequences
- If the admin hits a limit while testing/seeding via the API, they disable the target or raise the limit at
  runtime.
- Adding admin exemption later is additive (an optional injected predicate) and needs no schema change.

### Key files (planned)
- `internal/ratelimit/middleware/limiter.go` (future `isExempt` hook), `cmd/all-in-one/server/server.go`
