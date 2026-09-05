# ADR: Demo mode (shared demo account + abuse guard rail)

This document records design decisions for **demo mode**: advertising a shared demo account on the
public landing page (`web/src/routes/+page.svelte`) and the login page (`web/src/routes/login/+page.svelte`)
so visitors can try the app without signing up, gated by a single backend flag that also acts as an
abuse guard rail.

Implementation status is tracked in `.context/DEMO_MODE_PROGRESS.md`. This document records the
*decisions*.

---

## ADR-001: Backend config as the single source of truth (not a frontend build-time flag)

### Status
Accepted

### Context
The credentials could have been hardcoded in the SPA behind a Vite build-time env var
(`VITE_DEMO_MODE`). That is the lightest option, but toggling it requires a frontend rebuild and the
flag can only affect what the UI renders — it cannot stop a determined visitor from using the demo
account, since the credentials are public by design.

The flag's primary purpose is a **guard rail**: if the demo account is abused, the operator wants to
disable it at runtime and have that actually block access, not just hide a button.

### Decision
Model demo mode as backend config (`internal/config`), read by both the backend (for enforcement)
and the frontend (for display) at runtime:

```yaml
demo_mode:
  enabled: true          # advertise + allow the demo account; false hides it AND blocks the demo user
  username: "demo"        # the account visitors log in with — created out of band
  password: "demo123"     # intentionally public (served via GET /api/v1/config)
```

Viper default is `enabled: false` (safe default if the key is absent); `config/config.yml` sets it
`true` for this deployment. Env overrides are bound: `ALLINONE_DEMO_MODE_ENABLED` /
`ALLINONE_DEMO_MODE_USERNAME` / `ALLINONE_DEMO_MODE_PASSWORD`.

The flag never provisions the account — it only displays credentials and gates access. Creating the
`demo` user is done out of band (it already exists in `internal/authnz/seed`).

---

## ADR-002: A public `GET /api/v1/config` endpoint for pre-auth client config

### Status
Accepted

### Context
The SPA is static (`adapter-static`, SSR disabled) and had no channel for backend-provided config —
the only env it read was Vite's built-in `import.meta.env.DEV`. The landing and login pages need to
know, before authentication, whether to advertise the demo account.

### Decision
Add a single public, unauthenticated endpoint `GET /api/v1/config` (handler `PublicConfig` in
`internal/http/http.go`, registered next to `HealthCheck` in `cmd/all-in-one/server/server.go`). It
returns only client-facing config:

```json
{ "success": true, "data": { "demo_mode": { "enabled": true, "username": "demo", "password": "demo123" } } }
```

Credentials are included **only when `enabled` is true** (`omitempty` on the response struct), so a
disabled demo never leaks a username/password. The endpoint is auto-traced by the existing
`otelmux` middleware like every other `/api/v1` route; it carries no secrets beyond the intentionally
public demo credentials, so no auth or new metrics are warranted.

The frontend reads it in the root `+layout.ts` load via `fetchDemoMode()` (`web/src/lib/config.ts`),
which defaults to `{ enabled: false }` on any error so a failed request never surfaces credentials.
`demoMode` is threaded to the landing page (props) and login page (`$page.data`).

---

## ADR-003: Guard rail enforced at login and refresh, keyed by username

### Status
Accepted

### Context
Hiding the UI is not enough — a disabled demo must actually be unusable. Enforcement needs to cover
both new logins and the renewal of any session that was already live when the flag was flipped.
Enforcing on every authenticated request (middleware) was considered but rejected as too broad for a
single-account guard rail.

### Decision
Add `Handler.demoLoginBlocked(username)` in `internal/authnz/handler/session.go`:
`!demo_mode.enabled && username == demo_mode.username` (case-insensitive, mirroring how the
credentials are advertised). Apply it at two choke points:

- **`CreateSession`** — checked first, before any DB lookup or password check, so a disabled demo is
  rejected outright and cannot be probed. Returns `403 "Demo access is currently disabled"`, with
  login metric label `result="demo_disabled"`.
- **`RefreshToken`** — checked after the user is loaded, so any in-flight demo session cannot renew
  and dies within the 30-minute access-token window.

This deliberately does not invalidate existing access tokens instantly (they remain valid until they
expire, at most 30 minutes) — an acceptable bound for an abuse guard rail, avoiding a per-request
check on the hot path. Non-demo users are never affected by the flag.
