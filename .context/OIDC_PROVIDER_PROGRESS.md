# OIDC Provider (aio as central auth) — Progress

> **Live tracker: Nextcloud Deck** — cards prefixed "OIDC" on the All-in-one board (https://nc.opan.dev/index.php/apps/deck/board/3)
> and the Cashflow board (https://nc.opan.dev/index.php/apps/deck/board/9). This file only records decisions for a resumed session.
> **Status:** implemented and verified end to end (2026-10-04); decisions in `docs/adr/OIDC_PROVIDER_ADR.md`.
> **Design:** `docs/rfc/RFC-001-central-user-management.md` (accepted, Option A; merged into this PR, #31 closed).
> **Branches:** aio `feat/oidc-provider` · cashflow `feat/aio-oidc-login`.

## Decisions made during the build

- **Library:** `github.com/zitadel/oidc/v3` (`pkg/op` for aio, `go-oidc` + `x/oauth2` in cashflow).
  Spike confirmed it mounts on gorilla/mux with all endpoints under `/api/v1/oauth2/` and discovery at
  `/.well-known/openid-configuration` (the only path outside `/api/v1`, fixed by the spec).
- **Persistence:** clients and signing keys in the DB (migration 11). Auth requests, codes and access tokens
  are in memory with a TTL: a restart mid-login only means the user starts the login again, and apps use the
  ID token once at login, so nothing long-lived depends on them.
- **Signing key:** RSA, generated on first start, private key encrypted with the existing
  `auth.totp_encryption_key` (AES-GCM helper already in `internal/auth`).
- **Config:** `auth.oidc.{enabled,issuer,auth_request_ttl,access_token_lifetime,id_token_lifetime}`;
  enabled requires an absolute http(s) issuer without a path.

## Review fixes (2026-10-05, PR #32 review passes 1 and 2)

- **Critical, login hand-off:** auth requests are bound to the starting browser (`aio_oidc_browser` cookie,
  `service/binding.go`); completion needs that cookie, and `authorize/callback` also needs aio's session for
  the completing user. A completed request can't be re-completed as another user.
- **Logout CSRF:** aio's session ends only when the hint's `sub` equals the session's `user_id`, and only from
  `TerminateSessionFromRequest` (after the library accepted the request).
- PKCE S256 required for all clients; `prompt=login` / `max_age` refused; codes used up atomically on lookup;
  JWKS publishes every active DB key. Single-replica limit documented (ADR-O3, RFC §8, config.yml);
  moving auth requests/codes to the DB is the follow-up.

## App-branded login pages (2026-10-10, ADR-O8)

- Clients carry `brand_color` + `icon` (migration 12); CLI `oidc:client:update`, admin `PATCH /oidc/clients/{id}`.
- `ui_locales` → `locale` on the auth request; pages render from `web/src/lib/oauth-i18n.ts` (en, id); hand-off
  API errors follow `Accept-Language`. Shared frame: `web/src/components/auth-shell.svelte`.
- cashflow sends `ui_locales=id`; register it with `--brand-color '#0f766e' --icon 💰`.
- Fixed: wrong password on the login page lost `?next=` (api.ts 401 redirect); login now uses plain fetch.

## Existing cashflow users (2026-10-10, ADR-O9)

- Production cashflow has users → `all-in-one users:import` (`internal/authnz/userimport`,
  `cmd/all-in-one/userimport`). Reads cashflow's DB from `CASHFLOW_DATABASE_URL` (same PG server), copies
  username + bcrypt hash, links `users.aio_user_id`. Dry run default; `--apply`; `--link-existing`.
- Protected names: `rbac.admin_username` (reserved, linkable), `demo_mode.username` (blocked).
- Runbook + Kubernetes Job in cashflow's README ("Memindahkan pengguna lama ke All-in-one").

## Out of scope for this build (Deck "Later" column)

Admin-issued reset codes, back-channel logout, admin UI page for clients.
