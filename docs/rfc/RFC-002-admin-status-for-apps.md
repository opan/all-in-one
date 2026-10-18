# RFC-002: Telling apps whether a user is an aio admin

- **Status:** Draft
- **Author:** opan
- **Created:** 2026-10-15 · **Updated:** 2026-10-18
- **Consumers:** cashflow, tally (both log in through aio, RFC-001)
- **Related:** [RFC-001](RFC-001-central-user-management.md), [OIDC_PROVIDER_ADR](../adr/OIDC_PROVIDER_ADR.md) (ADR-O10), [ACCESS_MANAGEMENT_ADR](../adr/ACCESS_MANAGEMENT_ADR.md), [EXTERNAL_RATE_LIMIT_ADR](../adr/EXTERNAL_RATE_LIMIT_ADR.md) (the "aio as a service" pattern)

## Complexity score: **3 / 10**

aio already knows who is an admin (`rbac.Resolver.IsAdmin`). This adds one server-to-server endpoint that
answers for a given user, and apps call it on every admin request.

## 1. Summary

Add `GET /api/v1/oidc/users/{sub}/status`, authenticated with the app's existing OIDC client credentials,
returning `{"admin": bool, "blocked": bool}`. Apps call it **on every admin request**, over the cluster-local
address (`http://all-in-one.app.svc.cluster.local`), with no cache. A user removed from aio's admin group
loses app admin access on their next request, not after a delay.

## 2. Motivation

cashflow and tally have no admin page. The operator wants a read-only overview of all users' data in each
app, without keeping a second admin list per app, and with **no window** in which a demoted or blocked user
can still act as an admin.

## 3. Current state

- ID token claims: `sub`, `preferred_username`, `name`, `email` only (`fillUserinfo`,
  `internal/oidc/service/storage.go`). cashflow and tally request `openid profile`.
- aio admin = member of the built-in `admin` group (`Resolver.IsAdmin`); aio's own admin routes use `RequireAdmin`.
- No refresh tokens; access tokens live 15 minutes. App sessions live 30 days.
- aio, cashflow and tally run in the same Kubernetes namespace (`app`). aio's Service is `all-in-one` on
  port 80. Today both apps reach aio only through its public URL (`https://aio.opanmustopah.com`).

## 4. Goals / Non-goals

**Goals**
- Apps can tell whether the current user is an aio admin, checked on every admin request.
- Demotion or blocking in aio takes effect in apps on the next request.
- No new credentials: apps authenticate with the OIDC client id and secret they already have.

**Non-goals**
- Per-app or delegated roles (e.g. a read-only support group).
- Write access for admins in apps.
- Ending app sessions when aio logs out or blocks a user (back-channel logout, RFC-001 follow-up).

## 5. Options

| Option | Verdict |
|---|---|
| **A. Server-to-server status check on every admin request, cluster-local, no cache** | **Proposed.** No staleness window; one cheap in-cluster call per admin page |
| B. `aio_admin` claim in the ID token + re-login round trip every 15 min (first draft) | Rejected. A demoted admin keeps access for up to 15 minutes; every re-check is ~4 browser redirects |
| C. `aio_admin` claim at login only | Rejected. App sessions last 30 days, so a demoted admin keeps access for up to a month |
| D. Per-app RBAC feature (`cashflow-admin`) | Later. Bootstrap grants new non-admin-only features to `regular-user`, which would make every user an app admin; an admin-only feature is the same as `IsAdmin` |
| E. Call `userinfo` when needed | Rejected. Needs an access token, which expires in 15 minutes, and there are no refresh tokens |

No `aio_admin` claim is added. Apps that want to show an "Admin" link call the same endpoint once at login
and keep the result on the session as a display hint only (§6.2).

## 6. Design

### 6.1 aio: `GET /api/v1/oidc/users/{sub}/status`

- **Auth:** HTTP Basic with the OIDC `client_id` and `client_secret` (`Service.AuthenticateClient`, constant
  time). Unknown, wrong or revoked client → 401. No user session involved.
- **Response:** `200 {"success":true,"data":{"admin":bool,"blocked":bool}}` in the standard envelope.
  Unknown `sub` (not a UUID, or no such user) → 404. Resolver or database error → 500; apps treat any
  non-200 as "not admin".
- **Admin** comes from `Resolver.IsAdmin`. **Blocked** users are reported as `admin: false` as well, so a
  blocked admin can't use app admin pages even though their group is unchanged.
