# RBAC / Access-Management Feature — Implementation Plan

> Status: **not started** (planning complete, approved). See [RBAC_PROGRESS.md](RBAC_PROGRESS.md) for live status.
> This is the git-tracked, portable copy of the approved plan (the plan-mode copy lives at
> `~/.claude/plans/` which is machine-local). Keep this file authoritative for cross-session/cross-machine resume.

## Context

The all-in-one app today has **authentication** (JWT + sessions) but **no authorization** — every
authenticated user can reach every app-feature (listing, chat, shortener). We are adding an admin-only
**Access Management** capability (UI under *Settings > Access Managements*) that controls, per user,
which app-features they can access.

This is a **greenfield** addition — no role/permission/RBAC code exists anywhere. The design reuses the
established module conventions (repository interface + factory + sqlite/postgres impls, per-subrouter
middleware, idempotent seed, dual-DB migrations, per-handler OTel metrics).

### Locked decisions (confirmed with user)

1. **One group per user** — `users.group_id` FK (nullable; NULL resolves to the built-in `regular-user` group).
2. **Groups are permission presets** over features. Two built-in groups: `admin` (superuser) + `regular-user` (default). Admins can create/edit/delete custom groups (e.g. `listing-group`).
3. **Per-user overrides are tri-state (grant AND revoke)** — precedence: **admin bypass > user override > group grant > default-deny**.
4. **Admin = full superuser** — bypasses ALL feature gates (every app now + future, plus Access Management). System must always keep ≥1 admin (lockout guard).
5. **Allow-all by default, except admin-only features** — features carry an `admin_only` flag. Non-admin features are granted to `regular-user` at bootstrap (and auto-granted when new apps ship); admin-only features (e.g. `access-management`) are reachable only by admins. *Realized purely through seeded data, not a resolver special-case.*

> Granularity is **per-app/feature** (not per-action/CRUD) — per-action is a documented future extension.
> Features are **code-defined** (they map to real apps), synced into a DB table so the UI can list them and FKs work.

---

## 1. Database schema — migration `06_add_rbac_tables`

Four files (up+down × sqlite3+postgres), mirroring the `04_add_2fa_tables` style. Migrations auto-run on
server start (`cmd/all-in-one/server/server.go:84`) and in `db:seed` — no runner change.

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
`groups` is a valid unquoted table name in both SQLite and Postgres (GROUPS is non-reserved); fallback name `access_groups` if ever needed.

---

## 2. Backend package — `internal/rbac/`

Layout mirrors `internal/authnz/`:
```
internal/rbac/
  features.go            # canonical code registry + feature-key constants
  model/model.go         # Feature, Group, GroupFeature, UserFeatureOverride, UserAccessRow
  repository/            # interface.go, factory.go, sqlite.go/postgres.go adapters, sqlite/, postgres/
  service/              # service.go (mgmt logic + guards), resolver.go, bootstrap.go
  middleware/authz.go    # RequireFeature / RequireAdmin
  handler/              # handler.go (RegisterAdminRoutes), access.go, metrics.go, mocks/
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

---

## 3. Permission resolver + precedence (`service/resolver.go`)

```go
func (r *Resolver) IsAdmin(ctx, userID uuid.UUID) (bool, error)
func (r *Resolver) CanAccess(ctx, userID uuid.UUID, featureKey string) (bool, error)
func (r *Resolver) EffectiveFeatures(ctx, userID uuid.UUID) (isAdmin bool, groupID, groupName string, featureKeys []string, err error)
```

`CanAccess` precedence:
```
group := userGroupID ?? regularGroupID
if group == adminGroupID:                 return true   // admin bypass (ignores overrides)
if ovr := override(user, key); ovr != nil: return ovr.Allow  // tri-state
if group grants key:                       return true   // group grant
return false                                              // default deny
```
"Allow-all for non-admin features" comes from **bootstrap data** (regular-user granted every non-admin
feature), not resolver logic. Admin-bypass above overrides is what makes admins immune to lockout. Cache
the two built-in group IDs (immutable post-bootstrap). Per-request cost ≈ 2–3 indexed lookups — consistent
with the existing per-request `sessionRepo.Get` in `jwt.go:144`. No user-permission cache initially
(future: short-TTL cache invalidated on mgmt writes).

---

## 4. Enforcement middleware + `server.go` wiring

**`middleware/authz.go`** — `NewAuthz(resolver, cfg)` with `RequireFeature(key) mux.MiddlewareFunc` and `RequireAdmin(next)`:
1. `auth.GetUserFromContext` → `SendUnauthorized` if absent.
2. **Direct-auth branch:** if `claims.SessionID == "direct-auth"` → allow per `cfg.RBAC.DirectAuthIsAdmin` (avoids the `uuid.Parse(username)` failure; that dev bypass already forgoes auth).
3. `CanAccess`/`IsAdmin`; on deny → `httpHelper.SendError(w,"forbidden",403)` + increment `aio.rbac.access.denied{feature,reason}`.

**`server.go` (replace lines 143-150):** split the single authenticated subrouter into siblings — the
enforcement seam. The sibling-`NewRoute().Subrouter()` pattern is proven by the public/authenticated split.
```go
rbacSvc, _ := rbacsvc.NewService(ctx, db, s.config, s.log)
if err := rbacSvc.Bootstrap(ctx, asvc.Store.UserRepo()); err != nil { return err }
asvc.Handler.SetAccessResolver(rbacSvc.Resolver)          // for /users/me
authz := rbacMw.NewAuthz(rbacSvc.Resolver, s.config)
jwtMiddleware := middleware.NewJWTMiddleware(s.config, asvc.Store.SessionRepo())

