# Rate Limiting App-Feature — Progress Tracker

> **Plan:** [RATE_LIMITING_IMPLEMENTATION_PLAN.md](RATE_LIMITING_IMPLEMENTATION_PLAN.md) · **Decisions:** [docs/adr/RATE_LIMITING_ADR.md](../docs/adr/RATE_LIMITING_ADR.md)
> Live status of the build. Update the checkboxes and the "Resume here" pointer as work proceeds.

**Overall status:** 🟡 Planning complete — awaiting user review of ADR + plan before Phase 1.

**Resume here:** Phase 1 (schema + registry skeleton). Nothing implemented yet.

---

## Phase checklist

- [ ] **Phase 1 — Schema + package skeleton**
  - [ ] `08_add_rate_limit_tables.{up,down}.sql` (sqlite3 + postgres)
  - [ ] `model/model.go`, `errors.go`, `registry.go` (6 initial targets)
  - [ ] migrate up/down verified on SQLite + Postgres; `go build ./...`
- [ ] **Phase 2 — Repository (dual backend)**
  - [ ] `interface.go`, `factory.go`, `adapter.go`
  - [ ] `sqlite/` + `postgres/` (storage, helpers, rule_repository, counter_repository)
  - [ ] atomic `IncrAndGet` (`ON CONFLICT … RETURNING count`)
  - [ ] repo tests incl. seed-idempotency + concurrency test; `.mockery.yaml` + regen
- [ ] **Phase 3 — Config**
  - [ ] `RateLimitConfig` + defaults + per-key `BindEnv` + `config.yml` block
- [ ] **Phase 4 — Service + cache + seed + tickers**
  - [ ] `NewService`, seed-on-boot, `ruleCache` (RuleProvider), refresh + cleanup tickers, `Close`
  - [ ] service tests (mocked repos)
- [ ] **Phase 5 — Limiter middleware**
  - [ ] `limiter.go` (+ `memStore`, `clientIP`), `metrics.go`
  - [ ] middleware tests (mux + httptest; throttle/daily/disabled/fail-open/keying/XFF)
- [ ] **Phase 6 — Admin REST API**
  - [ ] `handler.go`, `targets.go`, `metrics.go`, swagger godoc
  - [ ] handler tests; `.mockery.yaml` (handler `Service`) + regen
- [ ] **Phase 7 — Server wiring + enforcement**
  - [ ] construct `rlsvc` + `defer Close`; `publicRoutes.Use(rlMw)`; `sr.Use(rlMw)` in `mkGated`
  - [ ] `RegisterAdminRoutes(adminRoutes)`; boot-time `r.Walk` route-binding validation
  - [ ] end-to-end enforcement + regression check
- [ ] **Phase 8 — Frontend**
  - [ ] `ratelimit-api.ts`, `/admin/ratelimit/+page.svelte`, sidebar entry
  - [ ] `npm run check` + `npm run build`; browser walkthrough
- [ ] **Verification & docs sweep**
  - [ ] full `go test ./...` green; curl walkthrough on SQLite **and** Postgres
  - [ ] `docs/metrics.md` "Rate Limiting" section added
  - [ ] ADR amended only on deviation; this tracker fully checked → status done

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

- [ ] Confirm whether the public deployment is behind a reverse proxy → if yes, set
      `ratelimit.trust_proxy_headers: true` (else per-IP limits collapse). *(ADR-009)*
- [ ] Decide whether to include the optional SQLite WAL/`busy_timeout` DSN hardening. *(ADR-007)*

## Notes / deviations

_(record any implementation deviation from a locked decision here, then amend the ADR.)_
