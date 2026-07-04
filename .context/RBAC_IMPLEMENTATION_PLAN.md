# RBAC / Access-Management Feature — Implementation Plan

> **Status tracker:** [RBAC_PROGRESS.md](RBAC_PROGRESS.md) (live phase-by-phase status)
> **Design rationale:** [docs/adr/ACCESS_MANAGEMENT_ADR.md](../docs/adr/ACCESS_MANAGEMENT_ADR.md) (the *why* behind each decision)
> This is the git-tracked, portable copy of the approved plan (the plan-mode copy lives at
> `~/.claude/plans/`, which is machine-local — do not rely on it for resume).

This plan is organized into **7 sequential phases**. Each phase has its own goal, file list, and
Definition of Done, so it can be implemented, tested, and committed **independently** — a new chat
session (on this machine or another) can resume at any phase boundary by reading just that phase's
section plus [RBAC_PROGRESS.md](RBAC_PROGRESS.md), without needing the whole document loaded.

```
Phase 1 (schema) → Phase 2 (rbac core, unwired) → Phase 3 (enforcement wiring)
    → Phase 4 (management API) ─┬→ Phase 6 (frontend) → Phase 7 (final verification)
    Phase 5 (transfer.go) ──────┘   (Phase 5 only needs Phase 2; can run any time after it)
```

## Context

The all-in-one app today has **authentication** (JWT + sessions) but **no authorization** — every
authenticated user can reach every app-feature (listing, chat, shortener). We are adding an admin-only
**Access Management** capability (UI under *Settings > Access Managements*) that controls, per user,
which app-features they can access.

This is a **greenfield** addition — no role/permission/RBAC code exists anywhere. The design reuses the
established module conventions (repository interface + factory + sqlite/postgres impls, per-subrouter
middleware, idempotent seed, dual-DB migrations, per-handler OTel metrics).

### Locked decisions (confirmed with user — do not re-litigate; see ADR for full rationale)

1. **One group per user** — `users.group_id` FK (nullable; NULL resolves to the built-in `regular-user` group).
2. **Groups are permission presets** over features. Two built-in groups: `admin` (superuser) + `regular-user` (default). Admins can create/edit/delete custom groups (e.g. `listing-group`).
3. **Per-user overrides are tri-state (grant AND revoke)** — precedence: **admin bypass > user override > group grant > default-deny**.
4. **Admin = full superuser** — bypasses ALL feature gates (every app now + future, plus Access Management). System must always keep ≥1 admin (lockout guard).
5. **Allow-all by default, except admin-only features** — realized purely through seeded data (regular-user is granted every non-admin feature at bootstrap), not a resolver special-case.

> Granularity is **per-app/feature** (not per-action/CRUD) — per-action is a documented future extension.
> Features are **code-defined** (they map to real apps), synced into a DB table so the UI can list them and FKs work.

---

## Phase 1 — Database Schema Foundation

**Goal:** RBAC tables exist in both backends. Nothing reads or writes them yet, so this phase has **zero
behavior change** to the running app — the safest possible first commit.
**Depends on:** nothing
**Files:**
- `db/migrations/sqlite3/06_add_rbac_tables.up.sql` / `.down.sql`
- `db/migrations/postgres/06_add_rbac_tables.up.sql` / `.down.sql`
- `internal/authnz/model/user.go` (add `GroupID` field)

Mirrors the `04_add_2fa_tables` style. Migrations auto-run on server start
(`cmd/all-in-one/server/server.go:84`) and in `db:seed` — no runner change needed.

**Ordering:** create `groups` **before** `ALTER TABLE users ADD group_id` (outgoing FK). On down, drop
`group_id` before `groups`; drop child tables (group_features, user_feature_overrides) before parents.

