# ADR: Home Dashboard (launcher + stats)

This document records design decisions made when replacing the placeholder home page
(`web/src/routes/+page.svelte`, previously the default "Welcome to SvelteKit" text) with a
personalized **launcher dashboard**: a greeting, per-feature stat tiles, app launcher cards, and
admin quick links. Add a new entry here for any future change that touches the home dashboard.

Implementation status is tracked in `.context/DASHBOARD_PROGRESS.md`. This document records the
*decisions*.

---

## ADR-001: A dedicated `internal/dashboard` app that only aggregates existing storages

### Status
Accepted

### Context
The dashboard needs per-user counts drawn from three existing apps (listing, chat, shortener). Those
counts don't belong to any single app, and none of the apps should depend on the others.

### Decision
Add a new app package `internal/dashboard` (`model` + `handler` + `service`) exposing a single
endpoint. Its `service` is a **pure aggregator**: it receives the already-constructed
`listing`/`chat`/`shortener` repository `Storage` interfaces plus the RBAC `Resolver` from
`server.go` and opens **no DB handle of its own**. Counts reuse existing repository methods — no new
repository interface methods, SQL, or mocks:

- Listing:   `TopicRepo().GetAll(ctx, userID)` → `len`
- Chat:      `SessionRepo().GetAllByUserID` → conversations; `InviteRepo().GetPendingByInviteeID` → pending invites
- Shortener: `ShortLinkRepo().ListByOwner(ctx, userID, 1, 1)` → returned total

### Rationale
- A cross-cutting read view is exactly what an aggregator package is for; putting it inside one app
  (e.g. authnz) would leak unrelated dependencies into that app.
- Reusing existing read methods keeps the change additive and avoids touching each app's
  repository/interface/mocks/both DB backends.
- Dependencies point one way (dashboard → apps); no app imports dashboard, so there's no cycle.

### Alternatives Considered
- **New aggregate SQL per app / a `CountByOwner` method:** cleaner counts, but multiplies the surface
  (interface + sqlite + postgres + mocks × 3 apps) for a first-pass dashboard. Deferred; can replace
  the `len()`/`ListByOwner(1,1)` approach later behind the same handler contract if counts get hot.
- **Frontend fans out to each app's list endpoint:** more round-trips, and it would re-derive feature
  gating in the client; a single server-gated endpoint is simpler and authoritative.

---

## ADR-002: `GET /api/v1/dashboard/summary` is authenticated but not feature-gated

### Status
Accepted

### Context
`server.go` splits authenticated routes into per-feature subrouters gated by `RequireFeature`. A user
may hold any subset of features. A summary gated on a single feature would 403 users who lack it, and
there is no natural single feature for a cross-app view.

### Decision
Register the endpoint on the **`selfRoutes`** subrouter (JWT auth, no feature gate) — the same
subrouter as `/users/me`. Feature gating happens **inside** the aggregator: it calls
`Resolver.EffectiveFeatures` once and includes a section only for features the user can access
(admins see all). Each section is a pointer with `omitempty`, so the JSON simply omits sections the
user can't access.

### Rationale
- Mirrors `/users/me`, which is likewise authenticated-but-not-feature-gated self-service data.
- One resolver call is the same RBAC source the sidebar and middleware use, so the dashboard, the
  sidebar, and the backend gate all agree on what a user can access.
- Section presence becomes the single source of truth the frontend keys off (see ADR-004).

### Consequences
- No rate-limit target is added: `selfRoutes` carries no limiter middleware (same as `/users/me`), so
  the `validateRateLimitBindings` boot check is unaffected.
- OTel tracing is automatic: `otelmux` instruments every `/api/v1/*` route, matching every other
  handler (no handler manually starts spans). No new metrics were added; no `metrics.md` exists.

---

## ADR-003: Per-app counts are best-effort

### Status
Accepted

### Decision
Feature access is resolved once up front and a resolver error fails the request (500). But each
individual count is best-effort: a per-app count error is logged and its value left at 0; the section
is still returned. One app's DB hiccup degrades a single tile rather than blanking the whole
dashboard.

### Rationale
The dashboard is a glanceable overview, not a transactional read — partial data is strictly better
than an error page. The failure is logged for observability.

---

## ADR-004: Frontend renders from section presence; logged-out users redirect to /login

### Status
Accepted

### Decision
- `web/src/lib/dashboard-api.ts` types the payload; `web/src/routes/+page.ts` loads it via `apiLoad`,
  which redirects to `/login` on an unrecoverable 401. The home route is therefore an authenticated
  dashboard, not a public landing page.
- `+page.svelte` derives **both** the stat tiles and the launcher cards from *which sections the
  summary contains* — it does not re-derive feature access from a separate list. The greeting name and
  the admin-only quick links come from the shared `auth` store (`$auth.name`, `$auth.is_admin`).

### Rationale
- Reusing `apiLoad`'s existing 401→refresh→redirect flow means the auth gate needs no new code.
- Keying UI off section presence keeps the client in lock-step with the server-side RBAC gate
  (ADR-002): a feature the backend omits can never render a tile or card for it.
- A non-auth error (e.g. 500) yields an empty summary so the page still renders instead of erroring
  the route.

### Consequences
- Swagger annotations were added to the handler, but `swag`/`mockery` are not on PATH in this
  environment, so `make gen-swagger` (regenerating `docs/`) must be run wherever those tools are
  installed to surface the endpoint in the Swagger UI.
