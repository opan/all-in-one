# ADR: Access Management (RBAC)

This document records design decisions made when adding the admin-only **Access Management** feature —
authorization control over which app-features (listing, chat, shortener, …) each user can access. Add a
new entry here for any future feature or fix that touches authorization.

Implementation status is tracked separately in `.context/RBAC_PROGRESS.md`; this document records the
*decisions*, not build progress.

---

## ADR-001: One group per user, not many-to-many

### Status
Accepted

### Context
Users need to be grouped into permission presets (e.g. `admin`, `regular-user`, a custom `listing-group`).
The two natural designs are a single `users.group_id` FK, or a `user_groups` join table supporting
multiple groups per user with union-of-grants semantics.

### Decision
Each user belongs to exactly one group via a nullable `users.group_id` FK. `NULL` resolves to the
built-in `regular-user` group at read time (no backfill required).

### Rationale
- Matches the mental model requested: users are "assigned to a group," not a set of groups.
- Collapses permission resolution to a single group lookup instead of a union across N groups, which
  keeps both the resolver and the management UI simpler.
- A single FK is trivial to reason about for the admin doing the assigning ("which group is this user
  in?" always has one answer).

### Alternatives Considered
| Option | Rejected because |
|---|---|
| `user_groups` many-to-many join table | Union-of-grants across multiple groups is harder to reason about ("why can this user access X?" requires checking every group they're in) and the UI has to present multi-select instead of a single dropdown. Not needed by the stated requirements. |

### Consequences
- If multi-group membership is needed later, it's an additive migration (new join table) plus a resolver
  change from "the group" to "union of groups" — the precedence rule (ADR-002) is unaffected since
  overrides still sit above any group-derived grant.

### Key files
- `db/migrations/{sqlite3,postgres}/06_add_rbac_tables.up.sql` — `users.group_id` column
- `internal/rbac/service/resolver.go` — NULL → `regular-user` resolution

---

## ADR-002: Precedence — admin bypass > user override > group grant > default-deny

### Status
Accepted

### Context
Access to a feature can be influenced by three things: whether the user is an admin, whether the user
has a personal override for that feature, and what their group grants. These can conflict (e.g. the
user's group grants `chat`, but an admin has explicitly revoked `chat` for that one user) and the
precedence must be unambiguous.

### Decision
Resolution order, highest precedence first:
1. **Admin bypass** — membership in the `admin` group grants every feature, unconditionally.
2. **Per-user override** — a row in `user_feature_overrides` for `(user, feature)` is tri-state: its
   presence means `allow` decides the outcome regardless of group; its absence means "inherit."
3. **Group grant** — a row in `group_features` for `(group, feature)` grants access.
4. **Default** — deny.

### Rationale
- Tri-state overrides (not grant-only) are required to satisfy "access set to specific users should take
  higher priority than group level" in both directions — an admin must be able to both grant a user
  something their group doesn't have, and revoke something their group does.
- Putting admin bypass *above* overrides (rather than modeling admin as "a group with an override on
  every feature") makes the admin-lockout guard (ADR-004) trivially correct: an admin can never be
  denied by an override, by construction, so the lockout guard only needs to reason about group
  membership, not override state.
- "Allow all by default" is deliberately *not* a branch in this precedence chain — it is realized by
  seeding the `regular-user` group with a grant for every non-admin-only feature (ADR-005/ADR-007). This
  keeps the resolver's logic uniform (a grant is a grant, regardless of why it exists) instead of adding a
  special "default allow unless admin-only" branch that every future reader would have to reconcile
  against the explicit grant table.

### Alternatives Considered
| Option | Rejected because |
|---|---|
| Grant-only overrides (no revoke) | Cannot satisfy the requirement that per-user access can restrict below the group's grant — e.g. temporarily pulling one user out of an otherwise-permitted group feature without moving them to a whole new group. |
| Model admin as "grants implied for every feature via group_features rows" | Requires inserting a `group_features` row per feature per app for the admin group, and re-inserting on every new feature — brittle, and re-introduces a lockout risk if an override or a missing row could deny an admin. |

### Consequences
- Custom restrictive groups (e.g. `listing-group`) are safe by default: a feature not explicitly granted
  to a group is denied, with no separate "deny list" needed at the group level.
- A group can be deleted or a user's override cleared without ever being able to elevate a non-admin to
  admin-equivalent access — admin status is solely a function of group membership.

### Key files
- `internal/rbac/service/resolver.go` — `CanAccess`, `IsAdmin`, `EffectiveFeatures`
- `internal/rbac/service/bootstrap.go` — seeds the "allow all by default" grants

---

## ADR-003: JWT stays role-free; authorization resolved from DB per request

### Status
Accepted

### Context
Once a user's group/role is known, it could be embedded in the JWT (stateless checks, no extra DB
lookups) or resolved from the database on every request.

### Decision
Do not add role, group, or permission claims to the access token. `createAccessToken` continues to embed
only `sub`, `user_id`, `username`, `email`. Authorization is resolved from the database on every gated
request via `internal/rbac/service.Resolver`.

### Rationale
- The JWT middleware already performs a mandatory DB round-trip per authenticated request
  (`sessionRepo.Get` in `internal/authnz/middleware/jwt.go`, added for session-invalidation-on-password-
  reset — see `USER_AUTHENTICATION_ADR.md` ADR-002). Resolving authorization from the DB adds only a
  couple of additional indexed lookups on a request that is already DB-bound; it is not a new class of
  cost.
- The alternative (JWT-embedded role) reintroduces exactly the staleness problem ADR-002 in
  `USER_AUTHENTICATION_ADR.md` solved for session invalidation: a user demoted from admin, moved to a
  more restrictive group, or given a revoking override would keep their old privileges until the access
  token expires (up to 30 minutes) or is refreshed. Zero-staleness authorization is a stronger security
  property than saving a few indexed reads.

### Alternatives Considered
| Option | Rejected because |
|---|---|
| Embed role/group in JWT claims | Reintroduces a stale-privilege window on every access/group/override change; requires forcing re-login or token refresh to take effect, which is easy to forget when writing the admin UI. |
| Short-TTL in-process permission cache keyed by user ID | Reasonable future optimization if per-request lookups become a bottleneck, but adds invalidation complexity (must bust on every group/override/membership write) not justified at current scale. Left as a noted future extension, not built now. |

### Consequences
- Every gated request costs ~2–3 additional indexed lookups (group ID, override-by-key, grant-by-key).
  Consistent with the cost profile already accepted for session validation.
- No cache invalidation logic is needed anywhere in the management API — every write is immediately
  authoritative.

### Key files
- `internal/authnz/handler/session.go` (`createAccessToken`) — unchanged, no role claim added
- `internal/authnz/middleware/jwt.go` — existing per-request session lookup this decision piggybacks on
- `internal/rbac/service/resolver.go` — per-request resolution

---

## ADR-004: Admin is a full superuser; system guarantees at least one admin

### Status
Accepted

### Context
The `admin` group needs a well-defined scope (does membership unlock only the Access Management UI, or
everything?), and admin-group management (reassigning/removing the last admin) creates an obvious
self-lockout hazard.

### Decision
Membership in the `admin` group bypasses every feature gate — every app, current and future, plus the
Access Management UI itself (ADR-002). The system enforces a standing invariant: at least one user must
belong to the `admin` group at all times. This is enforced two ways:
1. **Bootstrap** (`service/bootstrap.go`) assigns `cfg.RBAC.AdminUsername` (default `"admin"`) to the
   admin group whenever the admin group is empty — covering both first-run initialization and recovery
   from a hypothetical lockout.
2. **Management API guard** — reassigning a user's group away from `admin` is rejected with `409` when
   they are the last remaining admin (`CountByGroup(admin) > 1` required to proceed). The built-in
   `admin`/`regular-user` groups themselves cannot be deleted or renamed.

### Rationale
- A partial-admin scope (e.g. "admin only unlocks Access Management, but admins still need explicit
  feature grants like anyone else") was considered and rejected per explicit user preference — the
  simpler mental model of "admin = superuser" avoids admins needing to grant themselves access to apps
  they should obviously be able to reach, and avoids an admin locking themselves out of a feature by
  misconfiguring their own group's grants.
- Because admin bypass sits above per-user overrides (ADR-002), the lockout guard only has to reason
  about group membership — an override can never be the cause of an admin losing access.

### Consequences
- Deleting or emptying the `admin` group is impossible through the management API; the only path to zero
  admins is direct DB manipulation, which bootstrap self-heals on the next server start.
- Admin actions (e.g. narrowing `regular-user`'s grants) cannot accidentally affect admins themselves,
  since admin bypass ignores group grants entirely.

### Key files
- `internal/rbac/service/bootstrap.go` — `EnsureBuiltin`, last-admin backstop
- `internal/rbac/service/service.go` — last-admin-reassignment guard, built-in group protection

---

## ADR-005: Allow-all-by-default realized as seeded data, not resolver logic

### Status
Accepted

### Context
Non-admin users should be able to access every current app-feature by default, except features flagged
admin-only (e.g. Access Management itself). This default needs to hold both at initial bootstrap and
whenever a new app/feature is added later.

### Decision
Every feature carries an `admin_only` boolean. Bootstrap grants the `regular-user` group every feature
where `admin_only = false`, and this sync runs idempotently on every server start — so a newly-registered
feature (see ADR-007) is automatically granted to `regular-user` the first time the updated binary boots,
with no manual migration step.

### Rationale
- Keeping "default allow" as *data* (rows in `group_features`) rather than a resolver special-case
  ("if feature.admin_only == false and user has no group, allow") means the resolver has exactly one
  access rule — "is there a grant" — instead of two rules that must be kept consistent with each other.
  It also means an admin can *narrow* `regular-user`'s default access later by editing the group's grants
  through the same management API used for any other group, with no special-cased "reset to default"
  path needed.
- Admin-only features are simply never inserted into `regular-user`'s grants, so non-admins fall through
  to default-deny (ADR-002) without any dedicated admin-only check outside the ordinary precedence chain.

### Consequences
- A fresh install and an upgraded existing install converge to the same state: bootstrap is safe to run
  unconditionally on every boot (see ADR-007).
- If a future requirement needs group-level *deny* (not just absence-of-grant), `group_features` would
  need an `allow` boolean column mirroring `user_feature_overrides` — noted as a clean, additive future
  extension, not built now since nothing today requires denying a specific feature to a custom group
  (absence of a grant already denies it).

### Key files
- `internal/rbac/service/bootstrap.go`
- `db/migrations/{sqlite3,postgres}/06_add_rbac_tables.up.sql` — `features.admin_only`

---

## ADR-006: Enforcement via per-app gated sibling subrouters, not a path-prefix map

### Status
Accepted

### Context
All four apps (listing, authnz, chat, shortener) currently register their authenticated routes onto one
shared `authenticatedRoutes` gorilla/mux subrouter (`cmd/all-in-one/server/server.go`). Gating by feature
requires inserting an authorization check somewhere in that chain. Routes are not cleanly namespaced per
app: `chat` registers `/users/search` alongside `authnz`'s `/users/me`, `/users/reset_password`, etc., so
a `PathPrefix("/users")`-based feature map would incorrectly gate a chat endpoint as if it were an authnz
endpoint (or vice versa).

### Decision
Split the single `authenticatedRoutes` subrouter into sibling subrouters — one per app, each independently
carrying `jwtMiddleware.JWTAuth` followed by `authz.RequireFeature("<key>")` — plus an ungated sibling for
authnz's own account self-service routes (`/users/me`, `/users/reset_password`, `/users/2fa/*`), and an
admin-only sibling (`RequireAdmin`) for the RBAC management API itself. This mirrors the existing
public-vs-authenticated sibling-subrouter split already in `server.go`, which already demonstrates that
gorilla/mux backtracks correctly across sibling subrouters and applies only the matched sibling's
middleware chain.

### Rationale
- Exact-route matching (via each app's own `RegisterAuthenticatedRoutes` call against its own subrouter)
  sidesteps the `/users/search` vs `/users/*` collision entirely — no route-to-feature string map needs to
  be built or kept in sync as routes change.
- No app's `RegisterAuthenticatedRoutes(router *mux.Router)` signature needs to change; only `server.go`'s
  wiring changes.

### Alternatives Considered
| Option | Rejected because |
|---|---|
| Single middleware on the shared subrouter, mapping matched route template → feature key | Requires building and maintaining a route-template-to-feature lookup table that duplicates information already implicit in which service registered the route; the `/users/search` (chat) vs `/users/me` (authnz) overlap makes template-based matching error-prone. |
| Gate by URL path prefix per app | Only `shortener` has a real path prefix (`/shortener/*`); listing/chat/authnz share top-level route namespaces, so this doesn't generalize. |

### Consequences
- Adding a new gated app means adding one `mkGated("<feature-key>")`-wrapped subrouter line in
  `server.go`, documented as part of the new-app-onboarding steps in `RBAC_IMPLEMENTATION_PLAN.md`.

### Key files
- `cmd/all-in-one/server/server.go` — subrouter construction
- `internal/rbac/middleware/authz.go` — `RequireFeature`, `RequireAdmin`

---

## ADR-007: Code-defined feature registry synced into a DB table

### Status
Accepted

### Context
Gateable features correspond to real, code-defined apps (listing, chat, shortener, access-management),
not admin-authored data. The management UI and the schema's foreign keys (`group_features`,
`user_feature_overrides`) both need a `features` table to reference, but the set of valid features should
not silently drift from what the codebase actually implements.

### Decision
`internal/rbac/features.go` holds a Go-level `Registry` (feature key constants + name + `admin_only`) as
the single source of truth. `service/bootstrap.go` upserts this registry into the `features` table
idempotently on every server start (insert-if-absent by `key`; does not overwrite `admin_only` on rows
that already exist, to avoid silently changing an admin's prior data-level customization — drift between
code and DB is logged instead of auto-corrected).

### Rationale
- New apps register a feature by adding one entry to `Registry`; bootstrap then auto-creates the DB row
  and (per ADR-005) auto-grants it to `regular-user` if non-admin-only — no manual data migration required
  to onboard a new app into the RBAC system.
- Keeping features code-defined (rather than admin-creatable through the UI) reflects that a "feature" is
  a real, enforced gate tied to actual route-wiring (ADR-006) — an admin-created feature with no
  corresponding `RequireFeature(key)` call anywhere would be meaningless (grantable in the UI but
  enforcing nothing). The Features tab in the management UI is therefore read-only.

### Consequences
- Removing an app from the codebase leaves an orphaned (but harmless) `features` row unless manually
  cleaned up — acceptable since apps are not expected to be removed as a matter of course.
- If per-action (CRUD-level) granularity is needed later, it is an additive change to `Registry` (more,
  finer-grained keys) plus corresponding `RequireFeature` call sites — the schema and resolver need no
  change.

### Key files
- `internal/rbac/features.go`
- `internal/rbac/service/bootstrap.go`

---

## ADR-008: Dedicated Admin area + user management (edit email, block login)

### Status
Accepted

### Context
ADR-001–007 delivered an admin-only **Access Management** screen, but it shipped as a fourth tab inside
the personal **Settings** page (`/settings`). Settings is self-service — a user's own theme, password, and
2FA — whereas Access Management administers *other* users, so the two were conceptually mismatched. Separately,
there was no way for an admin to perform basic identity administration: correcting another user's email, or
disabling a compromised/departed account.

### Decision
1. **Dedicated Admin area.** Admin-only features move out of Settings into a new top-level **Admin** section
   in the sidebar (rendered only when `is_admin`), with two real routes: **`/admin/users`** (the user roster:
   edit email, block/unblock, group assignment, per-user overrides) and **`/admin/access`** (Groups + Features
   policy). Settings returns to purely personal. The frontend `/admin/*` guard is cosmetic; the backend
   `RequireAdmin` subrouter is the real gate.
2. **Block login.** Users carry a `blocked` boolean (migration `07`). A blocked user is rejected at login
   (403 "account is blocked", checked after the password and *before* the 2FA branch), and blocking also
   deletes all their sessions via the existing `SessionRepository.DeleteByUserID` — the same invalidation
   path used for password-reset — so the block takes effect immediately on live sessions with no new
   per-request cost. Enforcement stays DB-per-request and JWT stays role/state-free (consistent with ADR-003).
3. **Admins are never blockable.** Blocking a user who is an admin returns **409**; an admin must have their
   admin access removed first. This makes self-block impossible and admin lockout unreachable by construction
   (mirrors the last-admin group guard in ADR-004). The guard reuses the already-wired `AccessResolver`.
4. **Domain split.** User-identity mutations (email, block/unblock) live in **authnz** (`/api/v1/admin/users/*`),
   since they touch account state and sessions (authnz-owned). The read model reuses rbac's existing
   `/access/users` roster — extended with one `blocked` column — so the Users page needs a single list call
   rather than a second identity endpoint plus client-side merge.

### Rationale
- Grouping admin-only capabilities in one area matches the mental model (and convention: GitHub/Linear-style
  admin vs. personal settings) and stops Access Management from being buried in unrelated personal settings.
- Reusing `DeleteByUserID` for block enforcement means no new middleware and no new per-request DB read —
  the blocked check at login plus session-deletion is both cheap and immediate.
- "Admins are never blockable" is the simplest rule that removes all lockout risk; the alternative
  ("allow except the last admin") needs a non-blocked-admin count and was rejected as unnecessary complexity.

### Alternatives Considered
| Option | Rejected because |
|---|---|
| Keep Access Management as a Settings tab, add user admin there | Perpetuates the personal-vs-admin mismatch and crowds a personal page with cross-user administration. |
| Enforce block via a new per-request middleware check | Adds a per-request user lookup; deleting sessions at block time + the login check already gives immediate, zero-staleness enforcement for free. |
| Put email/block endpoints in rbac alongside `/access/*` | Email and session termination are authnz/identity concerns, not authorization policy; keeping them in authnz preserves the domain boundary (rbac already reaches into `users` only for group joins). |
| Allow blocking admins except the last one | Requires counting non-blocked admins and a lockout guard; "never block an admin" is safer and simpler. |

### Consequences
- Adding `model.User.Blocked` is mandatory (every user query is `SELECT *`); the flag is wired through
  `db:transfer` so it survives cross-backend migration.
- To block an administrator you must first move them out of the admin group, then block — a deliberate,
  explicit two-step.
- New admin identity operations extend the authnz admin surface (`RegisterAdminRoutes`) on the same
  `RequireAdmin` subrouter as rbac's management API.

### Key files
- `db/migrations/{sqlite3,postgres}/07_add_user_blocked.*`, `internal/authnz/model/user.go`
- `internal/authnz/handler/{session.go, admin_user.go}` (login check, admin API + block guard)
- `internal/authnz/repository/*/user_repository.go` (`UpdateEmail`, `SetBlocked`)
- `internal/rbac/.../user_group_repository.go` + `internal/rbac/model/model.go` (`blocked` on the roster)
- `cmd/all-in-one/server/server.go` (wiring), `cmd/all-in-one/db/transfer.go` (backfill)
- `web/src/components/app-sidebar.svelte`, `web/src/routes/admin/**`, `web/src/lib/admin-api.ts`

---

## ADR-009: Admin content-management pages (pilot: Shortener)

### Status
Accepted

### Context
ADR-001–008 gave admins control over *access* (who can use which app-feature) and basic *identity*
administration (email, block). Neither gives an admin visibility into the actual **content** each app
produces — short links, listing topics/items, chat sessions/messages — all of which are owned per-user with
no existing owner-agnostic read or write path. As the platform grows, an admin needs to be able to see and
moderate that content (e.g. take down an abusive short link) without waiting on the owning user.

### Decision
1. **Scope: view + moderate, not full CRUD.** Admins can list every record across all owners and
   activate/deactivate or delete any record. They cannot create or edit content on another user's behalf —
   that stays exclusively the owning user's action.
2. **One admin page per app**, under `/admin/<app>`, mirroring `/admin/users`'s page-per-concern layout
   rather than a single tabbed "Content" page — clearer per-app UX and avoids one page's row-action set
   leaking into another's.
3. **Pilot one app before replicating.** Shortener ships first (single record type, already paginated,
   simplest ownership model — a separate `short_link_owners` join table rather than an owner column). Listing
   and chat follow later using the same recipe, not as part of this change.
4. **Gating: `is_admin` only, no feature-registry change.** These pages reuse the existing `RequireAdmin`
   subrouter (ADR-004) exactly like `/admin/users` and `/admin/access` — no new `admin_only` feature-registry
   entry, so access can't be delegated via RBAC groups/overrides, only the superuser flag.

**The recipe**, established by the Shortener pilot and intended to repeat per app:
- Add owner-agnostic sibling methods to the repository (`ListAll`, `DeleteByCode`, `SetActiveByCode` for
  shortener) alongside the existing owner-scoped ones — every app's repository was built assuming a single
  owning user, so this is the actual cost of each rollout, not the routing/UI.
- A new admin handler file (`admin.go`) with its own `RegisterAdminRoutes`, mounted with one line on the
  `adminRoutes` subrouter already wired in `server.go` — no per-endpoint admin re-check needed, since the
  subrouter is already `RequireAdmin`-gated.
- A `/admin/<app>` page mirroring the app's existing user-facing table/dialog patterns (e.g. Shortener's
  admin page reuses the same Switch-toggle + AlertDialog-delete UI as the user-facing `/shortener` page),
  plus an owner column since the admin view spans every user.

### Rationale
- Reusing the existing `RequireAdmin` subrouter and per-app `RegisterAdminRoutes` pattern (ADR-006, ADR-008)
  means each new admin page is additive wiring, not new infrastructure.
- Piloting one app first was deliberate: every app needed the same category of change (new owner-agnostic
  repo methods), so validating the recipe once on the simplest app (Shortener) de-risks the same recipe
  landing correctly on listing and chat, which have more record types and less mature ownership models (e.g.
  chat has no message-delete method at all yet, and listing's `TopicRepository` has no list-all).
- Scoping to view + moderate (not full CRUD) keeps the admin surface to what oversight actually requires;
  letting admins fabricate content as another user is a materially different (and unrequested) capability.

### Alternatives Considered
| Option | Rejected because |
|---|---|
| Single tabbed `/admin/content` page (Groups/Features-tab style, ADR-008) | Each app's records and row actions differ enough that one page's actions would be irrelevant clutter on another app's tab; per-app pages stay focused. |
| Ship listing + chat admin pages in the same change | Each app needs new owner-agnostic repository methods with no shared code path between apps; bundling triples the review surface for no coupling benefit. Piloting one app first lets the recipe be corrected before repeating it. |
| Gate via a new `admin_only` feature (e.g. `shortener-admin`) instead of `is_admin` | Every existing admin page (`/admin/users`, `/admin/access`) already gates on the superuser flag directly; introducing per-page delegable features here would be inconsistent and adds a registry entry with no current use case for delegation. |
| Full CRUD (admin can create/edit on a user's behalf) | Not requested and expands the trust/audit surface (an admin-authored record indistinguishable from the owner's) for no identified need; view + moderate covers oversight. |

### Consequences
- Every future app's admin page requires adding owner-agnostic repository methods first — this is the
  standing cost of the recipe, not a one-time Shortener cost.
- Listing and chat are explicitly deferred; chat additionally needs a message-delete method that doesn't
  exist yet, and listing's `TopicRepository` needs a global list-all (today only `ItemRepository.GetAll` is
  owner-agnostic).
- Because gating is `is_admin` only, these pages cannot be delegated to a non-admin "moderator" role without
  a future ADR revisiting decision 4.

### Key files
- `internal/shortener/repository/interfaces.go` + `sqlite/postgres/shortlink_repository.go` (`ListAll`,
  `DeleteByCode`, `SetActiveByCode`)
- `internal/shortener/handler/admin.go` (new), `internal/shortener/service/service.go` (`RegisterAdminRoutes`)
- `cmd/all-in-one/server/server.go` (`ssvc.RegisterAdminRoutes(adminRoutes)`)
- `web/src/lib/shortener-admin-api.ts`, `web/src/routes/admin/shortener/+page.svelte`,
  `web/src/components/app-sidebar.svelte`

---

## ADR-010: Rate Limiter and User Management listed in the Features tab, display-only

### Status
Accepted

### Context
`internal/rbac/features.go`'s `Registry` was never updated when the rate-limiter (`44abb71`) and
authnz admin user-management (`9692673`, shipped alongside RBAC itself) apps landed, so neither appeared
in the Access Management → Features tab, unlike listing/chat/shortener/access-management. This read as a
missing-registration bug. But per ADR-008 and ADR-009 (decision 4), both capabilities are intentionally
enforced by `RequireAdmin` only — `/api/v1/admin/users/*` and `/api/v1/ratelimit/*` are mounted on the
shared `adminRoutes` subrouter (`cmd/all-in-one/server/server.go`) alongside RBAC's own management API and
shortener's moderation endpoints, none of which consult `RequireFeature`. ADR-009 explicitly rejected
giving admin-only pages their own delegable feature key ("no current use case for delegation").

### Decision
Add `FeatureRateLimit` ("ratelimit") and `FeatureUserManagement` ("user-management") to `Registry` as
`AdminOnly: true`, purely so they render in the Features tab for visibility — mirroring how
`FeatureAccessManagement` already appears despite the RBAC management API itself being `RequireAdmin`-only
underneath. No route gating changes: their handlers keep using `RequireAdmin`, not `RequireFeature`. This
does not reverse ADR-009 decision 4, which was specifically about *enforcement* (should a non-admin group
be delegable access to these) — that answer is still no.

### Rationale
- `AdminOnly: true` features are filtered out of the Groups tab's grantable list
  (`web/src/components/access-management/GroupsTab.svelte`, `grantableFeatures`), so listing these two
  cannot be used, accidentally or otherwise, to delegate rate-limiter or user-management access to a
  non-admin group — the only path to either remains full admin membership.
- `access-management` already established the precedent that an admin-only capability can have a
  display-only Registry entry without becoming a "real" delegable gate (ADR-007's "meaningless
  grantable-but-unenforced feature" concern doesn't apply here, since these entries are never grantable in
  the UI at all).
- Bootstrap requires no change (ADR-007): `AdminOnly: true` rows are skipped by the regular-user
  auto-grant step (`service/bootstrap.go`), same as `access-management` today.

### Alternatives Considered
| Option | Rejected because |
|---|---|
| Also wire `authz.RequireFeature(...)` into the rate-limiter/authnz admin routes, making them genuinely delegable | Reverses ADR-009 decision 4's explicit "no current use case for delegation" call; a materially bigger, security-relevant change the user did not ask for — they only wanted the two apps visible in the Features list. |
| Leave the Registry unchanged | Now that the gap is identified, leaving two shipped apps permanently invisible in the one place admins go to see "what app-features exist" is confusing with no offsetting benefit, given a display-only entry is risk-free. |

### Consequences
- Any future purely-admin app-feature (no regular-user-facing surface) should default to this same
  pattern: `AdminOnly: true` Registry entry for visibility, `RequireAdmin` for enforcement, no
  `RequireFeature` wiring — unless a real delegation need is identified, which would warrant its own ADR.

### Key files
- `internal/rbac/features.go`
- `web/src/components/access-management/GroupsTab.svelte` (`grantableFeatures` filter)
