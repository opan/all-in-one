# RFC-002: Telling apps whether a user is an aio admin

- **Status:** Draft
- **Author:** opan
- **Created:** 2026-10-15
- **Consumers:** cashflow, tally (both log in through aio, RFC-001)
- **Related:** [RFC-001](RFC-001-central-user-management.md), [OIDC_PROVIDER_ADR](../adr/OIDC_PROVIDER_ADR.md) (ADR-O10), [ACCESS_MANAGEMENT_ADR](../adr/ACCESS_MANAGEMENT_ADR.md)

## Complexity score: **3 / 10**

aio already knows who is an admin; this only hands that fact to apps at login and keeps it fresh.

## 1. Summary

Add an `aio_admin` scope. When an app requests it, aio puts `"aio_admin": true|false` in the ID token, from
`rbac.Resolver.IsAdmin`. Apps use it to gate a **read-only** admin overview of all their data. Because app
sessions last 30 days, apps re-check the flag through a fresh single sign-on round trip before showing an
admin page if the last check is older than 15 minutes.

## 2. Motivation

cashflow and tally have no admin page. The operator wants a read-only overview of all users' data in each
app, without a second admin list per app.

## 3. Current state

- ID token claims: `sub`, `preferred_username`, `name`, `email` only (`fillUserinfo`,
  `internal/oidc/service/storage.go`). cashflow and tally request `openid profile`.
- aio admin = member of the built-in `admin` group (`Resolver.IsAdmin`); aio's own admin routes use `RequireAdmin`.
- The library drops any scope beyond the standard ones unless `Client.IsScopeAllowed` allows it
  (`op.ValidateAuthReqScopes`); today `IsScopeAllowed` returns false.
- No refresh tokens; access tokens live 15 minutes. App sessions live 30 days.

## 4. Goals / Non-goals

**Goals:** apps can tell, at login, whether the user is an aio admin; demotion or blocking in aio takes effect
in apps within minutes, not 30 days; no new secrets or machine APIs.

**Non-goals:** per-app or delegated roles (e.g. a read-only support group); write access for admins in apps;
back-channel push of role changes.

## 5. Options

| Option | Verdict |
|---|---|
| **A. `aio_admin` claim + fresh check via SSO round trip** | **Proposed.** Small, standard, freshness bounded to 15 min |
| B. Per-app RBAC feature (`cashflow-admin`) in the claim | Later. Bootstrap auto-grants new non-admin-only features to `regular-user`, which would make everyone an app admin; marking it admin-only makes it equal to `IsAdmin` |
| C. App calls `userinfo` when needed | Rejected. Access token expires in 15 min and there are no refresh tokens |
| D. New machine API (`client credentials`) per admin request | Rejected for now. Always current, but a new authenticated endpoint plus a network call on every admin request |

## 6. Design (Option A)

### 6.1 aio
- Constant scope `aio_admin`, listed in discovery `scopes_supported`.
- `opClient.IsScopeAllowed("aio_admin")` returns true (all clients are first-party and admin-registered).
  A per-client allow-list can come with the RFC-001 "allowed scopes" column if third parties ever appear.
- The OIDC service gets the RBAC resolver; `fillUserinfo` adds `aio_admin` via `info.AppendClaims` when the
  scope is present. A resolver error fails the token request (never defaults to `true`).
- Metric: count of ID tokens issued with `aio_admin=true`, by client (update `docs/metrics.md`).

### 6.2 Apps (cashflow, tally)
- Request scopes `openid profile aio_admin`.
- Store `is_admin` and `admin_checked_at` on the **session** row, set from the ID token at callback.
- `requireAdmin`: if the session is not admin → 404 (don't advertise the page). If `admin_checked_at` is
  older than 15 min → redirect to `/auth/login?next=<page>`. With an aio session, SSO completes with no form;
  the callback starts a new session with a fresh flag. Demoted → flag false; blocked → aio refuses login;
  logged out of aio → login form.
- Admin is keyed by the aio `sub` only, never by username; local-only accounts (`AUTH_PROVIDER=local`) are
  never admin.
- Admin pages are separate GET-only handlers (read-only enforced server-side, not by hiding buttons).
- Each admin view is logged (admin, page, which user's data) since it exposes other people's finances.

## 7. Security considerations

- The claim raises the stakes of any login flaw from account takeover to admin takeover; it depends on the
  browser binding and `prompt=login` refusal already in place (ADR-O4).
- The shared demo account can't log in to apps (ADR-O6); `direct-auth` can't complete an app login (its
  user id is not a UUID).
- Stored ID tokens (logout hint) now carry the flag; they are never used for authorization after login.

## 8. Rollout

1. aio: scope + claim + tests, behind the existing `auth.oidc.enabled`.
2. cashflow, then tally: request the scope, session columns (idempotent `ALTER ... IF NOT EXISTS`),
   `requireAdmin`, read-only overview pages.
3. An app that doesn't request the scope sees no change.

## 9. Open questions

- Freshness window: 15 minutes, or configurable per app?
- Should the overview hide amounts by default (counts only) and reveal per user on click?

## 10. Decision

Pending review.
