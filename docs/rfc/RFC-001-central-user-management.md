# RFC-001: all-in-one as Central User Management (SSO) for other apps

- **Status:** **Accepted: Option A**, implemented in [#32](https://github.com/opan/all-in-one/pull/32) (aio) and [opan/cashflow#3](https://github.com/opan/cashflow/pull/3). Decisions made during the build are in [OIDC_PROVIDER_ADR](../adr/OIDC_PROVIDER_ADR.md)
- **Author:** opan
- **Created:** 2026-09-27 · **Updated:** 2026-10-04
- **First consumer:** [cashflow](https://github.com/opan/cashflow); more apps planned
- **Related:** [EXTERNAL_RATE_LIMIT_ADR](../adr/EXTERNAL_RATE_LIMIT_ADR.md) (the "aio as a service" pattern), [USER_AUTHENTICATION_ADR](../adr/USER_AUTHENTICATION_ADR.md)

## Complexity score: **7 / 10** (Option A, scoped to OIDC core, built on a provider library)

| Approach | Score | One-liner |
|---|---:|---|
| **A — OpenID Connect provider, core profile (leaning)** | **7** (8 if hand-rolled) | Standard protocol; any new app integrates with an off-the-shelf OIDC client library. Scoped to what first-party apps need |
| B — Redirect + app-token introspection | 6 | Same redirect shape as A without the standard pieces (ID token, discovery, JWKS). Fallback if A proves too heavy |
| D — Embedded first-party flow (consumer keeps its own pages) | 5 | No redirect, but passwords pass through the consumer and it doesn't fit standard OIDC. Reference only |
| C — Shared cookie + local JWT validation | 3–4 | Cheapest, but only works if every app is under one parent domain; weak logout story |

Earlier drafts scored A at 9 and B at 7. Three findings brought the numbers down:
- **The user migration is small** (§7.3). aio accepts accounts without email, both apps store plain bcrypt hashes so passwords carry over, cashflow's user ids don't change, and there are only a handful of users.
- **A is scoped to OIDC core.** Consent screens, dynamic client registration and third-party clients are dropped (§4).
- **A provider library handles the protocol** (§7.1). Hand-rolling it is what keeps the 8.

What remains is that auth is security-critical: the protocol has to be wired correctly, and a security review gates production.

---

## 1. Summary

Make aio the single source of truth for user identity (registration, login, password, 2FA, account state) and an **OpenID Connect provider** for other first-party apps, starting with cashflow: "Login with all-in-one". Apps stop storing passwords. aio authenticates the user on its own pages and hands the app a signed ID token. Each app maps that identity to a thin local user record.

This does for **identity** what External Rate Limiting did for **rate limiting**: aio becomes the service and consumers integrate with it. Each consumer's integration is feature-flagged, so it falls back to its own local behavior when the flag is off.

## 2. Motivation

- One account per person across the homelab, and one place to change a password or enable 2FA.
- Consumers shed security-critical code (password hashing, session issuance, 2FA) they shouldn't each reimplement.
- **More apps are planned.** With a standard protocol, adding an app is configuration: register a client in aio, then point the app's OIDC client library at aio. No custom integration code.
- aio already has the hard parts: users, sessions, login and signup pages, TOTP 2FA with recovery codes, RBAC and admin user management.

## 3. Current state (grounded in code)

### aio (`internal/authnz`)
- `users(id TEXT PK, username UNIQUE, email UNIQUE NULL, name, password_hash, …)`. **id is a stable string**, carried in aio's JWT as `user_id`. **Email is optional**: migration 09 dropped `NOT NULL` so self-service sign-up needs only a username and password (`POST /users` validates just those two; the `/signup` page labels email "optional").
- Passwords are plain bcrypt at the default cost (`internal/auth/auth.go`).
- No "forgot password" flow and no admin password-reset endpoint. `POST /users/reset_password` is a *change* password: it requires a logged-in aio session plus the current password. Admins can change a user's email and block or unblock them, nothing more.
- Sessions are stored in the database and checked on every request. aio's own session JWT is **HS256 with a shared secret** (`auth.jwt_secret`), in `access_token`/`refresh_token` cookies (`SameSite=Lax`).
- Existing endpoints: `POST /users`, `POST /sessions`, `POST /sessions/refresh`, `GET /sessions/verify`, 2FA verify/recovery, `GET /users/me`, `DELETE /sessions`.

### cashflow
- `users(id uuid PK, username UNIQUE, password_hash)` and `sessions(id token PK, user_id FK, expires_at)`. Passwords are plain bcrypt at the default cost (`auth.go`). Opaque `cashflow_session` cookie with a 30-day TTL; the `withUser` middleware loads the user via `UserBySession`.
- Only two things reference a user: `sessions.user_id` and `cashplans.owner_id`.
- No email column.

### The two gaps
1. **Cross-origin sessions.** aio and cashflow are different origins, so identity must be handed over by redirect (or, only on a shared parent domain, a shared cookie).
2. **Existing identities.** cashflow rows reference cashflow `users.id`, so each cashflow user must be *linked* to an aio account. Usernames also move from a per-app namespace to one global namespace, so the same username can belong to two different people.

## 4. Goals / Non-goals

**Goals**
- aio is the authority for credentials, 2FA and account lifecycle.
- Consumers never see or store passwords.
- Adding a new app is configuration (register a client), not custom code.
- Existing cashflow users keep their data and their current passwords.
- Feature-flagged per consumer: flag off means today's local auth, unchanged.

**Non-goals (v1)**
- Consent screens. All clients are first-party and pre-approved, so users never see a consent prompt.
- Dynamic client registration and third-party clients. Clients are registered by an admin.
- Implicit and hybrid flows. Authorization code + PKCE only.
- Refresh tokens / `offline_access`. Consumers mint their own session after login, so they don't need them.
- Social login or external identity providers.
- Cross-app authorization. aio asserts *identity*; each app keeps its own permissions.

## 5. Design options

### Option A — OpenID Connect provider, core profile (leaning, score 7)
aio becomes a standard OIDC provider: authorization code flow with PKCE, a discovery document, JWKS, signed ID tokens, a userinfo endpoint and RP-initiated logout. Scope is limited to what first-party apps need (see non-goals).

**Pro:** standard. Each new app integrates with a well-tested OIDC client library (`github.com/coreos/go-oidc` + `golang.org/x/oauth2` in Go) instead of custom code. Libraries handle ID-token validation, which is the part most often done wrong by hand. Logout and session revocation have standard answers. **Con:** more surface than B. aio needs asymmetric signing keys, a client registry and integration between the authorize endpoint and its existing login pages. Login, signup and password pages live on aio, so users see an aio-hosted page (see §6 on reducing that friction).

### Option B — Redirect + app-token introspection (fallback, score 6)
The same redirect shape without the standard pieces. cashflow redirects to aio, aio logs the user in and returns a code, and cashflow exchanges the code using its app token for `{user_id, username, email}`. **Pro:** a bit less to build; reuses app tokens. **Con:** custom. Every future app needs custom integration code, and there are no standard libraries on either side. The data model and migration are the same as A, so moving from B to A later touches only aio and each consumer's login code.

### Option C — Shared cookie + local JWT validation (score 3–4)
If every app sits under one parent domain, aio sets its auth cookie on that domain and each app validates the JWT locally. **Pro:** almost no protocol. **Con:** requires one cookie domain, spreads signing trust to every app, and logout and revocation are awkward. Doesn't generalize as more apps are added.

### Option D — Embedded first-party flow (reference only, score 5)
cashflow keeps its own login, signup and password pages, and its backend calls aio APIs (`/auth/login`, `/auth/login/2fa`, `/auth/register`, `/auth/users/{id}/password`, `/auth/password/reset`) with its app token. **Pro:** no redirect. **Con:** passwords pass through each consumer; there is no single sign-on, since each app asks for the password; each app renders 2FA itself; and the app token becomes powerful enough to need per-capability permissions, the current password on every change, rate limits per username and IP, and strict no-logging of request bodies. It also doesn't fit standard OIDC: the password grant it effectively relies on was removed in OAuth 2.1. Kept for reference in case hosted pages are rejected later.

## 6. Recommended approach

**Adopt Option A, scoped to OIDC core, built on an established provider library** rather than hand-rolled. With more apps planned, the standard protocol pays for itself from the second app onward. Each consumer gets a **feature flag** (`AUTH_PROVIDER=aio|local`), like `AIO_RATELIMIT_ENABLED`, so it can fall back to its own auth during rollout.

**Hosted pages and friction.** Under A, login, signup and password pages live on aio. To keep the hop from feeling like leaving the app:
- **Brand aio's pages per client.** The client registry stores each app's display name and logo, and the login page shows "Masuk ke cashflow" with cashflow's look.
- **Serve aio under the same parent domain** as the apps (e.g. `auth.opan.dev` next to `cashflow.opan.dev`), so the address bar barely changes.
- **Single sign-on as payoff.** Once a user is logged in to aio, opening another app logs them in with no prompt at all.

### 6.1 Local user records

The consumer keeps a `users` table in its own database, because its domain rows (`cashplans.owner_id`) need a local owner. The table is **no longer the source of truth**:

| Kept in cashflow | Moves to aio |
|---|---|
| A row per person, linked by `aio_user_id` (the ID token's `sub`) | Password hash |
| The existing primary key, so foreign keys are untouched | 2FA, password changes, blocking |
| Optional cached profile (username, email), refreshed on each login | Registration |

### 6.2 Login and registration flow

Login and registration are the same flow. The "Daftar" button adds `prompt=create`, the OIDC extension for "show signup instead of login". If the provider library doesn't handle it, aio's authorize handler reads it and sends the user to the signup page.

```
User            cashflow                                aio
 │ click "Daftar"  │                                     │
 │────────────────>│ store state, nonce, PKCE verifier   │
 │                 │ in a short-lived cookie             │
 │                 │ 302 → /api/v1/oauth2/authorize      │
 │                 │   ?client_id=cashflow&response_type=code
 │                 │   &scope=openid profile&redirect_uri=…/auth/callback
 │                 │   &state=…&nonce=…&code_challenge=…&prompt=create
 │<────────────────│                                     │
 │───────────────────────────────────────────────────────>│ branded signup page (username, password,
 │  submit form    │                                     │ email optional); create account; start aio session
 │<───────────────────────────────────────────────────────│ 302 → cashflow/auth/callback?code=…&state=…
 │────────────────>│ check state                          │
 │                 │ POST /api/v1/oauth2/token            │
 │                 │ (client_id + secret + PKCE verifier)>│
 │                 │<─── ID token {sub, preferred_username}│
 │                 │ verify ID token: signature via JWKS, │
 │                 │ iss, aud, exp, nonce (go-oidc)       │
 │                 │ no local row for this sub → create it (JIT)
 │                 │ create cashflow_session              │
 │<────────────────│ 302 → dashboard                      │
```

cashflow ends up with no register or login handler of its own, only `/auth/login`, `/auth/callback` and `/auth/logout`. aio's signup asks only for a username and password, so moving signup to aio adds no fields.

| Case | Behavior |
|---|---|
| Username already taken | aio shows the error on its signup page; usernames are one namespace across all apps |
| User already has an aio account | aio's signup page offers "log in instead"; the flow continues and cashflow creates its local row on return |
| User already logged in to aio (e.g. from another app) | aio skips the login page and redirects straight back: single sign-on |
| Signup abandoned halfway | Nothing returns to cashflow, so no local row is created |
| aio account created but cashflow callback fails | The next login completes it, since the local row is created on whichever login happens first |
| Flag off (`AUTH_PROVIDER=local`) | cashflow's current `/register` and `/login` work unchanged |

### 6.3 Password change and reset

**Change password** (user knows the current one) happens on aio's account page, using the existing endpoint. cashflow just links to it ("Ubah password").

**Forgot password.** Users may never set an email, so the only recovery path is the admin. The admin verifies the user's identity, then issues a **one-time reset code** that the user redeems on aio's reset page to set their own new password. The admin never learns the password.

```
User               aio reset page          Admin (operator)            aio
 │ "Lupa password?"   │                        │                         │
 │──────────────────> │ "Contact the admin on  │                         │
 │                    │  WhatsApp, have your   │                         │
 │                    │  username ready"       │                         │
 │── WhatsApp: "forgot my password, username budi" ──>│                  │
 │<── verify: "name one of your cashplans" ───────────│                  │
 │── answer ─────────────────────────────────────────>│                  │
 │                    │                        │ Admin › Users › budi    │
 │                    │                        │ "Generate reset code" ─>│ hashed, 30 min, single use,
 │                    │                        │<── ABCD-EFGH (shown once)│ earlier codes voided
 │<── WhatsApp: "ABCD-EFGH, valid 30 minutes" ────────│                  │
 │ username + code + new password                     │                  │
 │──────────────────> │ POST /api/v1/auth/password/reset ───────────────>│ check code (expiry, attempts);
 │                    │                        │                         │ set password, burn code,
 │                    │                        │                         │ revoke sessions, audit log
 │<── "Password changed, please log in" ──────────────────────────────────│
```

**Identity verification is the real control.** The code is secure; the conversation before it is the weak point, because anyone can claim to be "budi". Before issuing a code, the admin checks either:
- **A known channel:** the request comes from the WhatsApp number already associated with that user.
- **Something only the owner knows:** cashflow data works well. Ask for the name of one of their cashplans, or roughly when they last recorded an entry.

**Code design**

| Property | Choice |
|---|---|
| Format | 8 characters, `XXXX-XXXX`, from an alphabet without look-alikes (no 0/O, 1/I) |
| Lifetime | 30 minutes, single use; generating a new code voids earlier ones |
| Attempts | 5 wrong tries void the code; the admin issues a new one |
| Storage | Hashed, never plain text (same approach as aio's 2FA recovery codes, `HashRecoveryCode`) |
| Shown to admin | Once, with a copy button, like the app-token creation dialog |
| Rate limiting | Its own rate-limit target on the reset endpoint, per IP |

A code is preferred over a temporary password. With a temporary password the admin briefly knows a working password, and aio would need a "must change password at next login" mechanism. With a code, the user picks their own password directly.

**Edge cases**
- **User has 2FA and still has their authenticator:** reset only the password; login still asks for the 2FA code.
- **User lost the authenticator too:** the admin must also disable 2FA. That's a bigger action than a password reset, so it needs stricter identity checks and its own audit entry.
- **Blocked account:** a reset doesn't unblock it; blocking stays a separate admin decision.
- **Reset because "someone may have my password":** aio revokes its own sessions, but consumer sessions are separate. OIDC Back-Channel Logout covers this: aio sends each registered app a signed logout notice and the app ends that user's session. Planned as a fast follow-up after v1 (§10).

Later, users who add an optional email can get a reset link instead, using the same reset endpoint with an emailed token in place of the admin code.

## 7. Changes required

### 7.1 aio (provider)
- **Provider library.** Evaluate an established Go OIDC provider library (candidates: `github.com/zitadel/oidc` and its `op` package, `github.com/ory/fosite`) and implement its storage interfaces over sqlx for both SQLite and Postgres. Hand-rolling the protocol is the "8 instead of 7" path and is not recommended.
- **Migration: client registry** (`oauth_clients`): client_id, display name, logo, client-secret hash (SHA-256, shown once, like app tokens), redirect URIs, post-logout redirect URIs, allowed scopes, created/revoked timestamps. Admin UI to register and revoke clients.
- **Signing keys.** ID tokens need asymmetric signatures (RS256 or ES256) so apps can verify them via JWKS without a shared secret. Keys are stored encrypted, like the TOTP key, and rotated with overlap: JWKS publishes the current and previous key. aio's internal session JWT can stay HS256.
- **Endpoints.** The discovery document lives at `/.well-known/openid-configuration`, a path fixed by the spec and the only exception to the `/api/v1` base path. The rest go under `/api/v1/oauth2/`: `authorize`, `token`, `userinfo`, `jwks`, `logout`.
- **Login page integration.** When the authorize endpoint finds no aio session, it sends the user to aio's existing login or signup page (branded per client). After login and 2FA, the user returns to the authorize endpoint to complete the flow. This is most of the aio-specific work outside the library.
- **Claims.** `sub` = aio `users.id`, `preferred_username`, plus `email` only when set (with `email_verified: false`, since aio doesn't verify emails).
- **Admin-issued reset codes** (§6.3): `password_reset_codes` table (user, code hash, expiry, attempts, used-at, issued-by), admin endpoint and dialog, public reset endpoint and page, audit log.
- **User import CLI** (§7.3).
- **Cross-cutting:** OpenTelemetry spans and metrics on every new endpoint, `docs/metrics.md` updated, swagger annotations, tests, and an ADR at closeout.
- Estimated ~1200–1800 lines including tests, admin UI and dual-backend storage.

### 7.2 cashflow (consumer)
- **Libraries:** `github.com/coreos/go-oidc/v3` and `golang.org/x/oauth2`. They handle discovery, the code exchange and ID-token validation.
- **Routes:** `/auth/login` (redirect), `/auth/callback` (exchange and verify, find-or-create the local user by `sub`, create `cashflow_session`), `/auth/logout` (clear the local session, then RP-initiated logout at aio). `withUser` is unchanged.
- **Migration:** add `aio_user_id TEXT UNIQUE` to `users`; make `password_hash` nullable (dropped after the rollback window).
- **Feature flag** `AUTH_PROVIDER=aio|local`, plus issuer URL, client id and client secret. Flag off means today's behavior exactly.
- Estimated ~250–400 lines plus the migration.

### 7.3 Moving existing cashflow users

Only user **accounts** move to aio. cashflow's data (plans, entries, dues, receipts) stays in cashflow's database, and cashflow's `users` rows keep their ids, so nothing that references them changes.

**Why it's cheap:**
- **Passwords carry over.** Both apps store plain bcrypt hashes at the same cost, so cashflow's hashes are copied into aio as-is. Nobody resets or re-registers.
- **No forced logouts.** Existing `cashflow_session`s point at rows that don't change, so they stay valid. Users meet aio's login page only when their 30-day session expires, and log in with the same password.
- **No email needed**, since aio accepts accounts without one.
- **Few users.** The local compose database has 13 users and 15 plans; production numbers may differ.

**Steps**
1. **Clean up test data.** The integration run left 2 test users (`integ_…`, `off_…`) and their plans in the local compose database. Remove them before any migration.
2. **Check username collisions against production aio.** If a cashflow username already exists in aio for a different person, matching by username would merge two people. Resolve each by hand (rename one side). Compare username rules first (cashflow allows `^[a-z0-9_]{3,30}$`).
3. **Import into aio.** A one-off command, `all-in-one users:import`, reads an export of `{id, username, password_hash, created_at}`, creates aio accounts with the hashes copied as-is, reports collisions, and writes a mapping of cashflow id → aio id. **CLI only, never an API endpoint:** accepting pre-hashed passwords over the network would be dangerous.
4. **Link in cashflow.** Run the migration adding `aio_user_id`, then apply the mapping. Verify every user row has an `aio_user_id`.
5. **After the rollback window,** drop `password_hash` from cashflow.

**Effort:** ~150–250 lines in aio (import command with tests), ~50–100 lines in cashflow, plus a dry run on a database copy. On its own this is about a 2/10.

**Alternative: claim on first login.** Skip the bulk import. On a user's first aio login, cashflow asks once for their old cashflow password and links the accounts. This avoids the import but adds a step for every user and keeps cashflow's password code alive. With a handful of users, the bulk import is simpler.

## 8. Security considerations

- **Redirect URIs** must match the client's registered list exactly; this blocks the classic open-redirect and code-interception attacks.
- **PKCE (S256)** for every client, including confidential ones.
- **`state`** against CSRF on the callback; **`nonce`** against ID-token replay.
- **Authorization codes** are single use with a short TTL (handled by the library).
- **ID-token validation on the consumer** checks signature (JWKS), `iss`, `aud`, `exp` and `nonce`. Use go-oidc; never parse ID tokens by hand.
- **Signing keys** are stored encrypted, rotated with overlap, and never leave aio.
- **Client secrets** are hashed (SHA-256), shown once, and revocable.
- **Auth fails closed.** Unlike rate limiting, which fails open, an authentication decision must fail closed: if aio is down, nobody new can log in. Existing consumer sessions keep working until they expire, so an outage doesn't log everyone out.
- **Reset codes:** random, hashed at rest, single use, short TTL, attempt-limited; every use revokes the user's sessions and is written to the audit log. Verifying the requester's identity before issuing one is the admin's responsibility (§6.3).
- **User import** accepts password hashes only through the CLI, never over the network.
- A full security review is a required gate before production. Running the OpenID Foundation's conformance suite against aio is a cheap extra check.

## 9. Rollout plan

1. Ship the aio provider behind an off-by-default `auth.oidc.enabled` switch; verify with a throwaway client.
2. Register cashflow as a client; add the `AUTH_PROVIDER=aio` path while `local` stays the default.
3. Clean up test data, check collisions against production, and dry-run the import on a database copy.
4. Import users, link them, and switch cashflow to `aio` in a quiet window. Keep `local` available for one release.
5. Add back-channel logout; drop `password_hash` from cashflow after the rollback window; write the ADR.
6. Each later app: register a client, add an OIDC client library and a local user table.

## 10. Open questions

Answered by the build:
- **Provider library:** `github.com/zitadel/oidc/v3` (actively maintained; `ory/fosite` had not released in almost two years). Apps use `go-oidc` + `x/oauth2`.
- **Cross-app authorization:** identity only in v1; each app keeps its own permissions.
- **Back-channel logout:** a follow-up, not v1. v1 ends aio's session on RP-initiated logout when the app sends a valid `id_token_hint`.

Still open (follow-ups):
- **Domain layout:** will aio and the apps share a parent domain? This decides how visible the redirect is.
- **Username collisions:** to be checked against production aio before importing existing cashflow users.
- **Who can issue reset codes:** only the operator, or any aio admin (RBAC)?

## 11. Decision

**Accepted: Option A**, scoped to OIDC core and built on a provider library, because it is standard and more apps are planned. Implemented and verified end to end with cashflow in [#32](https://github.com/opan/all-in-one/pull/32) and [opan/cashflow#3](https://github.com/opan/cashflow/pull/3).

The build kept to this RFC, with three additions recorded in [OIDC_PROVIDER_ADR](../adr/OIDC_PROVIDER_ADR.md):
- the shared demo account (`demo_mode`) is refused for app logins, since every visitor would share one account there;
- aio's own session ends on app logout only with a valid `id_token_hint` (logout CSRF guard);
- a consumer with a strict CSP must allow aio's origin in `form-action`, because logout is a form POST that redirects to aio.

Deferred to follow-ups: admin-issued password reset codes (§6.3), importing existing cashflow users (§7.3), back-channel logout, and an admin UI page for clients.