### `db/migrations/sqlite3/06_add_rbac_tables.up.sql`
```sql
CREATE TABLE IF NOT EXISTS features (
    id TEXT PRIMARY KEY, key TEXT NOT NULL UNIQUE, name TEXT NOT NULL,
    description TEXT, admin_only INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMP NOT NULL, updated_at TIMESTAMP NOT NULL);

CREATE TABLE IF NOT EXISTS groups (
    id TEXT PRIMARY KEY, name TEXT NOT NULL UNIQUE, description TEXT,
    is_builtin INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMP NOT NULL, updated_at TIMESTAMP NOT NULL);

CREATE TABLE IF NOT EXISTS group_features (
    group_id TEXT NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    feature_id TEXT NOT NULL REFERENCES features(id) ON DELETE CASCADE,
    created_at TIMESTAMP NOT NULL, PRIMARY KEY (group_id, feature_id));

CREATE TABLE IF NOT EXISTS user_feature_overrides (
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    feature_id TEXT NOT NULL REFERENCES features(id) ON DELETE CASCADE,
    allow INTEGER NOT NULL,            -- 1=grant, 0=revoke; "inherit" = no row
    created_at TIMESTAMP NOT NULL, updated_at TIMESTAMP NOT NULL,
    PRIMARY KEY (user_id, feature_id));

ALTER TABLE users ADD COLUMN group_id TEXT REFERENCES groups(id) ON DELETE SET NULL;

CREATE INDEX idx_group_features_feature_id         ON group_features(feature_id);
CREATE INDEX idx_user_feature_overrides_feature_id ON user_feature_overrides(feature_id);
CREATE INDEX idx_users_group_id                    ON users(group_id);
```
Down: drop the 3 indexes, `ALTER TABLE users DROP COLUMN group_id` (verified working on SQLite 3.50 /
go-sqlite3 v1.14.32 even with the outgoing FK), then drop the 4 tables in child→parent order
(user_feature_overrides, group_features, groups, features).

**Postgres twin:** identical, but `admin_only`/`is_builtin`/`allow` become `BOOLEAN` (`DEFAULT FALSE`); same ALTER + indexes.

Bool-as-INTEGER already works with sqlx here (`users.totp_enabled INTEGER` ↔ `model.User.TOTPEnabled bool`), so no scanner work.
`groups` is a valid unquoted table name in both SQLite and Postgres; fallback name `access_groups` if ever needed.

**`internal/authnz/model/user.go`:** add `GroupID *uuid.UUID \`json:"group_id,omitempty" db:"group_id"\``.
**MANDATORY** — every user query uses `SELECT *`; without this field sqlx throws `missing destination name
group_id` app-wide the moment the column exists.

### Definition of Done
- [ ] `go run main.go all-in-one db:migrate up` applies cleanly on a fresh SQLite DB
- [ ] `db:migrate down --steps 1` cleanly reverts (drops table/column in correct order)
- [ ] Same up/down cycle verified against Postgres
- [ ] `go build ./...` succeeds (confirms `GroupID` field satisfies `SELECT *`)
- [ ] Existing authnz tests still pass unmodified

---

## Phase 2 — RBAC Core Package (data + business logic, not wired in)

**Goal:** `internal/rbac` exists as a fully self-contained, fully-tested package. Server behavior is
**still unchanged** — nothing outside this package calls into it yet.
**Depends on:** Phase 1
**Files:**
```
internal/rbac/
  features.go            # canonical code registry + feature-key constants
  model/model.go         # Feature, Group, GroupFeature, UserFeatureOverride, UserAccessRow
  repository/            # interface.go, factory.go, sqlite.go/postgres.go adapters, sqlite/, postgres/
  service/resolver.go    # permission resolver (precedence engine)
  service/bootstrap.go   # idempotent bootstrap
```

**`features.go`** — source of truth new apps append to:
```go
const ( FeatureListing="listing"; FeatureChat="chat"; FeatureShortener="shortener"; FeatureAccessManagement="access-management" )
var Registry = []model.Feature{
    {Key: FeatureListing, Name: "Listings", AdminOnly: false},
    {Key: FeatureChat, Name: "Chats", AdminOnly: false},
    {Key: FeatureShortener, Name: "Shortener", AdminOnly: false},
    {Key: FeatureAccessManagement, Name: "Access Management", AdminOnly: true},
}
```

