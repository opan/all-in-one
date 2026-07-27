# Home Dashboard — Progress Tracker

## Goal
Replace the placeholder "Welcome to SvelteKit" home page (`web/src/routes/+page.svelte`)
with a **personalized launcher dashboard**:

1. Greeting header (`Welcome back, {name}`).
2. Stat tiles — per-feature counts (listing topics, chat conversations + pending
   invites, shortener links), only for features the user can access.
3. App launcher cards — quick links to the apps the user has access to.
4. Admin quick links — only for admins (Users, Access, Shortener, Rate Limits).

Logged-out visitors are redirected to `/login`.

## Decisions
- **Scope:** Launcher + stats (user picked this over launcher-only).
- **Logged-out:** redirect to `/login` (home is an authenticated dashboard).
- **New backend endpoint:** `GET /api/v1/dashboard/summary`.
  - Authenticated but **not** feature-gated → registered on `selfRoutes`
    (same subrouter as `/users/me`), so a user with only some features still
    gets a 200 with just their accessible sections.
  - Server-side feature gating via `rbac` Resolver.EffectiveFeatures: only
    sections the user can access are included (each section is a pointer,
    `omitempty`).
  - Counts are **best-effort**: a per-app count error is logged and the count
    falls back to 0 rather than failing the whole response.
  - OTel: auto-instrumented by `otelmux` (all `/api/v1/*` paths). No manual span,
    matching every other handler in the codebase.
  - Rate limiting: none (selfRoutes carries no rlMw, same as `/users/me`), so no
    ratelimit Registry entry needed.
  - Metrics: no new metrics (no `metrics.md` exists; endpoint gets standard
    request logging + tracing).
- **New app package:** `internal/dashboard` — a pure aggregator over the
  existing listing/chat/shortener storages + rbac Resolver (constructed in
  `server.go` from already-built services; does not open its own DB handle).

## Counts (reuse existing repo methods — no new repo/interface/mock changes)
- Listing:   `TopicRepo().GetAll(ctx, userID)` → `len(topics)`
- Chat:      `SessionRepo().GetAllByUserID(ctx, userID)` → conversations;
             `InviteRepo().GetPendingByInviteeID(ctx, userID)` → pending invites
- Shortener: `ShortLinkRepo().ListByOwner(ctx, userID, 1, 1)` → total (uint32)

## Response shape
```json
{
  "success": true,
  "data": {
    "listing":   { "topics": 5 },
    "chat":      { "conversations": 3, "pending_invites": 1 },
    "shortener": { "links": 12 }
  }
}
```
Sections absent when the user lacks the feature.

## Tasks
- [x] `internal/dashboard/model/summary.go` — Summary + section structs
- [x] `internal/dashboard/handler/handler.go` — Summary HTTP handler + routes (+ handler_test.go)
- [x] `internal/dashboard/service/service.go` — aggregator wiring (+ service_test.go)
- [x] Wire into `cmd/all-in-one/server/server.go` (selfRoutes)
- [x] `web/src/lib/dashboard-api.ts` — types + `getDashboardSummary()`
- [x] `web/src/routes/+page.ts` — load: fetch summary (apiLoad auto-redirects on 401)
- [x] `web/src/routes/+page.svelte` — greeting + tiles + cards + admin links
- [x] Verify: `go build ./...` ok, `go test ./internal/dashboard/...` ok, `npm run check` 0 errors, `npm run build` ok
- [x] ADR: `docs/adr/DASHBOARD_ADR.md`

## Notes
- Tests use interface-embedding fakes (mockery not on PATH here); no generated mocks needed.
- `swag` not on PATH — handler carries Swagger annotations; run `make gen-swagger` where swag is
  installed to refresh `docs/` and surface the endpoint in Swagger UI.
- Not yet done: real browser E2E (the `frontend-browser-testing` skill) against a running server —
  optional follow-up to click through the live page.

## Status: DONE
