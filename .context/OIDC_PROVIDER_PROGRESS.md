# OIDC Provider (aio as central auth) — Progress

> **Live tracker: Nextcloud Deck board "aio · Central auth (OIDC)"** — https://nc.opan.dev/index.php/apps/deck/board/10
> (owned by `llm-bot`, shared with `opan`). This file only records decisions and pointers for a resumed session.
> **Design:** `docs/rfc/RFC-001-central-user-management.md` (PR #31), Option A.
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

## Out of scope for this build (Deck "Later" column)

Admin-issued reset codes, `users:import`, back-channel logout, admin UI page for clients.