**Models** — structs with `json`+`db` tags (`Feature`, `Group{…, FeatureKeys []string \`db:"-"\`}`, `GroupFeature`, `UserFeatureOverride{Allow bool}`, `UserAccessRow` for the users↔group join).

**Repository interfaces** (`repository/interface.go`) — thin, RBAC-owned (do **not** extend authnz's
`UserRepository`, whose `Update` hard-lists columns): `FeatureRepository` (List/GetByKey/Upsert),
`GroupRepository` (CRUD + `EnsureBuiltin`), `GroupFeatureRepository`
(ListKeysByGroup/HasGrantByKey/Grant/ReplaceGrants), `OverrideRepository`
(ListByUser/GetByKey/Set/Delete/ReplaceForUser), `UserGroupRepository`
(GetGroupID/AssignGroup/CountByGroup/ListUsersWithGroup), aggregate `Storage`. Concrete impls copy
`internal/authnz/repository/sqlite/*` precisely — `getExecCtx(db, opts...)` + `queryOptions{trx}` helper
(from `session_repository.go`), `?`/`$N` placeholders, `INSERT OR IGNORE`/`ON CONFLICT DO NOTHING`,
`httpHelper.ErrNotFound` on `sql.ErrNoRows`. RBAC shares the same `*sqlx.DB` (`store.DB()`); its
`Storage.Close()` is a **no-op** (top-level storage owns the connection). `CreateTrx(ctx)` mirrors the
authnz session repo for transactional Replace* ops.

**Resolver** (`service/resolver.go`):
```go
func (r *Resolver) IsAdmin(ctx, userID uuid.UUID) (bool, error)
func (r *Resolver) CanAccess(ctx, userID uuid.UUID, featureKey string) (bool, error)
func (r *Resolver) EffectiveFeatures(ctx, userID uuid.UUID) (isAdmin bool, groupID, groupName string, featureKeys []string, err error)
```
Precedence:
```
group := userGroupID ?? regularGroupID
if group == adminGroupID:                 return true   // admin bypass (ignores overrides)
if ovr := override(user, key); ovr != nil: return ovr.Allow  // tri-state
if group grants key:                       return true   // group grant
return false                                              // default deny
```
Cache the two built-in group IDs (immutable post-bootstrap). Per-request cost ≈ 2–3 indexed lookups —
consistent with the existing per-request `sessionRepo.Get` in `jwt.go:144`. No user-permission cache
initially (future: short-TTL cache invalidated on mgmt writes).

**Bootstrap** (`service/bootstrap.go`) — `Bootstrap(ctx, userRepo)`, idempotent, safe to call on every
server start:
1. **Sync features:** `Upsert` each `Registry` entry by `key` (do NOT auto-flip `admin_only` on existing rows — log drift).
2. **Ensure built-ins:** `EnsureBuiltin("admin")`, `EnsureBuiltin("regular-user")` (`is_builtin=1`).
3. **Grant regular-user all non-admin features** (insert-ignore) → realizes allow-all + auto-grants newly shipped non-admin features each boot.
4. **Ensure ≥1 admin:** if `CountByGroup(admin)==0`, `FindByUsername(cfg.RBAC.AdminUsername)` → `AssignGroup(admin)`.

NULL-as-default recommended: don't mass-backfill NULL→regular-user; the resolver already treats NULL as
regular-user, keeping boot O(1).

### Definition of Done
- [ ] Resolver precedence-matrix table test passes: admin-bypass-ignores-deny, override allow, override
      deny, group-grant, group-no-grant→deny, admin-only×(admin/non-admin), NULL-group→regular, unknown
      feature→deny
