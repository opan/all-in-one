# Demo Mode — progress tracker

## Goal
Advertise a shared demo account on the landing + login pages so visitors can try
the app without signing up, gated by a backend `demo_mode` feature flag that also
acts as a **guard rail**: flipping it off hides the credentials in the UI *and*
blocks the demo user from logging in / renewing sessions (in case of abuse).

Decisions (agreed with user):
- Backend config as source of truth (not a frontend build-time flag), so it can be
  toggled at runtime as an abuse guard rail.
- Flag name: `demo_mode`.
- User creates the actual demo account themselves (`demo` / `demo123`); the flag
  does NOT provision the account, only displays credentials + gates access.

## Design
- Config `demo_mode: { enabled, username, password }`. Viper default enabled=false
  (safe); `config/config.yml` sets enabled=true, username=demo, password=demo123.
  Env overrides: `ALLINONE_DEMO_MODE_ENABLED|USERNAME|PASSWORD`.
- Public endpoint `GET /api/v1/config` (in internal/http/http.go, next to
  HealthCheck, registered on the public subrouter). Returns demo_mode.enabled;
  includes username/password only when enabled. Auto-traced by otelmux.
- Guard rail in authnz session handler: reject login (CreateSession, before the
  2FA branch) and refresh (RefreshToken, after loading user) when
  demo_mode.enabled=false and username == configured demo username. 403
  "Demo access is currently disabled". Login metric label result="demo_disabled".
- Frontend: root +layout.ts fetches /api/v1/config → demoMode threaded down.
  Landing page reads credentials from data.demoMode (renders callout + banner note
  only when enabled). Login page shows a demo-credentials hint when enabled.

## Checklist
- [x] config struct + defaults + BindEnv (internal/config/config.go)
- [x] config/config.yml demo_mode block
- [x] GET /api/v1/config handler (internal/http/http.go) + register (server.go)
- [x] guard rail in session.go (CreateSession + RefreshToken) + metric label
- [x] frontend: +layout.ts fetch (web/src/lib/config.ts) + landing page + login page
- [x] tests: guard rail (session_test.go), config endpoint (internal/http/config_test.go)
- [x] go build + go vet + go test ./... + npm run check (0 errors)
- [x] browser screenshot verify — enabled vs disabled, landing + login, plus
      curl checks: /config hides creds when off, demo login/refresh → 403, normal user unaffected
- [x] ADR: docs/adr/DEMO_MODE_ADR.md

## Status: COMPLETE
All checks green. Demo account (`demo`/`demo123`) already exists via authnz seed.
`config/config.yml` ships with demo_mode enabled; set `ALLINONE_DEMO_MODE_ENABLED=false`
(or edit config) to pull the guard rail.
