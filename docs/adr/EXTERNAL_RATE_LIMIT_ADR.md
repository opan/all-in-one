# ADR: External Rate Limiting (aio as a rate-limit service)

This document records the design decisions made when extending the `internal/ratelimit` app-feature so that
**other services** (first consumer: [cashflow](https://github.com/opan/cashflow), a separate Go repo) can use
aio's limiter instead of each app growing its own. It builds on `docs/adr/RATE_LIMITING_ADR.md`, which covers
the internal (aio's-own-routes) limiter; only the *additions* are recorded here.

Implementation progress is tracked in `.context/EXTERNAL_RATE_LIMIT_PROGRESS.md`; the phased build recipe is
`.context/EXTERNAL_RATE_LIMIT_IMPLEMENTATION_PLAN.md`.

---

## ADR-E1: Two enforcement modes over one shared decision core

### Status
Accepted

### Context
The internal limiter resolves a target from the matched mux route and derives the bucket key from aio's own
JWT/IP. That model cannot serve a second service: its routes aren't in aio's mux and its users aren't in
aio's JWT.

### Decision
aio becomes the rate-limit *decision service*. A consumer `POST /api/v1/ratelimit/check` with a target key
and a **caller-supplied** bucket key; aio counts and answers. Both modes share a single counting core —
`Limiter.Check(ctx, rule, bucketKey)` — extracted from the middleware's `enforce`. `enforce` keeps everything
HTTP (route→target resolution, bucket-key derivation, the 429 write); the external handler calls `Check`
directly with the caller's bucket key.

### Rationale
- One counting implementation, two ways to feed it — no divergent enforcement logic to keep in sync.
- The refactor is behavior-preserving for the internal path (its whole test suite passed unchanged), so the
  new mode carries no regression risk to existing limits.

---

## ADR-E2: External targets are DB rows, not `Registry` entries

### Status
Accepted

### Context
Internal targets are compile-time `ratelimit.Registry` entries. The Registry's value is boot-time
route-binding drift protection (`validateRateLimitBindings` `log.Fatal`s if a target has no matching route).

### Decision
External targets are `rate_limit_rules` rows with `is_external = true` and their own `scope`/`kind`/`name`
columns; they never enter the Registry. `ruleCache.Reload` was inverted to iterate the DB rows (not the
Registry): a row with a Registry entry merges (code wins for scope/kind), an `is_external` row is built from
its own columns, and an orphan row (neither) is skipped **with a warning**.

### Rationale
- An external target has no aio route to bind to, so the Registry's drift protection buys it nothing while
  costing an aio redeploy for every consumer-side limit change.
- **Load-bearing consequence:** because `validateRateLimitBindings` walks `Registered()`, external targets
  must *never* be added to the Registry — an entry with no route would refuse the boot. This is documented at
  the `Registry` declaration. The prior `Reload` silently dropped any non-Registry row; that silent drop is
  the bug this inversion fixes (external targets would have looked configured and never enforced).

---

## ADR-E3: App tokens are SHA-256, admin-issued, and prefix-scoped

### Status
Accepted

### Context
External callers authenticate with an app token (`X-API-Key`), verified on the caller's request hot path.

### Decision
- **SHA-256, not bcrypt.** The token is a 256-bit random secret (`aio_<app>_<base64url(32 bytes)>`), looked
  up by a unique index on its hash — one indexed SELECT, constant time.
- **Admin-issued only** (aio admin / CLI), never user-issued.
- **Prefix-scoped**: a token declares a `scope_prefix` (default `<app>.`), and `/check` rejects (403) any
  target key not under it.
- Verification is cached in memory with a short TTL (`ratelimit.external.token_cache_ttl`, default 60s);
  revocation clears the cache and is enforced at the SQL level (`GetByHash` excludes revoked), so a revoked
  token stops working immediately in-process and within one TTL elsewhere.

### Rationale
- bcrypt is deliberately ~60–100ms; on a per-check hot path that is unacceptable, and its brute-force
  resistance is moot for a high-entropy secret.
- The risk is the *holder*, not the issuer: the token lives in the consumer's deployment, so a leak must not
  be able to exhaust aio's own quotas — hence prefix scoping (ADR-E4 below reinforces this).

---

## ADR-E4: `Scope` is advisory for external targets

### Status
Accepted

### Context
`/check` receives a caller-supplied bucket key. aio cannot verify it truly came from a session.

### Decision
`/check` validates only that the bucket key has the shape expected for the target's scope (`user:` / `ip:` /
`global`). This is a **bug-catcher, not a security control**, and is commented as such in the handler. The
real containment is the token's `scope_prefix` (ADR-E3), which *is* enforced.

---

## ADR-E5: Rejection is `200 allowed:false`, never 429

### Status
Accepted

### Decision
`/check` answers `200` with `{"allowed": false, ...}` on a rejection, never `429`.

### Rationale
The caller is *asking a question*, not being rate-limited itself. A `429` would conflate "your user is
limited" with "aio limited *you*", leaving the consumer unable to tell them apart. A `429` from `/check`
means only the self-protection throttle (ADR-E6) fired.

---

## ADR-E6: Self-protection via one internal target, fail-open preserved

### Status
Accepted

### Decision
- The `/check` subrouter carries the limiter middleware bound to one **internal** target,
  `ratelimit.check.ip` (ip/throttle, default 6000/min), so the endpoint cannot itself be a flood vector.
  No recursion: the middleware enforces that internal target; the handler enforces the caller's external
  target — different keys, different code paths.
- Fail-open (ADR-007) is preserved end to end: on an internal counter-store error `/check` returns
  `200 allowed:true` and meters it; the reference consumer fails open on any timeout/error and never retries
  (a retry would double-count, since `/check` increments as a side effect).
- The external API has its own master switch (`ratelimit.external.enabled`, default off) distinct from the
  platform-wide `ratelimit.enabled`, so it can be turned off without disabling aio's own limits. With it off,
  `/check` returns `503` (an unambiguous signal, not a silent allow).

---

## Known ceilings (recorded, not fixed)

- **Distributed throttle counters.** The in-memory throttle store is per-process; if aio ever scales beyond
  one instance, throttle limits multiply by replica count. Daily quotas are DB-backed and unaffected.
- **`remaining` for throttle targets** is a snapshot of a fixed-window count, not a reservation, so it is
  best-effort under concurrency. Daily-quota `remaining` is exact.
- **Metric cardinality** on `aio_ratelimit_check_total` is operator-controlled: external targets/apps are
  created at runtime, so the `target`/`app` label space is no longer bounded by the Registry (see
  `docs/metrics.md`).
- **Login throttling for consumers is deliberately out of scope.** A fail-open quota service is the wrong
  shape for a security control that would vanish exactly when aio is down; that belongs at the edge
  (e.g. Cloudflare) or as a small local throttle in the consumer.