- [ ] Repository integration tests pass against a real temp SQLite DB (grants, overrides upsert/replace,
      `CountByGroup`, `ListUsersWithGroup` join, `ON DELETE SET NULL`/`CASCADE` behavior)
- [ ] Bootstrap idempotency test passes (2nd run is a no-op; adding a new `Registry` entry auto-grants it
      to regular-user on the next run)
- [ ] `.mockery.yaml` updated with `internal/rbac/repository` interfaces; mocks regenerated
- [ ] `go build ./... && go vet ./...` pass; nothing outside `internal/rbac` changed

---

## Phase 3 — Enforcement Wiring (the "flip the switch" phase)

**Goal:** Authorization is now actually enforced end-to-end. **Highest-risk phase** — after this merges,
non-admin users' access is gated for real, so the regression check below is not optional.
**Depends on:** Phase 2
**Files:**
- `internal/rbac/middleware/authz.go`
- `internal/config/config.go` (+ `config/config.yml`)
- `cmd/all-in-one/server/server.go`
- `cmd/all-in-one/db/seed.go` (bootstrap call site)

**Middleware** (`middleware/authz.go`) — `NewAuthz(resolver, cfg)` with `RequireFeature(key)
mux.MiddlewareFunc` and `RequireAdmin(next)`:
1. `auth.GetUserFromContext` → `SendUnauthorized` if absent.
2. **Direct-auth branch:** if `claims.SessionID == "direct-auth"` → allow per `cfg.RBAC.DirectAuthIsAdmin`
   (avoids the `uuid.Parse(username)` failure; that dev bypass already forgoes auth).
3. `CanAccess`/`IsAdmin`; on deny → `httpHelper.SendError(w,"forbidden",403)` + increment
   `aio.rbac.access.denied{feature,reason}`.

**`server.go` (replace lines 143-150):** split the single authenticated subrouter into siblings.
```go
rbacSvc, _ := rbacsvc.NewService(ctx, db, s.config, s.log)
if err := rbacSvc.Bootstrap(ctx, asvc.Store.UserRepo()); err != nil { return err }
asvc.Handler.SetAccessResolver(rbacSvc.Resolver)          // for /users/me (Phase 4)
authz := rbacMw.NewAuthz(rbacSvc.Resolver, s.config)
jwtMiddleware := middleware.NewJWTMiddleware(s.config, asvc.Store.SessionRepo())

selfRoutes := api.NewRoute().Subrouter()                  // authnz self-service, UNGATED
selfRoutes.Use(jwtMiddleware.JWTAuth); asvc.RegisterAuthenticatedRoutes(selfRoutes)

mkGated := func(feature string) *mux.Router {
    sr := api.NewRoute().Subrouter(); sr.Use(jwtMiddleware.JWTAuth); sr.Use(authz.RequireFeature(feature)); return sr }
lsvc.RegisterAuthenticatedRoutes(mkGated(rbac.FeatureListing))
csvc.RegisterAuthenticatedRoutes(mkGated(rbac.FeatureChat))
ssvc.RegisterAuthenticatedRoutes(mkGated(rbac.FeatureShortener))

adminRoutes := api.NewRoute().Subrouter()                 // RBAC mgmt API (Phase 4)
adminRoutes.Use(jwtMiddleware.JWTAuth); adminRoutes.Use(authz.RequireAdmin); rbacSvc.RegisterAdminRoutes(adminRoutes)
```
**Why per-app subrouters, not a path-prefix map:** routes are NOT cleanly prefixed — `authnz` owns
`/users*`+`/sessions`, but **chat also owns `/users/search`** (`chat/handler/handler.go:55`); a
`PathPrefix("/users")` gate would misfire. Per-app subrouters gate by exact route with zero change to each
app's `RegisterAuthenticatedRoutes` signature.

