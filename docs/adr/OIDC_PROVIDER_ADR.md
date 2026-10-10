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
  `auth.totp_encryption_key` via the existing AES-GCM helper. The newest active key signs; every active key in
  the database is published (re-read on each JWKS request), so tokens signed before a rotation, or by another
  replica's key, still verify. Changing that encryption key makes aio refuse to start with a clear error.
- **Auth requests, codes, access tokens**: in memory with a TTL (`auth_request_ttl`, `access_token_lifetime`).
  A restart mid-login only means the user starts again; apps use the ID token once, at login. A code is used up
  the moment it is looked up, so simultaneous token requests can't both redeem it.
- **Single replica only while the provider is on.** Because auth requests and codes live in one process, a
  login started on one pod and finished on another fails with "this login link has expired". Run one replica
  with the `Recreate` strategy (a `RollingUpdate` briefly runs two). Moving auth requests and codes into the
  database lifts this limit and is the follow-up if aio needs more than one replica.

---

## ADR-O4: Login hand-off through aio's own pages

`Client.LoginURL` points at the SPA page `/oauth/login`. It reads the request (`GET /api/v1/oidc/auth-requests/{id}`:
which app, signup or login). If the browser already has an aio session it completes immediately (single
sign-on); otherwise it sends the user to `/login` or `/signup` (`prompt=create`) with `?next=` back to itself.
Login and signup accept only same-site `next` paths (no `//host`, no scheme) so `next` can't be an open
redirect, and say "continue to <app>". Completion (`POST .../complete`) requires an aio session, refuses
blocked users and revoked clients, then returns the provider's callback URL.

**Each login is tied to the browser that started it.** The page completes with no click, so without this an
attacker could start a login in their own browser, send the `/oauth/login` link to a victim logged in to aio,
and then follow the callback themselves to get an app session as the victim. Two checks stop it:

- the authorize endpoint sets an `aio_oidc_browser` cookie (random, HttpOnly, path `/api/v1/`, reused across
  tabs) and the auth request stores its hash; completion requires the same cookie (403, reason `other_browser`);
- the callback that issues the code (`authorize/callback`) requires the same cookie *and* aio's session cookie
  for the user who completed the request; otherwise the app gets `access_denied`.

A completed request can't be re-completed as a different user. `prompt=login` and `max_age` are refused
(`invalid_request`) because the page would answer them with single sign-on, and PKCE with S256 is required for
every client (the library itself only requires it for public clients).

The SPA uses plain `fetch` for this flow because `apiClient` redirects to `/login` on a 401, which would drop
`next`.

---

## ADR-O5: Logout ends aio's session only for the hint's own user

RP-initiated logout (`end_session`) also deletes aio's session row and clears aio's cookies, but only when:

- the library has accepted the request (valid `id_token_hint`, active client, registered post-logout URI); this
  runs in `TerminateSessionFromRequest`, which the library calls only after validating, with the HTTP request
  and response passed through the context;
- the hint's `sub` equals the `user_id` of aio's session cookie. Expired hints are accepted, so a hint alone
  proves nothing: every user can get a valid ID token of their own from an app's logout URL. Without the user
  check, a link carrying the attacker's token would log any victim out of aio (logout CSRF).

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

## ADR-O8: Login pages are presented as the app that sent the user

**Problem:** a cashflow user who clicks "Masuk" lands on pages that say "All-in-one", in English (cashflow is
Indonesian), with aio-only extras: a theme toggle, a "Login with Google" button that isn't implemented yet and a
"Forgot your password?" link to a page that doesn't exist. To someone who has never heard of aio it looks like a
different, possibly broken, site.

**Decision:** when an auth request opened the page (`/oauth/login`, or `/login` / `/signup` with `?next=` back
to it), aio presents it as that app's login:

