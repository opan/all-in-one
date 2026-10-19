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
- Gmail SMTP with an app password (`ALLINONE_EMAIL_SMTP_PASSWORD`).
- After login, users without a verified email get an "add your email" prompt with "Later" (back after 7 days).
  Email stays optional at signup.

## Status

- [x] RFC-003 draft
- [ ] RFC review by opan
- [ ] email package + config (SMTP, fake sender for tests)
- [ ] migration 13 (email_verified_at, email_prompted_at, email_verifications, password_resets,
      sessions.password_change_required)
- [ ] editable email + verification link; admin `email_verified`; ID token `email_verified`
- [ ] forgot-password endpoint + page (replaces dead `/forgot-password`), branded in app logins
- [ ] restricted session + "choose a new password" page, returns to `next`
- [ ] admin "Send temporary password"
- [ ] add-email prompt after login
- [ ] metrics (docs/metrics.md), swagger, otel, README, ADR at the end
- [ ] homelab: app password secret, enable email in configmap
- [ ] cashflow + tally guide updates