**Config** — add `RBAC RBACConfig \`mapstructure:"rbac"\`` to `Config`:
```go
type RBACConfig struct {
    AdminUsername     string `mapstructure:"admin_username"`
    DirectAuthIsAdmin bool   `mapstructure:"direct_auth_is_admin"`
}
```
`SetDefault("rbac.admin_username","admin")`, `SetDefault("rbac.direct_auth_is_admin",true)`, plus explicit
`BindEnv` for both (underscore-key convention, `config.go:152-170`). Add matching `rbac:` block to
`config/config.yml`.

**Seed:** call `rbacSvc.Bootstrap(ctx, ...)` in `cmd/all-in-one/db/seed.go` after `authnzSeed.SeedUsers`
(`seed.go:57`) — independent of `SeedUsers`' early-return so existing installs still get bootstrapped.

### Definition of Done
- [ ] Middleware unit tests: `RequireFeature` allow/deny (403+metric), `RequireAdmin`, direct-auth branch
      (both flag states), missing-context→401
- [ ] Server boots; logs show bootstrap created `admin`+`regular-user` groups and the 4 features
- [ ] Smoke test: seeded `admin` can reach everything; seeded `user`/`demo` (both default to
      `regular-user`) can still reach listing/chat/shortener — **regression check**, since this phase is
      what could accidentally lock out existing users
- [ ] `db:seed` run on a fresh DB completes and bootstrap logs fire

---

## Phase 4 — `/users/me` Extension + Admin Management API

**Goal:** Full admin-facing REST surface for managing groups/users/overrides exists and is guarded; the
frontend (Phase 6) has something to call.
**Depends on:** Phase 3
**Files:**
- `internal/authnz/handler/user.go` (+ `internal/authnz/handler/handler.go` for the `AccessResolver` field)
- `internal/rbac/service/service.go` (management logic + guards)
- `internal/rbac/handler/*` (handler.go, access.go, metrics.go)

**`/users/me` extension:** response wrapper `CurrentUserResponse{ model.User; IsAdmin bool; Group
*GroupRef; Features []string }`. Cross-package wiring without an import cycle: authnz/handler declares an
`AccessResolver` interface returning only primitives; `*rbac/service.Resolver` satisfies it structurally;
`server.go` calls `asvc.Handler.SetAccessResolver(...)`. In `GetCurrentUser` (`user.go:35`), after
`Find(uid)`, call `EffectiveFeatures` and assemble the response. Guard the direct-auth path (non-UUID
`UserID`) — return a minimal payload instead of erroring; this also fixes the pre-existing 401 direct-auth
hits on `/users/me`.

**Management API** (admin-only, on `adminRoutes` from Phase 3, under `/api/v1/access/*`):

| Method | Path | Body → Returns |
|---|---|---|
| GET | `/access/features` | → `[]Feature` |
| GET | `/access/groups` | → `[]Group{…,feature_keys[]}` |
| POST | `/access/groups` | `{name,description,feature_keys[]}` → 201 Group |
| GET/PUT/DELETE | `/access/groups/{id}` | get / update / delete (409 if built-in) |
| PUT | `/access/groups/{id}/features` | `{feature_keys[]}` → replace grants (txn) |
| GET | `/access/users` | → `[]UserAccess{id,username,email,group,is_admin}` |
| PUT | `/access/users/{id}/group` | `{group_id: string\|null}` → 200 / 409 lockout |
| GET/PUT | `/access/users/{id}/overrides` | list / replace `[{feature_key,allow}]` (txn) |

**Guards (`service.go`):** (a) **last-admin** — reassigning the current admin away requires
`CountByGroup(admin) > 1`, else 409; (b) **built-in protection** — delete/rename `admin`/`regular-user` →
409 (editing `regular-user` *grants* is allowed; editing `admin` grants is a no-op, reject with hint); (c)
overrides can't lock out admins (guaranteed by precedence). `ReplaceGrants`/`ReplaceForUser` run in a
`CreateTrx` transaction.

**OTel (`handler/metrics.go`):** `observability.Meter("rbac")` → `aio.rbac.access.denied{feature,reason}`
(from Phase 3 middleware); optional `aio.rbac.groups.changed{action}`, `aio.rbac.user_group.assigned`.

