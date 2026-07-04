# RBAC / Access-Management — Progress Tracker

**Status:** 🟡 Not started (planning complete & approved). Working tree is clean — no code written yet.
**Last updated:** 2026-07-04
**Full plan:** [RBAC_IMPLEMENTATION_PLAN.md](RBAC_IMPLEMENTATION_PLAN.md) (authoritative, git-tracked)

## What this feature is
Admin-only **Access Management** (Settings > Access Managements) controlling which app-features
(listing, chat, shortener, …) each user can access. See the plan for full detail.

### Locked decisions (do NOT re-litigate)
1. One group per user (`users.group_id` FK; NULL → `regular-user`).
2. Groups = permission presets; built-ins `admin` (superuser) + `regular-user` (default).
3. Per-user overrides are tri-state (grant AND revoke). Precedence: **admin > user override > group > default-deny**.
4. Admin = full superuser (bypasses all gates; system keeps ≥1 admin).
5. Allow-all by default **except** `admin_only` features (e.g. `access-management`). Realized via seeded data.

## Checklist (implementation order)
- [ ] 1. Migration `06_add_rbac_tables` — sqlite3 + postgres, up + down (features, groups, group_features, user_feature_overrides, `users.group_id`)
- [ ] 2. `model.User.GroupID *uuid.UUID` field (**CRITICAL** — `SELECT *` breaks app-wide without it)
- [ ] 3. `internal/rbac/` package: `features.go` registry, `model/`, `repository/` (interface/factory/sqlite/postgres)
- [ ] 4. `service/resolver.go` (precedence) + `service/bootstrap.go` (idempotent)
- [ ] 5. `middleware/authz.go` (RequireFeature/RequireAdmin) + `service.go` (mgmt + guards) + `handler/` (API + metrics)
- [ ] 6. Wire into `cmd/all-in-one/server/server.go` (per-app gated subrouters + Bootstrap + resolver injection)
- [ ] 7. Extend `/users/me` (AccessResolver interface + CurrentUserResponse + direct-auth guard)
- [ ] 8. `RBACConfig` in `internal/config/config.go` + `config/config.yml`; call Bootstrap in `cmd/all-in-one/db/seed.go`
- [ ] 9. Update `cmd/all-in-one/db/transfer.go` (users.group_id + 4 tables, FK order) + regenerate mocks (`.mockery.yaml`)
- [ ] 10. Backend tests (resolver matrix, middleware, repository, handler guards, bootstrap idempotency)
- [ ] 11. Frontend: `rbac-api.ts`, `stores/auth.ts`, `/users/me` types, sidebar filtering, Access Management section + components
- [x] 12a. ADR written: [docs/adr/ACCESS_MANAGEMENT_ADR.md](../docs/adr/ACCESS_MANAGEMENT_ADR.md) (7 decisions — group model, precedence, JWT-role-free, admin superuser/lockout, default-allow-via-data, per-app subrouter enforcement, code-defined feature registry)
- [ ] 12b. Docs: metrics.md update + swagger regen (once handlers exist)
- [ ] 13. End-to-end verification (build, tests, run server, enforcement flow, postgres path)

## Key gotchas captured during planning
- **`model.User` needs `GroupID`** or every `SELECT * FROM users` fails (`missing destination name group_id`). Do this with the migration.
- **Routes are NOT cleanly prefixed** — chat owns `/users/search`, colliding with authnz `/users`. Gate via per-app sibling subrouters, not `PathPrefix`.
- **Direct-auth bypass** (`x-direct-auth-username`) fabricates `UserID=username` (non-UUID, no DB row). Middleware + `/users/me` must short-circuit on `SessionID=="direct-auth"` using `rbac.direct_auth_is_admin` (default true).
- **SQLite down-migration** `ALTER TABLE users DROP COLUMN group_id` — verified OK on go-sqlite3 v1.14.32 (SQLite 3.50) despite the outgoing FK. Drop indexes first, then column, then tables child→parent.
- **`transfer.go`** copies by explicit column/table lists — must add group_id + the 4 tables (correct FK insert order) or transfers silently drop RBAC data.
- **JWT stays role-free** — resolve authz from DB per request (session lookup already hits DB) → zero staleness.
- **Reserved word:** `groups` is valid unquoted in both SQLite & Postgres; fallback `access_groups` if ever needed.

## Resume instructions
- **Same machine / new session:** open [RBAC_IMPLEMENTATION_PLAN.md](RBAC_IMPLEMENTATION_PLAN.md), start at the first unchecked item.
- **Another machine:** `git add .context/RBAC_*.md` (+ any committed code), commit & push; pull on the other machine; a fresh Claude Code session reads these `.context/` files (the `~/.claude/plans/` copy does NOT travel — it's machine-local).
