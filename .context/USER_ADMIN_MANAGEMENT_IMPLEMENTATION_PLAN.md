# User & Admin Management — Implementation Plan

**Design rationale:** [docs/adr/ACCESS_MANAGEMENT_ADR.md](../docs/adr/ACCESS_MANAGEMENT_ADR.md) (ADR-008)
**Progress:** [USER_ADMIN_MANAGEMENT_PROGRESS.md](USER_ADMIN_MANAGEMENT_PROGRESS.md)
**Builds on:** the shipped RBAC feature ([RBAC_IMPLEMENTATION_PLAN.md](RBAC_IMPLEMENTATION_PLAN.md)).

## Context
The RBAC feature added an admin-only **Access Management** screen, but it lived as a 4th tab inside the
personal **Settings** page — a mismatch, since Settings is self-service while Access Management administers
*other* users. This change (a) promotes admin features into a dedicated **Admin** area in the sidebar, and
(b) adds the first **user-management** capability: an admin can **edit another user's email** and
**block/unblock their login**. A blocked user is force-logged-out everywhere and cannot log back in.

**Outcome:** an admin-only **Admin** sidebar section with two routes — **`/admin/users`** (roster: edit
email, block/unblock, group assignment, per-user overrides) and **`/admin/access`** (Groups + Features).
Access Management leaves Settings; Settings becomes purely personal.

## Locked decisions (user-confirmed)
1. **Placement** — dedicated admin-only **Admin** sidebar group; `/admin/users` + `/admin/access` real
   routes; Access Management moves out of Settings.
2. **Block policy** — admins are **not blockable** (409, "remove admin access first"). Self-block
   impossible; admin lockout unreachable.
3. **Scope (for now)** — edit email + block/unblock only. No user create/delete. Group assignment &
   overrides already exist and simply relocate to the Users page.
4. **Enforcement** — checked at login (reject) AND on live sessions by deleting them at block time (reuse
   `SessionRepository.DeleteByUserID`). Zero per-request cost.

## Phases (all independently committable, as RBAC was)

### Phase 1 — Backend
- **1a.** Migration `07_add_user_blocked` (sqlite `INTEGER`/postgres `BOOLEAN`, up/down; down uses
  `ALTER TABLE users DROP COLUMN blocked`).
- **1b.** `model.User.Blocked` (mandatory — `SELECT *`); narrow `UserRepository.UpdateEmail` /
  `SetBlocked` (sqlite + postgres), leaving the full-row `Update` untouched; regen mocks.
- **1c.** Login enforcement: `CreateSession` rejects `u.Blocked` with 403 *after* the password check and
  *before* the 2FA branch; `blocked` added to the `logins_total` `result` label.
- **1d.** Admin API in authnz (`internal/authnz/handler/admin_user.go` + `RegisterAdminRoutes`, wired on
  the existing `RequireAdmin` subrouter in `server.go`): `PATCH /admin/users/{id}` (email; 409 on UNIQUE),
  `POST /admin/users/{id}/block` (409 if target is admin, via the wired `AccessResolver`; then
  `SetBlocked`+`DeleteByUserID`), `POST /admin/users/{id}/unblock`. 3 OTel counters.
- **1e.** Expose `blocked` on the roster: `UserAccessRow.Blocked` + `users.blocked` in `ListUsersWithGroup`
  (sqlite + postgres).
- **1f.** `transfer.go`: `blocked` in `userRow` + users SELECT/INSERT (both directions), via `boolInt`/
  `boolForDst`.

### Phase 2 — Frontend restructure
- Sidebar **Admin** group (admin-only) with Users + Access links (`app-sidebar.svelte`).
- `/admin/+layout.ts` guard (redirect non-admins); `/admin/access/+page.svelte` (Groups + Features tabs).
- Remove Access Management from `settings/+page.svelte`.

### Phase 3 — Users management page
- `/admin/users/+page.svelte` (roster + group Select + Overrides + a `⋯` menu with Edit email and
  Block/Unblock; Blocked badge; block hidden for admin rows). `web/src/lib/admin-api.ts` (authnz admin
  calls); `blocked` added to the `UserAccess` interface; `apiPatch` helper added to `api.ts`. Retire the
  old `AccessManagement.svelte` + `UsersTab.svelte` (GroupsTab/FeaturesTab kept for `/admin/access`).

### Phase 4 — Verification & docs
- `go build/vet/test`, `mockery`, `make gen-swagger`, `npm run check`/`build`.
- Live headless-browser walkthrough (SQLite) + a Postgres pass (migration, block flow, `db:transfer`).
- Docs: ADR-008, this plan + progress tracker, `docs/metrics.md`.

## Key files
Backend: `db/migrations/{sqlite3,postgres}/07_add_user_blocked.*`, `internal/authnz/model/user.go`,
`internal/authnz/repository/{interface.go, sqlite/user_repository.go, postgres/user_repository.go}`,
`internal/authnz/handler/{session.go, admin_user.go, metrics.go, handler.go}`,
`internal/rbac/model/model.go`, `internal/rbac/repository/{sqlite,postgres}/user_group_repository.go`,
`cmd/all-in-one/server/server.go`, `cmd/all-in-one/db/transfer.go`.
Frontend: `web/src/components/app-sidebar.svelte`,
`web/src/routes/admin/{+layout.ts, users/+page.svelte, access/+page.svelte}`,
`web/src/routes/settings/+page.svelte`, `web/src/lib/{api.ts, admin-api.ts, rbac-api.ts}`.

## Gotchas
- `model.User.Blocked` mandatory or `SELECT *` breaks app-wide.
- Blocked check must sit before the 2FA branch in `CreateSession`.
- Email 409 (UNIQUE) mapped driver-agnostically, not surfaced as 500.
- transfer.go + mockery + swagger regen must not be skipped.
- `/admin` guard is client-side (ssr=false); backend `RequireAdmin` is the real enforcement.
