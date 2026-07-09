# Admin Content-Management Pages — Progress Tracker

**Status:** Pilot (Shortener) — Phases 1-5 complete, Phase 6 (final live verification) remaining.
**Last updated:** 2026-07-09
**Design rationale:** [docs/adr/ACCESS_MANAGEMENT_ADR.md](../docs/adr/ACCESS_MANAGEMENT_ADR.md) — ADR-009
**Plan file (session-local, does not travel across machines):** `.claude/plans/while-we-are-including-purrfect-wadler.md`

## What this feature is
Admin oversight/moderation over each app's actual **content** (short links, listing topics/items, chat
sessions/messages) — not access (RBAC, ADR-001–007) or identity (user admin, ADR-008), but the records each
app produces. Every app's repository was built assuming a single owning user, so each app needs new
owner-agnostic methods before an admin page can exist for it. Shortener is the pilot; listing and chat follow
later using the same recipe (separate future work, not part of this tracker).

### Locked decisions (do NOT re-litigate — see ADR-009 for rationale)
1. Scope: **view + moderate** (list all, activate/deactivate, delete) — no create/edit on a user's behalf.
2. **One admin page per app** (`/admin/<app>`), not a single tabbed content page.
3. **Pilot one app (Shortener) before replicating** to listing/chat.
4. Gating: **`is_admin` only**, reusing the existing `RequireAdmin` subrouter — no new feature-registry entry.

## The recipe (repeat per app)
1. Add owner-agnostic sibling repo methods (list-all, delete, moderate) on both SQLite and Postgres.
2. New `admin.go` handler file with `RegisterAdminRoutes`, mounted with one line on the existing
   `adminRoutes` subrouter in `server.go` (no per-endpoint admin re-check needed).
3. `/admin/<app>` page mirroring the app's existing user-facing table/dialog patterns, plus an owner column.
4. Sidebar entry under `adminItems` in `app-sidebar.svelte`.

## Phase checklist

- [x] **Phase 0 — Branch setup.** Working directly on `feat/rbac` (already has the `/admin/*` scaffolding
  from ADR-004/006/008); user will merge to `main` once this lands.
- [x] **Phase 1 — Repository layer.** `internal/shortener/model/shortlink.go` (`ShortLinkWithOwner`),
  `internal/shortener/repository/interfaces.go` + `sqlite`/`postgres` impls (`ListAll`, `DeleteByCode`,
  `SetActiveByCode`). Commit `fe44a68`.
- [x] **Phase 2 — Handler, service, server wiring.** `internal/shortener/handler/admin.go` (new),
  `metrics.go` (2 new counters), `service.go` (`RegisterAdminRoutes` passthrough), one line in `server.go`.
  Commit `542c9c9` — bundled with a fix regenerating the stale `mock_ShortLinkRepository.go` (Phase 1 added
  interface methods without updating the mock, which broke CI's "Run Go tests" step on `fe44a68`; fixed by
  reinstalling the exact pinned `mockery v2.53.5` and regenerating only that mock).
- [x] **Phase 3 — Backend tests.** Repo tests for `ListAll` (owner + owner-less mix, pagination, empty),
  `DeleteByCode`, `SetActiveByCode` against in-memory SQLite (schema extended with a minimal `users` table for
  the owner-username join); handler tests for the 3 admin endpoints (200/400/404). Commit `b1b9019`.
- [x] **Phase 4 — Frontend.** `web/src/lib/shortener-admin-api.ts` (new), `web/src/routes/admin/shortener/+page.svelte`
  (new — mirrors `/shortener`'s Switch-toggle + AlertDialog-delete pattern, adds an Owner column), sidebar
  entry. Commit `259eff6`. `npm run check` (0 errors) and `npm run build` both verified clean.
- [x] **Phase 5 — Docs.** `make gen-swagger` (3 `/admin/shortener/*` endpoints confirmed in
  `docs/swagger.json`/`.yaml`/`docs.go`, clean additive diff); `docs/metrics.md` (2 new counters,
  `action` label values, cardinality ~56 → ~59); ADR-009 added to `docs/adr/ACCESS_MANAGEMENT_ADR.md`; this
  tracker.
- [ ] **Phase 6 — Final verification.** Not yet started. Needs: scratch SQLite DB + `db:seed` + curl
  walkthrough as `admin` (list-all across seeded users, deactivate confirms `/r/{code}` 404s, delete confirms
  404 after); confirm non-admin gets redirected/403; headless-browser (Playwright) walkthrough of the real
  SPA; optional Postgres parity pass (rootless throwaway cluster, matching the RBAC/user-admin phases' bar).

## Environment notes (this sandbox specifically, not project-wide)
- No Go toolchain preinstalled — installed manually mid-session (`/usr/local/go/bin`).
- No C compiler (`gcc`/`build-essential`) and no passwordless sudo — `go-sqlite3` requires cgo, so `go build
  ./...`/`go test ./...` cannot fully run here. Verified affected packages compile via `gofmt`/`go vet` where
  possible instead, and relied on GitHub Actions CI (which has real cgo) to confirm each phase — every phase's
  commit was pushed and its CI run checked before proceeding to the next phase.
- Node/npm not on PATH by default but available via `nvm` (`~/.nvm`) — Phase 4 was fully verified locally
  (`npm run check`, `npm run build`) once `nvm use` + `npm install` were run.
- `mockery` and `swag` CLIs were not preinstalled; installed via `go install` (mockery pinned to the repo's
  existing `v2.53.5` to avoid unrelated version-bump diffs in unrelated mock files).

## Resume instructions
- Open this file — only Phase 6 is unchecked. Read that section and the ADR-009 Decision section for scope
  reminders.
- If resuming on another machine/session: this file + `docs/adr/ACCESS_MANAGEMENT_ADR.md` are git-tracked and
  travel with the branch; the plan file under `.claude/plans/` is machine-local and does not.
- All phase commits are on `feat/rbac` (`fe44a68`, `542c9c9`, `b1b9019`, `259eff6`, and this Phase 5 commit) —
  each was pushed and its CI run confirmed green before starting the next phase.