selfRoutes := api.NewRoute().Subrouter()                  // authnz self-service, UNGATED
selfRoutes.Use(jwtMiddleware.JWTAuth); asvc.RegisterAuthenticatedRoutes(selfRoutes)

mkGated := func(feature string) *mux.Router {
    sr := api.NewRoute().Subrouter(); sr.Use(jwtMiddleware.JWTAuth); sr.Use(authz.RequireFeature(feature)); return sr }
lsvc.RegisterAuthenticatedRoutes(mkGated(rbac.FeatureListing))
csvc.RegisterAuthenticatedRoutes(mkGated(rbac.FeatureChat))
ssvc.RegisterAuthenticatedRoutes(mkGated(rbac.FeatureShortener))

adminRoutes := api.NewRoute().Subrouter()                 // RBAC mgmt API
adminRoutes.Use(jwtMiddleware.JWTAuth); adminRoutes.Use(authz.RequireAdmin); rbacSvc.RegisterAdminRoutes(adminRoutes)
```
**Why per-app subrouters, not a path-prefix map:** routes are NOT cleanly prefixed — `authnz` owns
`/users*`+`/sessions`, but **chat also owns `/users/search`** (`chat/handler/handler.go:55`); a
`PathPrefix("/users")` gate would misfire. Per-app subrouters gate by exact route with zero change to each
app's `RegisterAuthenticatedRoutes` signature. `/users/me`, `/users/reset_password`, `/users/2fa/*` stay
ungated (account self-service).

---

## 5. JWT stays role-free

Keep `createAccessToken` (`session.go:254`) as-is (`sub,user_id,username,email`). The JWT middleware already
hits the DB per request (`jwt.go:144`), so DB-resolved authz adds only cheap indexed reads on an
already-DB-bound path, and gives **zero staleness** — a demoted admin or moved user loses access on the very
next request. Embedding role would create a ~15-min stale-privilege window.

---

## 6. `/users/me` extension

- **`internal/authnz/model/user.go`:** add `GroupID *uuid.UUID \`json:"group_id,omitempty" db:"group_id"\``. **MANDATORY** — every user query uses `SELECT *`; without this field sqlx throws `missing destination name group_id` app-wide.
- Response wrapper `CurrentUserResponse{ model.User; IsAdmin bool; Group *GroupRef; Features []string }`.
- **Cross-package wiring (no import cycle):** authnz/handler declares an `AccessResolver` interface returning only primitives; `*rbac/service.Resolver` satisfies it structurally; `server.go` calls `asvc.Handler.SetAccessResolver(...)`.
- **`GetCurrentUser` (`user.go:35`):** after `Find(uid)`, call `EffectiveFeatures` and assemble the response. Guard the direct-auth path (non-UUID `UserID`) — return minimal payload (`is_admin` per config, `features`=all non-admin keys if admin else `[]`); this also fixes the pre-existing 401 direct-auth hits on `/users/me`. Nil-resolver defensive fallback → `is_admin=false, features=[]`.

---

## 7. RBAC management API (admin-only, `/api/v1/access/*`)

On `adminRoutes` (RequireAdmin). Envelope responses; guards return **409**; swagger godoc on every handler (`@Tags access-management`).

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
409 (editing `regular-user` *grants* is allowed = adjusts the global default; editing `admin` grants is a
no-op, reject with hint); (c) overrides can't lock out admins (precedence guarantees it).
`ReplaceGrants`/`ReplaceForUser` run in a `CreateTrx` transaction (delete-then-insert).

**OTel (`handler/metrics.go`, mirror authnz metrics.go):** `observability.Meter("rbac")` →
`aio.rbac.access.denied{feature,reason}` (from middleware); optional `aio.rbac.groups.changed{action}`,
`aio.rbac.user_group.assigned`.

---

## 8. Seed + idempotent bootstrap (`service/bootstrap.go`)

`Bootstrap(ctx, userRepo)` — idempotent, runs on **every** server start (after `MigrateUp`) and in `db:seed`:
1. **Sync features:** `Upsert` each `Registry` entry by `key` (do NOT auto-flip `admin_only` on existing rows — log drift).
2. **Ensure built-ins:** `EnsureBuiltin("admin")`, `EnsureBuiltin("regular-user")` (`is_builtin=1`).
3. **Grant regular-user all non-admin features** (insert-ignore) → realizes allow-all + auto-grants newly shipped non-admin features each boot. Admin group granted nothing (bypass).
4. **Ensure ≥1 admin:** if `CountByGroup(admin)==0`, `FindByUsername(cfg.RBAC.AdminUsername)` → `AssignGroup(admin)`. Zero-count guard → runs only on fresh install / genuine lockout.

**NULL-as-default (recommended):** don't mass-backfill NULL→regular-user; the resolver treats NULL as
regular-user, keeping boot O(1).

**Call sites:** `server.go Start()` and `db.Run()` after `SeedUsers` (`seed.go:57`) — Bootstrap is
independent of `SeedUsers`' early-return so existing installs still get bootstrapped.

---

## 9. Config

**`internal/config/config.go`:** add `RBAC RBACConfig \`mapstructure:"rbac"\`` to `Config`:
```go
type RBACConfig struct {
    AdminUsername     string `mapstructure:"admin_username"`
    DirectAuthIsAdmin bool   `mapstructure:"direct_auth_is_admin"`
}
```
`Load()`: `SetDefault("rbac.admin_username","admin")`, `SetDefault("rbac.direct_auth_is_admin",true)`, plus
explicit `BindEnv("rbac.admin_username","ALLINONE_RBAC_ADMIN_USERNAME")` and
`BindEnv("rbac.direct_auth_is_admin","ALLINONE_RBAC_DIRECT_AUTH_IS_ADMIN")` (underscore-key convention,
config.go:152-170). Add matching `rbac:` block to `config/config.yml`.

---

## 10. Frontend (SvelteKit 2 SPA, Svelte 5 runes)

- **`web/src/lib/rbac-api.ts`** (new; mirror `shortener-api.ts`) — TS interfaces (`Feature,Group,GroupInput,UserAccess,UserOverride`) + typed funcs (`listFeatures/listGroups/createGroup/updateGroup/deleteGroup/setGroupFeatures/listUsers/assignUserGroup/getUserOverrides/setUserOverrides`) over `apiGet/apiPost/apiPut/apiDelete`, unwrapping `body.data`.
- **`web/src/lib/stores/auth.ts`** (new; mirror `stores/theme.ts`) — `writable<AuthUser|null>` with `{name,username,email,is_admin,group,features[]}`, `loadAuth()` (single `/users/me` fetch), `hasFeature()`. Replaces the 3+ ad-hoc `/users/me` fetches (sidebar, settings load, chat).
- **Sidebar** (`app-sidebar.svelte`) — tag `generalItems` with feature keys; render only where `features.includes(key)` (cosmetic; backend still enforces).
- **Settings** (`settings/+page.svelte`) — conditionally (`is_admin`) push `{id:'access-management',label:'Access Management',icon:'🛡️'}` to `navItems`; add `{:else if activeSection==='access-management'}` rendering a modular `<AccessManagement/>`.
- **`web/src/components/access-management/`** (new) — `AccessManagement.svelte` hosting shadcn **Tabs**:
  - **Features** — read-only `Table` (key, name, admin_only badge, granted-to-regular-user?). No edit (features are code-owned).
  - **Groups** — `data-table`; "New Group" `Dialog` (name/description + `Checkbox` feature list); edit reuses dialog; delete `AlertDialog` **disabled for built-ins**.
  - **Users** — `data-table` with inline `Select` to reassign group (surfaces 409 last-admin via `toast`); "Overrides" `Dialog` with a per-feature tri-state `Select` (Inherit=delete / Allow / Deny) → `setUserOverrides`.
  - CRUD reference: `web/src/routes/listing/topics/+page.svelte`.

---

## 11. Tests (table-driven, testify, mockery)

- **Resolver precedence matrix** (`service/resolver_test.go`) — admin-bypass-ignores-deny, override allow/deny, group grant/no-grant, admin-only×(admin/non-admin), NULL-group→regular, unknown feature→deny.
- **Middleware** (`middleware/authz_test.go`) — RequireFeature allow/deny(403+metric), RequireAdmin, direct-auth branch (both flag states), missing-context→401 (httptest + mocked `Resolver` interface).
- **Repository** (sqlite integration vs temp DB + migrations, pattern from shortener `integration_test.go`) — grants, overrides upsert/replace, `CountByGroup`, join, `ON DELETE SET NULL/CASCADE`.
- **Handlers** — happy paths + guard 409s (mocked service).
- **Bootstrap** — fresh→populated, second-run no-op (idempotency), new-Registry-feature auto-granted.
- **`.mockery.yaml`** — add `internal/rbac/repository` + `internal/rbac/service` (Resolver) blocks → output `internal/rbac/handler/mocks` (+ `middleware/mocks`); run `mockery`.

---

## 12. Docs

- **ADR** `docs/adr/ACCESS_MANAGEMENT_ADR.md` — the 5 decisions, precedence rule, schema, JWT-role-free rationale, new-app onboarding.
- **`.context/`** this plan + `RBAC_PROGRESS.md`; add both to `.context/README.md` index.
- **`docs/metrics.md`** — add RBAC section: `aio_rbac_access_denied_total` (Counter, labels `feature`,`reason`).
- **Swagger** — annotate `/access/*` + widened `/users/me`; regenerate per `.context/SWAGGER_INTEGRATION.md`.

---

## 13. Migration-safety checklist (must-not-miss)

1. **`model.User.GroupID` field** — without it `SELECT *` breaks app-wide (highest priority).
2. **`cmd/all-in-one/db/transfer.go`** — add `group_id` to `userRow` + users SELECT/INSERT; add the 4 new tables to `transferData/readAll/writeAll/logTransferSummary` using existing `boolInt`/`boolForDst` helpers. **FK order:** features+groups **before** users; group_features+overrides **after**. Omission silently drops RBAC data on transfer.
3. **Mock regen** after new interfaces.
4. **Admin lockout** — last-admin guard, built-in delete/rename block, admin-immune-to-override.
5. **Direct-auth** — middleware + `/users/me` short-circuit on `SessionID=="direct-auth"` (config `direct_auth_is_admin`).
6. **Deleted group** — `ON DELETE SET NULL` → members revert to regular-user default; `group_features` cascades. No orphans.
7. **New-app onboarding** — (a) append `Feature` to `rbac.Registry`; (b) wrap routes with `mkGated("<key>")`; (c) map sidebar menu→feature. Bootstrap auto-registers + auto-grants non-admin features.
8. **Doc/reality:** follow codebase's hand-written parameterized SQL (CLAUDE.md mentions squirrel, but nothing uses it).

---

## 14. Verification (end-to-end)

```bash
go build ./... && go test ./internal/rbac/... ./internal/authnz/...
mockery              # after interface changes
go run main.go all-in-one server      # migrations 06 apply; bootstrap logs groups/features
```
Then exercise: create a `listing-only` group, move `user` into it → `/api/v1/topics` 200 but
`/api/v1/chats` 403; add a per-user override allowing chat → 200 (override beats group); moving the sole
admin out of admin group → 409. `/api/v1/users/me` returns `is_admin` + `group` + `features[]`. Frontend:
Settings > Access Management shows Users/Groups/Features tabs (admins only); a restricted user's sidebar
hides disallowed apps. Repeat with `storage.type: postgres`.

---

## Suggested implementation order

Schema (`06`) + `model.User.GroupID` → `internal/rbac` model/repo/factory → resolver + bootstrap →
middleware + `server.go` wiring → `/users/me` extension → mgmt API + guards → config → transfer.go + mocks →
tests → frontend (api/store/sidebar/settings/components) → docs (ADR/context/metrics/swagger).
