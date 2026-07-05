# RBAC / Access-Management — Progress Tracker

**Status:** 🟢 Phase 1 complete. Phases 2-7 not started.
**Last updated:** 2026-07-05
**Full plan:** [RBAC_IMPLEMENTATION_PLAN.md](RBAC_IMPLEMENTATION_PLAN.md) (authoritative, git-tracked, phase-by-phase)
**Design rationale:** [docs/adr/ACCESS_MANAGEMENT_ADR.md](../docs/adr/ACCESS_MANAGEMENT_ADR.md)

## What this feature is
Admin-only **Access Management** (Settings > Access Managements) controlling which app-features
(listing, chat, shortener, …) each user can access. See the plan for full detail.

### Locked decisions (do NOT re-litigate — see ADR for rationale)
1. One group per user (`users.group_id` FK; NULL → `regular-user`).
2. Groups = permission presets; built-ins `admin` (superuser) + `regular-user` (default).
3. Per-user overrides are tri-state (grant AND revoke). Precedence: **admin > user override > group > default-deny**.
4. Admin = full superuser (bypasses all gates; system keeps ≥1 admin).
5. Allow-all by default **except** `admin_only` features (e.g. `access-management`). Realized via seeded data.

## Phase checklist

Each phase = one independently completable, testable, committable chunk. See the plan file for full
detail, file lists, and each phase's Definition of Done. Dependency chain:
`1 → 2 → 3 → 4 → 6 → 7`, with `5` branching off `2` (can run any time after it, in parallel with 3/4).

- [x] **Phase 1 — Database Schema Foundation.** Migration `06_add_rbac_tables` (sqlite3 + postgres, up/down) + `model.User.GroupID`. Zero behavior change — safest first commit.
  - Verified live on SQLite: full up→down→up cycle against a scratch DB (never touched `all-in-one.db`) — schema, indexes, and `PRAGMA foreign_key_check` all clean at each step.
  - Postgres twin verified by structural review only (diffed against SQLite after normalizing `BOOLEAN/FALSE`↔`INTEGER/0` — identical) — **no live Postgres server was available to test against** (no docker, no running local cluster, no passwordless sudo to start one). Re-verify against a real Postgres instance before/during Phase 7.
  - `go build ./...` and full `go test ./...` pass with no regressions.
  - Not yet committed to git.
- [ ] **Phase 2 — RBAC Core Package.** `internal/rbac/{features.go, model/, repository/, service/resolver.go, service/bootstrap.go}`. Fully unit-tested in isolation; not wired into the server yet.
- [ ] **Phase 3 — Enforcement Wiring.** `middleware/authz.go` + `RBACConfig` + `server.go` per-app gated subrouters + bootstrap call sites. ⚠️ The "flip the switch" phase — access is actually gated after this merges.
- [ ] **Phase 4 — `/users/me` Extension + Admin Management API.** `AccessResolver` wiring, `CurrentUserResponse`, full `/api/v1/access/*` CRUD + guards + OTel metrics + swagger.
- [ ] **Phase 5 — Data-Integrity Backfill.** `cmd/all-in-one/db/transfer.go` updates (group_id + 4 tables, FK order). Only depends on Phase 2.
- [ ] **Phase 6 — Frontend.** `rbac-api.ts`, `stores/auth.ts`, sidebar filtering, Access Management UI (Features/Groups/Users tabs).
- [ ] **Phase 7 — Final Verification & Docs Sweep.** Full test suite + curl walkthrough on both SQLite and Postgres; metrics.md/swagger/ADR reflect final state.

## Key gotchas captured during planning
(Full detail + phase tags in the plan's Appendix A.)
- **`model.User` needs `GroupID`** or every `SELECT * FROM users` fails. Phase 1.
- **Routes are NOT cleanly prefixed** — chat owns `/users/search`, colliding with authnz `/users`. Gate via per-app sibling subrouters, not `PathPrefix`. Phase 3.
- **Direct-auth bypass** (`x-direct-auth-username`) fabricates `UserID=username` (non-UUID, no DB row). Middleware + `/users/me` must short-circuit on `SessionID=="direct-auth"` using `rbac.direct_auth_is_admin` (default true). Phases 3 & 4.
- **SQLite down-migration** `ALTER TABLE users DROP COLUMN group_id` — verified OK on go-sqlite3 v1.14.32 (SQLite 3.50) despite the outgoing FK. Phase 1.
- **`transfer.go`** copies by explicit column/table lists — must add group_id + the 4 tables (correct FK insert order) or transfers silently drop RBAC data. Phase 5.
- **JWT stays role-free** — resolve authz from DB per request (session lookup already hits DB) → zero staleness. Phase 2/3 design constraint (see ADR-003).
- **Reserved word:** `groups` is valid unquoted in both SQLite & Postgres; fallback `access_groups` if ever needed. Phase 1.

## Docs status
- [x] ADR written: [docs/adr/ACCESS_MANAGEMENT_ADR.md](../docs/adr/ACCESS_MANAGEMENT_ADR.md) (7 decisions — group model, precedence, JWT-role-free, admin superuser/lockout, default-allow-via-data, per-app subrouter enforcement, code-defined feature registry)
- [x] Implementation plan written and split into phases: [RBAC_IMPLEMENTATION_PLAN.md](RBAC_IMPLEMENTATION_PLAN.md)
- [x] This progress tracker
- [ ] `docs/metrics.md` update (Phase 4)
- [ ] Swagger regen (Phase 4)

## Resume instructions
- **Same machine / new session:** open [RBAC_IMPLEMENTATION_PLAN.md](RBAC_IMPLEMENTATION_PLAN.md), find
  the first unchecked phase, read only that phase's section.
- **Another machine:** commit & push `.context/RBAC_*.md` + `docs/adr/ACCESS_MANAGEMENT_ADR.md` (+ any
  completed phase code); pull on the other machine; a fresh Claude Code session reads these git-tracked
  files (the `~/.claude/plans/` copy does NOT travel — it's machine-local).
- **Mid-phase handoff:** if a phase is partially done, note which files are finished under that phase's
  checklist item before pausing, so the next session doesn't redo work.