Every handler gets swagger godoc annotations (`@Tags access-management`).

### Definition of Done
- [ ] Handler tests: happy paths + 409 guard paths (mocked service)
- [ ] `/api/v1/users/me` returns correct `is_admin`/`group`/`features[]` for an admin, a regular user, and
      a direct-auth request
- [ ] Full curl walkthrough of every `/access/*` endpoint (see Phase 7 script)
- [ ] Swagger docs regenerated and render the new endpoints
- [ ] `docs/metrics.md` updated with `aio_rbac_access_denied_total` (+ any admin-action counters added)

---

## Phase 5 — Data-Integrity Backfill (`db:transfer`)

**Goal:** The sqlite↔postgres transfer tool doesn't silently drop RBAC data.
**Depends on:** Phase 2 only (needs the schema+tables; does not touch enforcement, so it can be done any
time after Phase 2 — in parallel with Phase 3/4 if convenient).
**Files:** `cmd/all-in-one/db/transfer.go`

Add `group_id` to `userRow` + the users `SELECT`/`INSERT` column lists. Add the 4 new tables to
`transferData`/`readAll`/`writeAll`/`logTransferSummary`, using the existing `boolInt`/`boolForDst`
helpers for the bool columns. **FK order matters:** `features`+`groups` must transfer **before** `users`
(users→groups FK); `group_features`+`user_feature_overrides` **after** `users`+`features`.

### Definition of Done
- [ ] `group_id` present in `userRow` struct and both SELECT/INSERT statements
- [ ] All 4 new tables wired into the transfer pipeline in the correct FK order
- [ ] Manual round-trip test: seed SQLite, `db:transfer` to Postgres, verify row counts match on all 4 new
      tables plus non-null `users.group_id` values

---

## Phase 6 — Frontend

**Goal:** Admins can manage access through *Settings > Access Management*; non-admins see a correctly
filtered sidebar.
**Depends on:** Phase 4 (needs the management API to exist)
**Files:**
- `web/src/lib/rbac-api.ts` (new)
- `web/src/lib/stores/auth.ts` (new)
- `web/src/components/app-sidebar.svelte`
- `web/src/routes/settings/+page.svelte`
- `web/src/components/access-management/*` (new)

**`rbac-api.ts`** (mirror `shortener-api.ts`) — TS interfaces (`Feature,Group,GroupInput,UserAccess,UserOverride`)
+ typed funcs (`listFeatures/listGroups/createGroup/updateGroup/deleteGroup/setGroupFeatures/listUsers/assignUserGroup/getUserOverrides/setUserOverrides`).

**`stores/auth.ts`** (mirror `stores/theme.ts`) — `writable<AuthUser|null>` with
`{name,username,email,is_admin,group,features[]}`, `loadAuth()` (single `/users/me` fetch), `hasFeature()`.
Replaces the 3+ ad-hoc `/users/me` fetches (sidebar, settings load, chat).

**Sidebar** — tag `generalItems` with feature keys; render only where `features.includes(key)` (cosmetic;
backend still enforces).

**Settings** — conditionally (`is_admin`) push `{id:'access-management',label:'Access
Management',icon:'🛡️'}` to `navItems`; add an `{:else if activeSection==='access-management'}` branch
rendering `<AccessManagement/>`.

**`<AccessManagement>`** — shadcn Tabs with 3 panes:
- **Features** — read-only `Table` (key, name, admin_only badge, granted-to-regular-user?). No edit.
- **Groups** — `data-table`; "New Group" `Dialog` (name/description + `Checkbox` feature list); delete
  `AlertDialog` disabled for built-ins.
- **Users** — `data-table` with inline `Select` to reassign group (surfaces 409 last-admin via `toast`);
  "Overrides" `Dialog` with a per-feature tri-state `Select` (Inherit/Allow/Deny).

CRUD reference: `web/src/routes/listing/topics/+page.svelte`.

