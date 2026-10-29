# Password reset by email (RFC-003) — Progress

> **Design:** `docs/rfc/RFC-003-password-reset-by-email.md` (replaces RFC-001 §6.3)
> **Branch:** aio `feat/password-reset-email` (from main d0646cf)

## Decisions (opan, 2026-10-19)

- Email, not WhatsApp. Self-service reset needs a **verified** email; this is the push for users to add one.
- Temporary password (not a link or code), emailed; logging in with it forces a new password. The current
  password keeps working until then, so a reset request can't lock anyone out.
- Users without email: they email the admin from the address to add; admin asks an app challenge question
  (cashflow: last transaction; tally: last match or players), marks the email verified, clicks
  "Send temporary password". The admin never sees it.
- SMTP via Brevo (2026-10-29, was Gmail): `All-in-one by Opan <noreply@mail.opanmustopah.com>`, key in
  `ALLINONE_EMAIL_SMTP_PASSWORD`. Tracking off. No MX, so replies bounce (`email.reply_to` deferred).
- After login, users without a verified email get an "add your email" prompt with "Later" (back after 7 days).
  Email stays optional at signup.
- (2026-10-21) Users start from the app ("Lupa kata sandi?", "Ubah kata sandi & email" links), but forms stay
  on aio: standalone pages branded by `client_id`, `return_to` checked against the client's registered URIs.
- (2026-10-21) Notice on aio's branded pages: account is managed by All-in-one, so the address is
  aio.opanmustopah.com (host from the issuer); apps say so before leaving; emails name the app.

## Status

- [x] RFC-003 draft
- [ ] RFC review by opan (PR #36)
- [x] address notice on branded pages
- [x] email package + config (SMTP with STARTTLS, recorder for tests), `server.public_url`
- [x] migration 13 (tested up/down on SQLite and Postgres)
- [x] editable email + verification link (settings, `/account`, signup); admin `email_verified`; ID token
      `email_verified`
- [x] forgot-password endpoint + page (replaces dead `/forgot-password`), branded in app logins and from
      direct app links (`GET /api/v1/oidc/apps/{id}`)
- [x] restricted session + `/set-password`, returns to `next` (incl. the app hand-off)
- [x] admin "Send temporary password" + verified badge + "save as verified"
- [x] add-email prompt after login (`/add-email`, snooze 7 days)
- [x] metrics (docs/metrics.md), swagger, README section; RFC §11 implementation notes
- [x] tests: handler flow tests on SQLite (10), unit tests; browser e2e on Postgres + Mailpit (20/20)
- [ ] ADR (docs/adr) once cashflow/tally are done
- [x] homelab: configmap `email.*` (Brevo) + env `ALLINONE_EMAIL_SMTP_PASSWORD` from
      `all-in-one-secrets.smtp_password` (optional), homelab b073e6b (2026-10-29)
- [x] opan: SMTP key in the secret; delivery tested with curl via Brevo, lands in inbox (2 Gmail accounts)
- [x] `auth.support_email: support@mail.opanmustopah.com`, Cloudflare Email Routing → opan's Gmail
      (homelab 13f937b, 2026-10-29)
- [ ] opan: turn off Brevo click/open tracking (check: links in the reset email point at aio directly)
- [ ] after deploy: send a test reset email; later DMARC `p=quarantine`
- [ ] maybe: `email.reply_to` (no MX on mail.opanmustopah.com)
- [x] cashflow + tally: "Lupa kata sandi?" (landing) and `/akun` → "Ubah kata sandi & email" with the
      leaving notice, guide updates (cashflow: 5 characters). PRs opan/cashflow#6, opan/tally#4 (2026-10-27),
      tested against a local aio from this branch; merge after this is deployed
