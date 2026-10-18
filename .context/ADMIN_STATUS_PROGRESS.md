# Admin status for apps (RFC-002) — Progress

> **Design:** `docs/rfc/RFC-002-admin-status-for-apps.md` · **Decisions:** `docs/adr/OIDC_PROVIDER_ADR.md` ADR-O10
> **Branches:** aio `docs/rfc-002-admin-status` (PR #34) · cashflow `feat/aio-admin-status` · tally `feat/aio-admin-status`

## Decisions (review of PR #34)

- Per-request check, **no cache**, so demotion/blocking applies on the next admin request (no 15-min window).
- No `aio_admin` ID token claim. A status fetched once at login may only show/hide the "Admin" link.
- Credential: the existing OIDC client id + secret (HTTP Basic); no separate token.
- Address is configurable: `AIO_INTERNAL_URL` (e.g. `http://all-in-one.app.svc.cluster.local`), falling back
  to `AIO_ISSUER`. Can switch between cluster and public later without code changes.
- cashflow overview: activity per user (counts, last activity, sign-up); rupiah figures only on a user's
  drill-down page, which is the audit-logged event. Revisit later. tally has no money.

## Status

- [x] aio: `GET /api/v1/oidc/users/{sub}/status`, metric, swagger, tests, smoke-tested on a real server
      (admin, regular, unknown, non-UUID, wrong/no/unknown credentials, demotion/block/revoke take effect
      on the next call)
- [x] cashflow (`feat/aio-admin-status`, 01f5d59): `AIO_INTERNAL_URL`, status client, `requireAdmin`, nav hint,
      `/admin` (activity, no amounts) + `/admin/users/{id}` (cashplans, totals), `admin_views` audit table,
      metric `cashflow.admin.checks{result}`. End-to-end run against aio: login hint, 404 for non-admins and
      local mode, live demotion/blocking, 503 when aio is down, audit rows, schema re-apply.
- [x] tally (`feat/aio-admin-status`, 8e2b522): same, as "Site admin" (en/id); drill-down lists tallies with
      role, visibility, players, play days, matches; private tallies not linked. Same end-to-end checks.
- [ ] homelab: set `AIO_INTERNAL_URL` on cashflow and tally deployments