- **Branding per client**, set by an admin, never by the request (a crafted authorize URL can't restyle the page):
  `brand_color` (`#rrggbb`) and `icon` (an emoji or 1-2 characters) on `oidc_clients` (migration 12), set with
  `oidc:client:create --brand-color --icon`, changed with `oidc:client:update` or `PATCH /api/v1/oidc/clients/{id}`.
  The header shows the icon and app name on the brand colour, buttons use it, and the text colour is picked for
  contrast. A small "Login secured by All-in-one" note under the form says whose account it is.
- **Language from the app**: the standard OIDC `ui_locales` parameter (cashflow sends `ui_locales=id`). aio
  stores the first language it supports (`en`, `id`) on the auth request and returns it as `locale`; the pages
  render from a small catalogue (`web/src/lib/oauth-i18n.ts`). The hand-off API's error messages follow
  `Accept-Language`, which the page sets to the app's language, so the shared response envelope is unchanged.
- **aio-only extras are hidden** in this mode (theme toggle, Google button, forgot-password link, the duplicate
  header buttons). The page waits for the app's details before showing the form so English doesn't flash first.
- aio's own login and signup pages (not opened by an app) are unchanged.

Found while testing it: the login page sent credentials through the shared API client, which treats any 401 as
an expired session and redirects to `/login`, dropping `?next=`. A mistyped password therefore lost the app
hand-off and the next login landed on aio's dashboard. Credentials now go through plain `fetch`.

**Rejected:** an embedded login form inside cashflow (RFC Option D) would remove the hop entirely but puts
passwords in every app; a fully custom per-app theme (logo images, fonts) is more than first-party apps need now
and can extend `Branding` later.

---

## ADR-O9: Moving existing cashflow accounts (`users:import`)

**Problem:** production cashflow already has users. Switching it to `AUTH_PROVIDER=aio` without them
would make their passwords useless and give them new, empty accounts.

**Decision:** a one-off CLI, `all-in-one users:import` (RFC-001 §7.3), in `internal/authnz/userimport`:

- **Reads cashflow's database directly** from `CASHFLOW_DATABASE_URL` (an env var, so the password stays
  out of the process list): users with a local password and no `aio_user_id`. cashflow and aio share one
  Postgres server, so this replaced the RFC's export file and SQL script: password hashes never touch disk.
- **Copies username + bcrypt hash as-is** into aio (both apps use bcrypt), so users keep their passwords.
  CLI only, never an API endpoint: accepting pre-hashed passwords is safe only from a trusted operator.
- **Dry run by default.** `--apply` writes nothing while any account is skipped, creates the aio accounts
  in one transaction, then sets `users.aio_user_id` in cashflow in one transaction (refusing if an account
  vanished or is linked to a different aio account).
- **Skipped until the operator decides:** a username aio already has (`--link-existing <name>` when it is the
  same person, who then logs in with the aio password; otherwise rename one side), a case-only match, aio's
  bootstrap admin name (`rbac.admin_username`: RBAC bootstrap would grant it the admin group; linking is
  allowed), aio's shared demo account (never imported or linked), invalid usernames or hashes, duplicates.
- **Idempotent:** an aio account with the same username and hash counts as already imported, so a re-run
  after a failed link only links. Accounts already linked aren't read again.

cashflow needs no code for this; its README has the runbook (including a Kubernetes Job, since aio's image
is distroless). `password_hash` stays in cashflow for now, so `AUTH_PROVIDER=local` remains a rollback.

---

## Verification

Unit and in-process tests cover the full code flow (ID token verified against the published keys),
single-use codes, wrong secrets, unregistered redirect URIs, blocked users, revoked clients, the demo account,
logout with and without a hint, and memory-store expiry. Review fixes added tests for a login link opened in
another browser, a callback without the completing user's session, several logins in one browser, refused
`prompt=login`/`max_age`/missing PKCE, logout with another user's hint or a rejected post-logout URI, codes
redeemed concurrently, and keys stored by another replica. ADR-O8 added tests for branding validation, storage
and updates, `ui_locales` → `locale`, localized hand-off errors, and browser checks of the branded pages (colour,
Indonesian copy, page language, Indonesian wrong-password error that keeps the hand-off, aio's own page
unchanged). ADR-O9 was checked against one Postgres server holding both databases: dry run, refused apply
with skips, `--link-existing` + renames, apply, idempotent re-run, then an existing cashflow session
surviving the switch and logins through aio with the old cashflow password landing on the same account and
cashplans. A real browser run against both apps on separate hosts
(aio on `127.0.0.1`, cashflow on `localhost`) passed 18/18 checks: signup through cashflow, logout ending both
sessions, login with an existing account, single sign-on with no form, and demo refusal. It also found three
bugs, all fixed: the CSP issue above, the demo account issue above, and a signup link that dropped `next`.

## Not in this build

Admin-issued password reset codes, back-channel logout and an admin UI page for clients (RFC-001 §6.3, §7.3; Deck "Later" cards).