- **Routing:** registered with the other aio-specific OIDC routes under `/api/v1/oidc/`, outside JWT auth
  and RBAC middleware, only when `auth.oidc.enabled`.
- **Observability:** OTel span plus metric `aio.oidc.user_status` (attributes `client_id`, `result`:
  `admin`, `not_admin`, `unknown_user`, `unauthorized`, `error`). Update `docs/metrics.md` and swagger.
- **Scope of answers:** any registered client can ask about any user. All clients are first-party and
  registered by an admin, so this is acceptable; restrict to users who have logged in to that client if
  third-party clients ever appear.

### 6.2 Apps (cashflow, tally)

- New optional setting `AIO_INTERNAL_URL` (e.g. `http://all-in-one.app.svc.cluster.local`). When unset the
  app uses `AIO_ISSUER`. Only this call uses it; discovery, the code exchange and ID token checks keep using
  the public issuer, since the token's `iss` is the public URL.
- `requireAdmin(w, r)`: the user must be logged in through aio (has an `aio_user_id`); then call the status
  endpoint with the user's `aio_user_id`. **No cache**; a short timeout (2 s).
  - `admin: true` → serve the page.
  - `admin: false` or 404 → 404 (don't advertise the page).
  - error, timeout or 5xx → 503 "Admin pages are unavailable while all-in-one is unreachable" (**fails closed**).
- Local-only accounts (`AUTH_PROVIDER=local`, or no `aio_user_id`) are never admin. Admin is keyed by the aio
  `sub`, never by username.
- **Nav hint:** the callback calls the endpoint once and stores `is_admin_hint` on the session to decide
  whether to show the "Admin" link. It is never used for authorization; a stale hint only shows or hides a link.
- Admin pages are separate GET-only handlers: read-only is enforced on the server, not by hiding buttons.
- Each admin view is logged (admin, page, which user's data) since it exposes other people's finances.
- **cashflow overview shows activity, not money, by default:** per user, the number of cashplans and entries,
  last activity and sign-up date. Rupiah figures (entry `amount`, balances, `due_amount`) appear only when the
  admin opens one user's page, which is also the audit-logged event. Revisit once the pages are in use.
  (tally stores no money; its overview has no such split.)

## 7. Request load

One extra call per admin page view, inside the cluster: no ingress, TLS or Cloudflare hop. aio answers from
two indexed lookups (user row, group membership). With one admin browsing read-only pages this is negligible,
so there is no cache, and therefore no staleness window.

## 8. Security considerations

- **Secret on the cluster network:** the call uses the client secret the apps already hold; no separate
  token. Which address it goes to is a deployment setting (`AIO_INTERNAL_URL`): the cluster address is plain
  HTTP between pods in a single-tenant cluster, and leaving it unset uses the public https URL. Switching is a
  config change, no code change.
- **Fails closed:** if aio is down or slow, admin pages are refused; normal app pages are unaffected.
- **Login flaws now matter more:** an app admin session depends on the login being sound; the browser binding
  and `prompt=login` refusal (ADR-O4) are prerequisites.
- The shared demo account can't log in to apps (ADR-O6); `direct-auth` can't complete an app login (its user
  id is not a UUID), so neither can reach an app admin page.
- **Accepted limitation:** the endpoint checks the account, not the browser. Logging out of aio does not lock
  an app's admin pages while the app session lasts; demotion and blocking do, immediately.

## 9. Rollout

1. aio: endpoint, metric, swagger, tests (behind `auth.oidc.enabled`).
2. cashflow, then tally: `AIO_INTERNAL_URL`, `requireAdmin`, nav hint, read-only overview pages, audit log.
   The homelab deployments set `AIO_INTERNAL_URL=http://all-in-one.app.svc.cluster.local`.
3. Apps that don't call the endpoint see no change.

## 10. Open questions

None left.

Settled in review:
- **Credential:** the existing OIDC client secret, no separate status token. The address is configurable
  (cluster or public), so plain HTTP is a deployment choice that can change later.
- **Rate-limit check:** `AIO_RATELIMIT_URL` is already configurable; moving it to the cluster address is a
  config change, outside this RFC.
- **Money in the cashflow overview:** hidden by default, shown per user on drill-down (§6.2); to be revisited.

## 11. Decision

Pending review.
