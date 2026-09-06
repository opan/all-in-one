# External Rate Limiting — Progress Tracker

> **Plan:** [EXTERNAL_RATE_LIMIT_IMPLEMENTATION_PLAN.md](EXTERNAL_RATE_LIMIT_IMPLEMENTATION_PLAN.md)
> **Builds on:** [RATE_LIMITING_PROGRESS.md](RATE_LIMITING_PROGRESS.md) · [docs/adr/RATE_LIMITING_ADR.md](../docs/adr/RATE_LIMITING_ADR.md)
> Live status of the build (18 phases). Tick each box, update **Resume here**, and commit after each phase.

**Overall status:** ⬜ **Not started** — plan approved, no code written.

**Resume here:** P1 (migration 10). P1/P2/P3 are independent, so any of them is a valid entry point.
P6 (the `Check` extraction) is also unblocked from the start and is the keystone — if you have appetite for
only one phase, do that one, since everything downstream depends on its shape.

Legend: ⬜ not started · 🟨 in progress · ✅ done

---

## Phase checklist

**Foundations** _(P1–P3 independent; any order)_
- ⬜ **P1 — Migration 10** · `app_tokens` + 6 columns on `rate_limit_rules` · up/down clean on SQLite+Postgres
- ⬜ **P2 — Models + errors** · `AppToken`, extended `Rule`/`Target`, `CheckRequest`/`CheckResponse`
- ⬜ **P3 — Config** · `ratelimit.external.{enabled,token_cache_ttl}` + per-key `BindEnv` + `config.yml`

**Repository** _(need P1, P2)_
- ⬜ **P4 — Token repository** · dual backend · `GetByHash` excludes revoked at SQL level · mocks
- ⬜ **P5 — Rule repo external ops** · `CreateExternal`/`Delete` · `List` selects new columns

**Service** _(P6 independent; P7 needs P5; P8 needs P4; P9 needs P5–P7)_
- ⬜ **P6 — Extract `Check` core** · pure refactor · **all existing limiter tests must pass unchanged**
- ⬜ **P7 — Rule cache union** · iterate DB rules, not `Registered()` · orphan rows warned, not silent
- ⬜ **P8 — Token service** · SHA-256 · plaintext returned once · scope prefix validated non-empty
- ⬜ **P9 — External target admin ops** · DB fallback at the 3 `ByKey` sites · `ResetDefaults` rejected

**Middleware & handlers** _(need P8; P11 needs P6,P9,P10)_
- ⬜ **P10 — App-token auth middleware** · `X-API-Key` · 401 paths · no token material in logs
- ⬜ **P11 — Check API** · 5 ordered guards · `200 allowed:false`, never 429 · fail-open on store error
- ⬜ **P12 — Token + external target admin API** · on existing `RegisterAdminRoutes` · soft revoke

**Wiring**
- ⬜ **P13 — Mount + self-protection** · `AppTokenAuth` subrouter · `ratelimit.check.ip` internal target

**CLI**
- ⬜ **P14 — Token CLI** · `token:create` / `token:list` / `token:revoke` · makes P15 non-blocking

**Frontend** _(need P12)_
- ⬜ **P15 — API client + tokens page** · plaintext shown once · sidebar → expandable
- ⬜ **P16 — External targets on `/admin/ratelimit`** · group by app · create/delete · no reset-defaults button

**Closeout**
- ⬜ **P17 — Metrics + swagger + verification + ADR** · SQLite **and** Postgres · `docs/adr/EXTERNAL_RATE_LIMIT_ADR.md`

**Consumer** _(separate repo; needs P13 deployed)_
- ⬜ **P18 — Cashflow client** · fail-open verified by killing aio, not assumed

---

## Carry-forward warnings

Three things from the plan's ATTENTION section, repeated here because they are the failure modes a resumed
session is most likely to walk into:

1. **Never bcrypt the app tokens.** `internal/auth/crypto.go` bcrypts passwords; copying that pattern puts
   ~60–100ms on cashflow's request hot path, per check. SHA-256 over a 256-bit random token — safe because
   the token is high-entropy.
2. **`ruleCache.Reload` drops unknown DB rows** ([rule_cache.go:57](../internal/ratelimit/service/rule_cache.go#L57)).
   Until P7 lands, an external target looks fully configured in the admin UI and never enforces. Silent
   failure — no error, no log.
3. **External targets must never enter `ratelimit.Registry`.** `validateRateLimitBindings` is a `log.Fatal`
   ([server.go:238](../cmd/all-in-one/server/server.go#L238)) and would refuse to boot the server, since an
   external target has no aio route to bind to.

---

## Open questions for the operator

- **Final external target list.** The plan seeds three examples (`cashflow.plan.create`,
  `cashflow.entry.create`, `cashflow.share.view.ip`) to prove the flow. Real targets and limits are an
  operator call and need no code change.
- **Where cashflow's login throttle lives.** Plan assumes Cloudflare edge rules (out of scope here). The
  alternative is a small local throttle in cashflow. Decide before P18, since it changes nothing in aio
  either way but does change what P18 has to cover.
- **`ratelimit.external.enabled` default.** Plan defaults it to `false` (explicit opt-in). Flip to `true`
  once P13 is deployed and verified, or leave off until cashflow is actually ready.

---

## Follow-ups deferred out of v1

- Per-token rate limits on `/check` (v1 uses one ip-scoped internal target instead).
- Distributed throttle counters — `memStore` is per-process, so throttle limits multiply by replica count if
  aio ever scales beyond one instance. Daily quotas are DB-backed and unaffected.
- Token rotation UX (mint-new-then-revoke-old is the manual path in v1).
- `remaining` accuracy for throttle-kind targets — the in-memory store tracks a count, not a reservation, so
  the number is a snapshot rather than a guarantee.
