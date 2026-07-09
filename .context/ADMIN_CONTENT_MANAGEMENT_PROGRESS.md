# Admin Content-Management Pages — Progress Tracker

**Status:** ✅ Feature complete. Pilot (Shortener) — all 7 phases done and verified (unit tests + a live
curl walkthrough + a live headless-browser walkthrough). Nothing outstanding; listing/chat replication is
separate future work, not part of this tracker.
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
- [x] **Phase 6 — Final verification.**
  - Once a C compiler was installed (this sandbox had none initially — see Environment notes), `go build
    ./...`, `go vet`, and `go test ./internal/shortener/...` all passed for real (previously only verified via
    `gofmt`/CI due to the missing cgo toolchain locally).
  - **Live curl walkthrough** against a scratch SQLite DB (`ALLINONE_STORAGE_SQLITE_DB_PATH`, never
    `all-in-one.db`) with the real seeded users: created short links as `user` and `demo`, confirmed
    `GET /api/v1/admin/shortener/links` as `admin` lists both across owners with correct `owner_username` (the
    `LEFT JOIN` to `users` works); confirmed `user` (non-admin) gets `403 forbidden`; deactivated `demo`'s link
    as `admin` and confirmed `GET /r/{code}` correctly 404s despite `admin` not owning it; deleted `user`'s
    link as `admin` and confirmed it's gone from the list and a second delete 404s.
  - **Live headless-browser walkthrough** (Playwright + Chromium — neither was preinstalled; Chromium
    (~300MB) and its system libs (`playwright install-deps`, 71 packages) were installed with the user's
    consent/sudo mid-phase). **13/13 checks passed**: sidebar shows the new Shortener admin item for `admin`
    and correctly hides it for `user`; `/admin/shortener` renders both seeded links with owner column
    populated; deactivating via the UI Switch dims the row; the delete `AlertDialog` names the specific short
    code and removing it updates the table live; non-admin visiting `/admin/shortener` is redirected away and
    a direct `fetch` of the admin API returns 403; zero unexpected 4xx/5xx from the admin page. Screenshots
    visually confirmed (not just DOM assertions) — table renders correctly, dialog renders correctly.
  - Postgres parity pass: **skipped** — not requested for this pilot; the SQLite path (this project's
    default/primary backend) is fully verified. Revisit if Postgres parity becomes a requirement before
    merging, following the same rootless-throwaway-cluster recipe used in the RBAC/user-admin phases.
  - Scratch server, DB, cookies, and screenshots all cleaned up after verification — nothing left running.

## Environment notes (this sandbox specifically, not project-wide)
- No Go toolchain preinstalled — installed manually mid-session (`/usr/local/go/bin`).
- No C compiler (`gcc`/`build-essential`) and no passwordless sudo initially — `go-sqlite3` requires cgo, so
  `go build ./...`/`go test ./...` couldn't fully run here for Phases 1-5. Verified affected packages via
  `gofmt`/`go vet` where possible instead, and relied on GitHub Actions CI (real cgo) to confirm each phase —
  every phase's commit was pushed and its CI run checked before proceeding. The user installed
  `build-essential` before Phase 6 (which needs to actually *run* the server, not just build it — CI can't
  substitute for that), after which `go build`/`go vet`/`go test` all passed locally too.
- Node/npm not on PATH by default but available via `nvm` (`~/.nvm`) — Phase 4 was fully verified locally
  (`npm run check`, `npm run build`) once `nvm use` + `npm install` were run.
- `mockery` and `swag` CLIs were not preinstalled; installed via `go install` (mockery pinned to the repo's
  existing `v2.53.5` to avoid unrelated version-bump diffs in unrelated mock files).
- Playwright's Chromium browser and its system libs (`playwright install-deps`, 71 packages — GTK/ATK/font
  libs for headless Chrome) were not preinstalled; both required the user's consent/sudo and were installed
  during Phase 6. Follow `.claude/skills/frontend-browser-testing/SKILL.md` verbatim for the scratch-server
  recipe — it already documents the known-working selectors and bits-ui gotchas.

## Resume instructions
- All 7 phases are done and independently verified (unit tests, CI, a live curl walkthrough, and a live
  headless-browser walkthrough). Nothing on this tracker is outstanding for the Shortener pilot.
- If resuming on another machine/session: this file + `docs/adr/ACCESS_MANAGEMENT_ADR.md` are git-tracked and
  travel with the branch; the plan file under `.claude/plans/` is machine-local and does not.
- All phase commits are on `feat/rbac` (`fe44a68`, `542c9c9`, `b1b9019`, `259eff6`, `8ff43f0`, and this Phase 6
  commit) — each was pushed and its CI run confirmed green before starting the next phase.
- Next step per ADR-009: replicate the same recipe to **listing** (needs a new global list-all-topics repo
  method) and **chat** (needs global lists + a message-delete method that doesn't exist yet) — separate future
  work, not part of this tracker. Otherwise, only normal commit/PR/review remains before this ships (the user
  plans to merge `feat/rbac` into `main` once done).
