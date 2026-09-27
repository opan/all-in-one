# RFC-001: all-in-one as Central User Management (SSO) for other apps

- **Status:** Draft (for discussion — not an accepted decision)
- **Author:** opan
- **Created:** 2026-09-27
- **First consumer:** [cashflow](https://github.com/opan/cashflow)
- **Related:** [EXTERNAL_RATE_LIMIT_ADR](../adr/EXTERNAL_RATE_LIMIT_ADR.md) (the "aio as a service" + app-token pattern this reuses), [USER_AUTHENTICATION_ADR](../adr/USER_AUTHENTICATION_ADR.md)

## Complexity score: **7 / 10** (recommended approach)

| Approach | Score | One-liner |
|---|---:|---|
| **B — Redirect + app-token introspection (recommended)** | **7** | OIDC-lite, domain-independent, reuses app-token infra; the crux of the cost is migrating existing users, not the protocol |
| C — Shared cookie + local JWT/JWKS validation | 4–5 | Cheapest, but only works if every app is under one parent domain and shares the trust boundary; weak logout story |
| A — Full OIDC/OAuth2 provider | 9 | Standards-complete (discovery, JWKS, consent, dynamic clients, third-party); overkill for a first-party homelab |

The protocol work is medium. What pushes this to a 7 is that **auth is security-critical** and **existing cashflow accounts must be migrated/linked** — cashflow's `cashplans.owner_id` and dues rows all FK to its local `users.id`, so the identity can't simply be swapped underneath them.

---

## 1. Summary

Make aio the single source of truth for user identity (registration, login, password, 2FA, account state) and let other first-party apps (starting with cashflow) delegate authentication to it — "Login with all-in-one". Apps stop storing passwords; aio owns credentials and issues a verifiable identity that each app maps to a thin local user record.

This mirrors, for **identity**, exactly what RFC/ADR "External Rate Limiting" did for **rate limiting**: aio becomes the service, consumers call it, and the integration is feature-flagged so a consumer degrades to its own local behavior when the flag is off.

## 2. Motivation

- One account per person across the homelab, one place to reset a password or enable 2FA.
- Consumers shed security-critical code (password hashing, session issuance, 2FA) they shouldn't each reimplement.
- aio already has the hard parts: users, sessions, JWT, refresh, `/sessions/verify`, TOTP 2FA + recovery codes, RBAC, admin user management, and — from the rate-limit work — an **app-token** mechanism for authenticating cross-app service calls.

## 3. Current state (grounded in code)

### aio (`internal/authnz`)
- `users(id TEXT PK, username UNIQUE, email NOT NULL UNIQUE, name, password_hash, …)` — **id is a stable string** and is what the JWT carries as `user_id`. **Email is mandatory and unique.**
- Sessions persisted (`SessionRepository`), validated per request by the JWT middleware (session id = JWT `sub`).
- JWT is **HS256, signed with a shared symmetric secret** (`auth.jwt_secret`); cookies `access_token` + `refresh_token`, `SameSite=Lax`, `Secure` configurable.
- Endpoints already present: `POST /users` (register), `POST /sessions` (login), `POST /sessions/refresh`, `GET /sessions/verify`, `POST /sessions/2fa/verify`, `…/2fa/recovery`, `GET /users/me`, `DELETE /sessions` (logout).
- 2FA (TOTP) with challenge tokens explicitly rejected on protected routes.

### cashflow
- `users(id uuid PK, username UNIQUE, password_hash)` + `sessions(id token PK, user_id FK, expires_at)`; bcrypt passwords; opaque `cashflow_session` cookie (30-day TTL); `withUser` middleware loads the user via `UserBySession`.
- **Every domain row keys off cashflow `users.id`**: `cashplans.owner_id → users(id)`, dues members, etc.
- No email column on cashflow users.

### The two gaps that make this non-trivial
1. **Cross-origin sessions.** aio and cashflow are different origins. A single shared cookie only works if both sit under one parent domain (`Domain=.example.com`); otherwise identity must be handed over by redirect.
2. **Existing identities.** cashflow rows already reference cashflow `users.id`. Central auth means each cashflow user must be *linked* to an aio account (and aio requires an email cashflow never collected).

## 4. Goals / Non-goals

**Goals**
- aio is the authority for credentials, 2FA, and account lifecycle.
- A consumer authenticates a user via aio without ever seeing the password.
- Existing cashflow users keep their data (plans, dues) after migration.
- Feature-flagged: flag off ⇒ cashflow uses its current local auth unchanged.
- Reuse the existing **app-token** service-auth (no new secret-distribution scheme).

**Non-goals (v1)**
- Full OIDC/OAuth2 compliance, discovery documents, third-party/dynamic client registration, consent screens.
- Social login / external IdPs.
- Cross-device session sync or a central session-management UI for consumers.
- Fine-grained cross-app authorization (RBAC stays per-app; aio only asserts *identity*).

## 5. Design options

### Option A — Full OIDC/OAuth2 provider (score 9)
aio implements authorization-code + PKCE, `/.well-known/openid-configuration`, JWKS, ID tokens, refresh, consent, client registry. **Pro:** standards-based, future-proof, any OIDC client works. **Con:** large surface, much of it (consent, discovery, third-party clients) is unused by a handful of first-party apps. Deferred as the eventual target if external consumers ever appear.

### Option B — Redirect + app-token introspection (recommended, score 7)
An OIDC-lite subset that reuses what exists:
1. cashflow redirects an unauthenticated user to aio `GET /api/v1/auth/authorize?client_id=cashflow&redirect_uri=…&state=…&code_challenge=…`.
2. aio authenticates the user with its **own** login + 2FA (reusing today's flow/UI), then redirects back to cashflow's registered `redirect_uri` with a short-lived, single-use **authorization code**.
3. cashflow's callback exchanges the code at aio `POST /api/v1/auth/token` **authenticated by its app token** (`X-API-Key`, the same credential type minted for rate limiting) + PKCE verifier, receiving `{ user_id, username, email, expires_at }`.
4. cashflow **JIT-provisions** a local shadow user keyed by aio `user_id` and mints its **own** `cashflow_session` (minimal change to `withUser`).

**Pro:** domain-independent (works across origins), no shared JWT secret sprawl, reuses app-token auth and aio's existing login/2FA, small blast radius on the consumer. **Con:** a redirect round-trip; aio must implement code issuance/exchange + a redirect-URI allowlist.

### Option C — Shared cookie + local JWT validation (score 4–5)
If every app is served under one parent domain, aio sets the auth cookie with `Domain=.opan.dev`; each app validates the JWT locally (shared secret, or aio publishes JWKS) and maps `user_id` to a local shadow user. **Pro:** cheapest, near-zero protocol. **Con:** requires one cookie domain, spreads the signing trust to every app, `SameSite`/CSRF care, and logout/revocation is awkward (cookie deletion doesn't invalidate other apps). Acceptable as a **Phase 0 shortcut** for a same-domain homelab, but it doesn't generalize.

## 6. Recommended approach

**Adopt Option B as the target.** It reuses the app-token service-auth already shipped for rate limiting, works regardless of domain, and keeps credentials/2FA solely in aio. Keep a **feature flag** on the consumer (`AUTH_PROVIDER=aio|local`) exactly like `AIO_RATELIMIT_ENABLED`, so cashflow falls back to local auth if aio is unavailable or the rollout is paused.

Consumers keep a **thin local `users` shadow table** (so existing FKs and `owner_id` stay valid) linked to aio by `aio_user_id`. New users are provisioned just-in-time on first aio login.

## 7. Changes required

### 7.1 aio (provider)
- **Migration 11** — a client/redirect allowlist. Extend the existing `app_tokens` concept (add `redirect_uris`, `client_id`) or a small `auth_clients` table linking a client_id to an app token + allowed redirect URIs + PKCE requirement.
- **`internal/authnz` — authorize endpoint** (`GET /api/v1/auth/authorize`): validates `client_id` + `redirect_uri` against the allowlist, drives the existing login + 2FA, issues a single-use authorization code (short TTL, bound to client + redirect_uri + PKCE `code_challenge`). New code, but reuses today's session/2FA machinery.
- **Token/exchange endpoint** (`POST /api/v1/auth/token`): app-token-authenticated (`X-API-Key`) + PKCE verifier; returns `{ user_id, username, email, expires_at }`. Single-use, replay-protected.
- **(Optional) userinfo/introspection** (`GET /api/v1/auth/userinfo`) for a consumer to re-verify identity without a full re-login.
- **Logout propagation:** front-channel logout callback or rely on short consumer-session TTL + periodic re-check (v1 can start with short TTL).
- **Config:** authorization-code TTL, per-client redirect-URI allowlist.
- **Cross-cutting:** otel spans/metrics on the new endpoints, swagger annotations, an ADR at closeout, tests (redirect-URI validation, code single-use/replay, PKCE, scope of returned claims).
- Estimated ~800–1200 LoC incl. tests; reuses existing login/session/2FA/app-token code.

### 7.2 cashflow (consumer)
- **Shadow user table:** add `aio_user_id TEXT UNIQUE` to `users`; make `password_hash` nullable (unused under aio auth, retained for the rollback window). `id` stays the PK so `cashplans.owner_id` and dues FKs are untouched.
- **New auth flow:** `GET /auth/login` → redirect to aio authorize; `GET /auth/callback` → exchange code with aio (using the app token) → find-or-create shadow user by `aio_user_id` → create local `cashflow_session`. `withUser` is essentially unchanged.
- **Retire local credential handling** (register/login/password) behind the flag; keep it as the `AUTH_PROVIDER=local` fallback.
- **Feature flag** `AUTH_PROVIDER` (+ `AIO_AUTH_URL`, reuse `AIO_RATELIMIT_TOKEN`-style app token). Off ⇒ today's behavior exactly.
- Estimated ~400–700 LoC + a DB migration + a one-time link/migration step.

### 7.3 Data migration (the hard, risky part)
1. Add `aio_user_id` (nullable) to cashflow `users`.
2. For each existing cashflow user, ensure a matching aio account exists — matched by `username`. **aio requires an email**, which cashflow never collected, so the operator must either supply emails or generate deterministic placeholders (e.g. `<username>@cashflow.local`) to be corrected later.
3. Write `aio_user_id` back onto the cashflow row (the link).
4. Invalidate existing cashflow sessions; users re-authenticate once via aio.
5. After a rollback window, drop `password_hash` from cashflow.

This step — not the protocol — is where correctness and reversibility must be proven (dry-run on a DB copy, reversible link, no orphaned `owner_id`).

## 8. Security considerations

- **Redirect-URI allowlist** is mandatory (exact-match) — the classic open-redirect / code-interception risk.
- **PKCE** on the code flow even for confidential clients, to harden code exchange.
- **Single-use, short-TTL authorization codes**; reject replay.
- **App token** already: SHA-256, prefix-scoped, admin-issued, revocable (reused as the client credential for `/auth/token`).
- **Session fixation / CSRF:** `state` parameter on the redirect; `SameSite` on the consumer session.
- **Blast radius:** aio becomes a single point of failure for login. Mitigations: consumer fail-*closed* for new logins (unlike rate limiting, auth must not fail open) but existing valid consumer sessions keep working until they expire, so an aio outage doesn't log everyone out instantly.
- **Fail-open vs fail-closed differs from rate limiting:** a rate-limit check fails open; an *authentication* decision must fail closed. This is the key philosophical difference from EXTERNAL_RATE_LIMIT and must be stated loudly in the implementation.
- A full security review is a required gate before enabling in production.

## 9. Rollout plan

1. Ship aio provider endpoints behind an off-by-default `auth.provider.enabled` switch; verify with a throwaway client.
2. Add cashflow's `AUTH_PROVIDER=aio` path while `local` remains default.
3. Dry-run the user-link migration on a copy of cashflow's DB; verify no orphaned `owner_id`.
4. Cut over cashflow in a low-traffic window; keep `local` fallback for one release.
5. Drop `password_hash` after the rollback window; write the ADR.

## 10. Open questions

- **Email for existing users:** collect real emails pre-migration, or placeholder-then-correct? (Blocks the migration; aio requires email.)
- **Domain layout:** are all apps under one parent domain? If yes, Option C is a legitimate Phase-0 shortcut worth its own mini-RFC.
- **Session ownership after login:** consumer mints its own session (recommended, minimal churn) vs. consumer validates aio's JWT on every request (tighter revocation, more coupling).
- **Logout propagation:** short TTL (simple) vs. front-channel logout (immediate, more work) for v1.
- **RBAC:** does aio's RBAC ever gate consumer features, or does each app keep its own authz? (v1: identity only.)

## 11. Decision

Pending. This RFC recommends **Option B**, phased, feature-flagged, with the user-link migration treated as the primary risk. On acceptance, split into a phased implementation plan under `.context/` and an ADR at closeout (per the repo's `docs/adr` convention).
