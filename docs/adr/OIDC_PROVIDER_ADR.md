# ADR: all-in-one as an OpenID Connect provider

Records the decisions made while implementing RFC-001 Option A (`docs/rfc/RFC-001-central-user-management.md`):
aio logs users in to other first-party apps over OpenID Connect, starting with cashflow. Build progress was
tracked on the Nextcloud Deck boards (cards prefixed "OIDC"); `.context/OIDC_PROVIDER_PROGRESS.md` has the
pointers.

---

## ADR-O1: Build on zitadel/oidc, scoped to the OIDC core profile

### Decision
aio uses `github.com/zitadel/oidc/v3` (`pkg/op`) and implements its storage interface; apps use
`github.com/coreos/go-oidc/v3` with `golang.org/x/oauth2`. Only the authorization code flow with PKCE (S256)
and `client_secret_basic` is supported. No refresh tokens, consent screens, dynamic client registration or
`prompt=none`.

### Rationale
The protocol is security-critical and the library is actively maintained (releases days apart at the time of
writing, unlike `ory/fosite`). Apps mint their own session after login, so refresh tokens buy nothing in v1.

### Consequence
The library hard-codes `implicit` and `jwt-bearer` into the discovery document. Neither works: clients only
allow the `code` response type, and storage refuses the jwt-bearer key lookup. A test pins this down.

---

## ADR-O2: Endpoint layout

Discovery is at `/.well-known/openid-configuration` (fixed by the spec, the only path outside `/api/v1`).
Provider endpoints live under `/api/v1/oauth2/` (`authorize`, `token`, `userinfo`, `keys`, `end_session`,
`revoke`) and are registered before the `/api/v1` subrouter so the provider owns that prefix. aio-specific
endpoints live under `/api/v1/oidc/`: the login hand-off (`auth-requests/{id}`, `.../complete`) and the admin
client registry (`clients`). Everything is behind `auth.oidc.enabled` (off by default) and needs an absolute
`auth.oidc.issuer` without a path.

---

## ADR-O3: What is persisted

- **Clients** (`oidc_clients`, migration 11): id, name, SHA-256 hash of a 256-bit secret (shown once, compared
  in constant time), redirect and post-logout URIs. Redirect URIs must be https, except loopback http for local
  development. Revocation is soft.
- **Signing keys** (`oidc_signing_keys`): RSA-2048, generated on first start, private key encrypted with
  `auth.totp_encryption_key` via the existing AES-GCM helper. The newest active key signs; all active keys are
  published so tokens signed before a rotation still verify. Changing that encryption key makes aio refuse to
  start with a clear error.
- **Auth requests, codes, access tokens**: in memory with a TTL (`auth_request_ttl`, `access_token_lifetime`).
  A restart mid-login only means the user starts again; apps use the ID token once, at login.

---

## ADR-O4: Login hand-off through aio's own pages

`Client.LoginURL` points at the SPA page `/oauth/login`. It reads the request (`GET /api/v1/oidc/auth-requests/{id}`:
which app, signup or login). If the browser already has an aio session it completes immediately (single
sign-on); otherwise it sends the user to `/login` or `/signup` (`prompt=create`) with `?next=` back to itself.
Login and signup accept only same-site `next` paths (no `//host`, no scheme) so `next` can't be an open
redirect, and say "continue to <app>". Completion (`POST .../complete`) requires an aio session, refuses
blocked users and revoked clients, then returns the provider's callback URL.

The SPA uses plain `fetch` for this flow because `apiClient` redirects to `/login` on a 401, which would drop
`next`.

---

## ADR-O5: Logout ends aio's session only with a valid id_token_hint

RP-initiated logout (`end_session`) is wrapped so that, besides the library's validation and redirect to the
client's registered post-logout URI, it deletes aio's session row and clears aio's cookies. This happens only
when the request carries an ID token aio issued; expired tokens are accepted, the signature is still verified.
Without it, any page could log users out of aio by linking to the endpoint (logout CSRF).

The library sets the issuer on the request context only inside its own handler, so the wrapper sets it
explicitly before verifying the hint.

---

## ADR-O6: The shared demo account can't log in to other apps

When `demo_mode` is enabled, completing an app login as the demo user is refused (403, metric reason `demo`).
The login page hides the demo shortcut when an app opened it, and the hand-off page offers "Use another
account". The demo account exists to explore aio; in another app every visitor would share one account and
its data.

---

## ADR-O7: Consumer side (cashflow)

- Feature-flagged with `AUTH_PROVIDER=aio|local`; `local` (default) leaves cashflow's login untouched. With
  `aio`, a half-done configuration is a startup error, and local password login and registration are refused.
- **Fails closed:** if aio is unreachable nobody new can log in. Discovery is lazy and retried per login, so
  cashflow still starts and already-logged-in users keep working on cashflow's own session.
- The callback verifies state, exchanges the code with the client secret and PKCE verifier, and verifies the
  ID token (signature, issuer, audience, expiry, nonce). The local user is found by `aio_user_id` (the token's
  `sub`) or created on first login; a username held by a local-only account is never taken over.
- The ID token is stored with the session and sent as `id_token_hint` on logout.
- Cashflow's CSP must list aio's origin in `form-action`: logout is a form POST that redirects to aio, and
  browsers apply `form-action` to that redirect.

---

## Verification

Unit and in-process tests cover the full code flow (ID token verified against the published keys),
single-use codes, wrong secrets, unregistered redirect URIs, blocked users, revoked clients, the demo account,
logout with and without a hint, and memory-store expiry. A real browser run against both apps on separate hosts
(aio on `127.0.0.1`, cashflow on `localhost`) passed 18/18 checks: signup through cashflow, logout ending both
sessions, login with an existing account, single sign-on with no form, and demo refusal. It also found three
bugs, all fixed: the CSP issue above, the demo account issue above, and a signup link that dropped `next`.

## Not in this build

Admin-issued password reset codes, `users:import` for existing cashflow accounts, back-channel logout and an
admin UI page for clients (RFC-001 §6.3, §7.3; Deck "Later" cards).