### Definition of Done
- [ ] `npm run build` succeeds, no type errors
- [ ] Manual browser walkthrough (per CLAUDE.md UI-testing requirement, using `/run` or `npm run dev`): as
      admin — create a group, reassign a user, set an override, confirm a 409 toast on attempting to
      remove the last admin; as a restricted user — confirm the sidebar hides gated apps and the Access
      Management section is absent from Settings

---

## Phase 7 — Final Verification & Docs Sweep

**Goal:** Everything works end-to-end on both storage backends; docs match the shipped implementation.
**Depends on:** all prior phases

```bash
go build ./... && go test ./internal/rbac/... ./internal/authnz/...
mockery
go run main.go all-in-one server      # migrations 06 apply; bootstrap logs groups/features
```
Then exercise:
```bash
# create a listing-only group, move 'user' into it
curl -XPOST /api/v1/access/groups -d '{"name":"listing-only","feature_keys":["listing"]}'
curl -XPUT  /api/v1/access/users/<user_id>/group -d '{"group_id":"<listing-only-id>"}'
curl /api/v1/topics   # 200
curl /api/v1/chats    # 403 forbidden
# per-user override beats group
curl -XPUT /api/v1/access/users/<user_id>/overrides -d '{"overrides":[{"feature_key":"chat","allow":true}]}'
curl /api/v1/chats    # 200
# lockout guard
curl -XPUT /api/v1/access/users/<admin_id>/group -d '{"group_id":"<regular-id>"}'   # 409
curl /api/v1/users/me  # is_admin + group + features[]
```
Repeat the full walkthrough with `storage.type: postgres`.

### Definition of Done
- [ ] Full backend test suite green: `go build ./...` && `go test ./...`
- [ ] Full curl verification script passes on both SQLite and Postgres
- [ ] `docs/metrics.md`, swagger, and the ADR all reflect the final implementation (amend the ADR only if
      an implementation deviated from a locked decision — see ADR intro note)
- [ ] [RBAC_PROGRESS.md](RBAC_PROGRESS.md) checklist fully checked off; status flipped to done

---

## Appendix A: Cross-cutting gotchas (tagged by phase)

1. **`model.User.GroupID` field** *(Phase 1)* — without it `SELECT *` breaks app-wide.
2. **Routes are NOT cleanly prefixed** *(Phase 3)* — chat owns `/users/search`, colliding with authnz
   `/users`. Gate via per-app sibling subrouters, not `PathPrefix`.
3. **Direct-auth bypass** *(Phase 3 middleware, Phase 4 `/users/me`)* — fabricates `UserID=username`
   (non-UUID, no DB row). Both must short-circuit on `SessionID=="direct-auth"` using
   `rbac.direct_auth_is_admin` (default true).
4. **`db:transfer` column/table lists** *(Phase 5)* — must add `group_id` + the 4 tables in correct FK
   order or transfers silently drop RBAC data.
5. **Mock regen** *(Phase 2, and again if Phase 4 adds a mockable `Resolver`/service interface)*.
6. **Admin lockout** *(Phase 2 resolver precedence, Phase 4 API guard)* — last-admin guard, built-in
   delete/rename block, admin-immune-to-override (guaranteed by precedence order, ADR-002/ADR-004).
7. **Deleted group** *(schema behavior, Phase 1)* — `ON DELETE SET NULL` → members silently revert to the
   regular-user default; `group_features` cascades. No orphaned users.
8. **New-app onboarding** *(future maintenance, not part of this feature's phases)* — (a) append a
   `Feature` to `rbac.Registry`; (b) wrap the app's routes with `mkGated("<key>")` in `server.go`; (c) map
   the sidebar menu entry to the feature key. Bootstrap auto-registers + auto-grants non-admin features on
   next boot.
9. **Doc/reality mismatch:** follow the codebase's hand-written parameterized SQL (CLAUDE.md mentions
   squirrel, but nothing in the codebase actually uses it).
