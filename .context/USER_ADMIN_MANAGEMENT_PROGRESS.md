# User & Admin Management — Progress Tracker

**Status:** ✅ Complete. Dedicated admin-only **Admin** sidebar area shipped (`/admin/users` +
`/admin/access`), Access Management moved out of Settings, and user management (edit email, block/unblock
login) is implemented, enforced, and verified end-to-end on both SQLite and Postgres.
**Last updated:** 2026-07-08
**Plan:** [USER_ADMIN_MANAGEMENT_IMPLEMENTATION_PLAN.md](USER_ADMIN_MANAGEMENT_IMPLEMENTATION_PLAN.md)
**Design rationale:** [docs/adr/ACCESS_MANAGEMENT_ADR.md](../docs/adr/ACCESS_MANAGEMENT_ADR.md) — ADR-008

## What this feature is
Promotes admin-only capabilities into a dedicated **Admin** sidebar section and adds basic user
administration. Admins can edit another user's email and block/unblock login; blocking force-logs-the-user
out and rejects further logins. Access Management (Groups/Features) moved from Settings into `/admin/access`.

### Locked decisions (do NOT re-litigate — see ADR-008)
1. Dedicated **Admin** sidebar area; `/admin/users` + `/admin/access`; Access Management out of Settings.
2. Admins are **not blockable** (409). Self-block impossible; admin lockout unreachable.
3. Scope: edit email + block/unblock only (no create/delete). Group assignment & overrides relocated.
4. Block enforced at login + by deleting sessions on block (`SessionRepository.DeleteByUserID`).

## Phase checklist
- [x] **Phase 1 — Backend.**
  - Migration `07_add_user_blocked` (sqlite `INTEGER`/postgres `BOOLEAN`, up/down). Applies cleanly on both
    backends (verified live).
  - `model.User.Blocked`; narrow `UserRepository.UpdateEmail`/`SetBlocked` on sqlite + postgres (full-row
    `Update` untouched — all its callers load-then-update); mocks regenerated.
  - Login enforcement in `CreateSession`: 403 "account is blocked" after password, before the 2FA branch;
    `blocked` added to the `logins_total` `result` label (reused counter, no new metric).
  - Admin API in **authnz** (`admin_user.go` + `RegisterAdminRoutes`, mounted on the existing `RequireAdmin`
    subrouter): `PATCH /admin/users/{id}` (email; **409** on UNIQUE via driver-agnostic match),
    `POST .../block` (**409** if target is admin, via the already-wired `AccessResolver`; then `SetBlocked` +
    `DeleteByUserID`), `POST .../unblock`. OTel: `aio_admin_user_email_updated_total`,
    `aio_admin_user_blocked_total{result}`, `aio_admin_user_unblocked_total`.
  - Roster read model: `UserAccessRow.Blocked` + `users.blocked` in `ListUsersWithGroup` (sqlite + postgres)
    — keeps the Users page to one list call.
  - `transfer.go`: `blocked` in `userRow` + users SELECT/INSERT (both directions); round-trip covered by
    `transfer_test.go` (blocked user survives).
  - Tests: `admin_user_test.go` (email 200/400/404/409, block success + admin-409 + 404, unblock) — all
    green, incl. `go test -race`. Full suite + `go vet` clean.
- [x] **Phase 2 — Frontend restructure.**
  - `app-sidebar.svelte`: admin-only **Admin** group (Users + Access), gated on `$auth.is_admin`.
  - `admin/+layout.ts` guard (redirects non-admins to `/`); `admin/access/+page.svelte` (Groups + Features
    tabs, keeping the `active`-prop refetch fix for bits-ui Tabs).
  - Access Management removed from `settings/+page.svelte` (Settings is personal-only again).
- [x] **Phase 3 — Users management page.**
  - `admin/users/+page.svelte`: roster `DataTable` (username + Admin/Blocked badges), group Select,
    Overrides dialog, and a `⋯` DropdownMenu with **Edit email** (Dialog) and **Block/Unblock** (AlertDialog
    confirm on block). Block hidden for admin rows.
  - `web/src/lib/admin-api.ts` (`updateUserEmail`/`blockUser`/`unblockUser`); `apiPatch` added to `api.ts`;
    `blocked` added to `UserAccess`. Old `AccessManagement.svelte` + `UsersTab.svelte` retired
    (GroupsTab/FeaturesTab kept).
  - `npm run check` (0 errors) + `npm run build` clean.
- [x] **Phase 4 — Verification & docs.**
  - **Live headless-browser walkthrough (SQLite): 25/25 checks** — Admin section visibility (admin) vs
    absence + redirect (non-admin); `/admin/users` renders roster + badges + gated `⋯` menu; `/admin/access`
    Groups/Features; Settings has no Access Management; PATCH email 200 + roster reflects it; block-admin 409;
    block demo 200 → demo login 403 "account is blocked" → unblock 200 → demo login 201.
  - **Postgres pass** (rootless `initdb`/`pg_ctl`): migration `07` applies (`blocked boolean NOT NULL DEFAULT
    false`); full block/email curl flow behaves identically to SQLite; `db:transfer --direction sqlite-to-pg`
    round-trips a blocked user correctly.
  - `make gen-swagger` (3 `/admin/users*` endpoints present); `docs/metrics.md` updated (3 counters +
    `result="blocked"`, cardinality ~56); ADR-008 + this tracker written.
  - Not yet committed to git.

## Gotchas captured
- `model.User.Blocked` mandatory or `SELECT *` breaks app-wide (same lesson as `GroupID` in RBAC).
- Blocked check sits before the 2FA branch in `CreateSession` (else a blocked 2FA user gets a challenge).
- Email UNIQUE violation mapped to 409 driver-agnostically (string match on the sqlite/pq error) rather than
  surfacing a raw 500.
- `db:transfer` + `mockery` + `make gen-swagger` must be re-run on any schema/interface/endpoint change.
- Test-harness note: a Playwright `page.evaluate(fetch)` must run from a page already navigated to the app
  origin — an `about:blank` context can't fetch the backend (cost one red run before the fix).

## Resume instructions
Feature is complete and verified; only `git` commit + review remain (nothing committed yet). For a new
session, read ADR-008 + this tracker; the machine-local `~/.claude/plans/` copy does not travel.
