# RFC-003: Password reset by email

- **Status:** Draft; implemented in aio on `feat/password-reset-email` (cashflow and tally links pending)
- **Author:** opan
- **Created:** 2026-10-19 · **Updated:** 2026-10-24
- **Consumers:** cashflow, tally (both log in through aio, RFC-001)
- **Related:** [RFC-001](RFC-001-central-user-management.md) §6.3 (replaced by this RFC), [OIDC_PROVIDER_ADR](../adr/OIDC_PROVIDER_ADR.md) (ADR-O8 branded pages), Deck "OIDC · Admin-issued password reset codes"

## Complexity score: **6 / 10**

aio sends no email today and users can't set their own email. This adds email sending, email verification,
a forgot-password flow with temporary passwords, an admin reset, and a forced password change that also
works in the middle of an app login.

## 1. Summary

Email becomes the way to recover an account:

- **Users with a verified email** use "Forgot password?" on the login page. aio emails a temporary password.
  Logging in with it forces the user to pick a new password before they continue.
- **Users without one** email the admin from the address they want to use. The admin checks a challenge
  question from the app's data (cashflow: their last transaction; tally: their last match or players), saves
  that address as verified, and clicks "Send temporary password". aio emails it; the admin never sees it.
- **Users are nudged to add an email:** after login, users without a verified email are asked to add one,
  with a "Later" button. Email stays optional at signup.

Email is sent through Brevo's SMTP relay from `noreply@mail.opanmustopah.com`. Nothing changes until `email.enabled` is turned on.

## 2. Motivation

cashflow and tally users log in through aio, and a user who forgets their password has no way back in:

- The branded login page has no "Forgot password?" link (hidden by ADR-O8, since aio's own link points to a
  page that doesn't exist). A wrong password only says "Nama pengguna atau kata sandi salah."
- aio has no reset endpoint and no admin reset. The only workaround is writing a password hash into the
  production database, after which the admin knows the password.
- Most users have no email: signup makes it optional, users imported from cashflow have none, and users can't
  add one themselves.

RFC-001 §6.3 planned admin-issued reset codes handed over by WhatsApp. The operator prefers email: it gives a
self-service path once a user has an email, and a reason for users to add one.

## 3. Current state

- `users.email` is nullable and unique (migration 09). There is no "verified" flag; the ID token always says
  `email_verified: false` (`internal/oidc/service/storage.go`).
- The email field on aio's settings page is read-only. Admins can change it (`PATCH /api/v1/admin/users/{id}`).
- `POST /api/v1/users/reset_password` is a *change* password: logged-in session plus current password. It
  revokes the user's other aio sessions (`SessionRepo().DeleteByUserIDExcept`).
- Passwords: bcrypt, 5 to 72 bytes. 2FA (TOTP) is optional per user.
- App logins go through aio's `/login?next=/oauth/login?authRequestID=…`, which posts to
  `POST /api/v1/sessions`, then the auth request is completed. One login handler serves both cases.
- aio's own login page links to `/forgot-password`, which doesn't exist.
- No email sending, no audit table; operational events go to structured logs and OTel metrics.

## 4. Goals / Non-goals

**Goals**
- Users with a verified email can reset their password without the admin.
- Users without one have a documented path through the admin, who never learns the new password.
- A reset can't lock anyone out: requesting one doesn't disable the current password.
- More users add a verified email over time.
- Works the same from cashflow, tally and aio itself, in the app's language and branding.

**Non-goals**
- Making email required at signup.
- Recovering a lost 2FA device (the admin disabling 2FA is a separate, later feature).
- Ending app sessions after a reset (back-channel logout, RFC-001 follow-up).
- Email for anything other than account security (no newsletters or notifications).

## 5. Options

| Option | Verdict |
|---|---|
| **A. Emailed temporary password, forced change on use** | **Proposed** (operator's choice). Familiar to users: copy, log in, pick a new password |
| B. Emailed reset link with a one-time token | Industry default and slightly safer (nothing typed by hand, no second login), but the operator prefers temporary passwords. The data model (§6.3) would serve either |
| C. Admin-issued codes over WhatsApp (RFC-001 §6.3) | Replaced. No self-service path and no push toward email |
| D. Admin sets a password and tells the user | Rejected. The admin knows a working password |

## 6. Design

### 6.1 Sending email

```yaml
email:
  enabled: false
  from: "All-in-one by Opan <noreply@mail.opanmustopah.com>"
  smtp:
    host: "smtp-relay.brevo.com"
    port: 587            # STARTTLS
    username: "<Brevo SMTP login>"
    # password: env ALLINONE_EMAIL_SMTP_PASSWORD (a Brevo SMTP key, from the k8s secret)
auth:
  support_email: "opan.neutron@gmail.com"   # shown to users who have no email on their account
```

- New package `internal/email`: a `Sender` interface with an SMTP implementation (`net/smtp` with STARTTLS,
  10 s timeout) and a recording fake for tests. Plain-text plus simple HTML body, from templates in `en` and `id`.
- Sending happens in the request but failures never change the user-facing answer (§6.4). A failure is
  logged and counted (`aio_email_sent_total{kind, result}`).
- Brevo's free plan allows 300 messages a day, enough for these apps. `mail.opanmustopah.com` is
  authenticated in Brevo (DKIM, DMARC). Click and open tracking stay off, so links in emails are not
  rewritten. Any SMTP provider (Gmail with an app password included) works through the same settings.
- With `email.enabled: false`, the forgot-password page only shows the "contact the admin" path, and adding
  an email in settings is unavailable.

### 6.2 Adding and verifying an email

- **Settings:** the email field becomes editable. Saving sends a verification link to the new address; the
  account shows it as "pending" until the link is used. A previously verified email stays in use until then.
- **Signup:** an email given at signup gets the same verification link.
- **Verification link:** `/verify-email?token=…`, valid 24 hours, single use, works in any browser without
  logging in. Using it sets `users.email` and `users.email_verified_at`. If the address was taken by another
  account in the meantime, verification fails with a generic message.
- **Changing a verified email** also sends a notice to the old address ("your email was changed to n•••@…").
- **Admin:** `PATCH /api/v1/admin/users/{id}` gains `email_verified: true`, used when the user has written to
  the admin from that address (§6.5). Admin › Users shows each user's email and whether it is verified.
- **ID token:** `email_verified` reflects `email_verified_at`. Only verified emails are put in the `email` claim.
- **Taken address:** the settings form answers "This email can't be used" without saying whether another
  account has it.

### 6.3 Data

Migration 13 (postgres and sqlite):

```
users
  + email_verified_at   TIMESTAMP NULL
  + email_prompted_at   TIMESTAMP NULL      -- last time the "add your email" prompt was skipped

email_verifications                         -- pending email changes
  id, user_id, email, token_hash, expires_at, used_at, created_at

password_resets                             -- temporary passwords
  id, user_id, temp_password_hash, expires_at, attempts, used_at,
  issued_by NULL (self-service) | admin user id, created_at

sessions
  + password_change_required  BOOLEAN NOT NULL DEFAULT false
```

Tokens and temporary passwords are stored hashed only.

### 6.4 Forgot password (user has a verified email)

```
User                     aio                                        User's mailbox
 │ login page: "Lupa kata sandi?"                                        │
 │── username or email ─> POST /api/v1/auth/password/forgot              │
 │<── "If this account has a verified email, we sent a temporary         │
 │     password. It works for 30 minutes." (always the same answer)      │
 │                         │── temp password (if verified email) ──────> │
 │── username + temp password ─> POST /api/v1/sessions                   │
 │                         │ temp password matches: burn it, create a    │
 │                         │ session marked password_change_required     │
 │<── "Choose a new password" page                                       │
 │── new password ──────> POST /api/v1/users/password/set                │
 │                         │ set password, void other temp passwords,    │
 │                         │ revoke other sessions, clear the flag,      │
 │                         │ email "your password was changed"           │
 │<── continues to where the login was going (the app, or aio home)      │
```

- **Temporary password:** 12 characters from an alphabet without look-alikes (no 0/O, 1/l/I), shown in the
  email grouped as `xxxx-xxxx-xxxx`; dashes and case are ignored when typed.
- **The current password keeps working** until the user sets a new one, so anyone can request a reset for
  any username without locking its owner out. The owner only gets an email they can ignore.
- **Expiry and attempts:** 30 minutes, single use. A new request voids earlier temporary passwords. Wrong
  temporary passwords count toward the existing login rate limit; 5 failures void it.
- **Limits per account:** at most one email every 5 minutes and 5 a day per user, so the form can't be used
  to flood a mailbox. New rate-limit target `auth.password.forgot` (per IP) for the endpoint itself.
- **No account enumeration:** the same answer and similar timing whether or not the username or email exists,
  has an email, or is verified. Unverified emails never receive a temporary password.
- **2FA still applies:** after the temporary password, users with 2FA enter their code as usual.
- **Blocked accounts** get the same answer and no email; a reset never unblocks.
- **Restricted session:** while `password_change_required` is set, the session may only load the "choose a new
  password" page and call `POST /api/v1/users/password/set` (new password only, no current password) and
  logout. Every other API, including completing an app's auth request, answers `403 password_change_required`.
  The flag is checked on the server; the page redirect is only for convenience.
- **The page in an app login** is branded and translated like the other app login pages (ADR-O8), and after the
  change it returns to `next`, so the user lands back in cashflow or tally logged in.
- **The forgot-password page** replaces the dead `/forgot-password` link on aio's login page and is linked from
  the branded app page as "Lupa kata sandi?" / "Forgot password?". It also explains the path for users
  without an email (§6.5).

### 6.5 Users without an email (through the admin)

The forgot-password page says: "No email on your account? Email the admin at <support_email> from the address
you want to use, with your username." The link opens a prefilled email.

```
User                         Admin (operator)                              aio
 │── email from budi@x: "forgot my password, username budi" ─>│            │
 │<── challenge question from the app's data ─────────────────│            │
 │── answer ─────────────────────────────────────────────────>│            │
 │                             │ Admin › Users › budi:                     │
 │                             │ set email budi@x, mark verified ────────> │
 │                             │ "Send temporary password" ──────────────> │ POST /api/v1/admin/users/{id}/password-reset
 │<── email with temporary password (from aio) ────────────────────────────│
 │ then the same login and forced change as §6.4                           │
```

- **Identity check is the real control.** Anyone can claim to be "budi". Before acting, the admin asks
  something only the owner knows, using the read-only admin pages from RFC-002:
  - cashflow: their last transaction (roughly the amount, date or description), or the name of a cashplan.
  - tally: their last match, or some players in one of their tallies.
- The request must come **from the address being added**, which proves the user controls it; that is why the
  admin may mark it verified without a link.
- `POST /api/v1/admin/users/{id}/password-reset` (admin only) requires a verified email, creates the temporary
  password with `issued_by` set, and emails it. The response never contains the password.
- Logged with admin id, user id and reason (`admin_reset`); metric `aio_password_reset_total{source, result}`
  with `source` `self` or `admin`.

### 6.6 Asking users to add an email

- After a successful login (aio's own, or an app login before returning to the app), a user without a
  verified email sees "Add your email so you can reset your password if you forget it", with an email field
  and a "Later" button.
- "Later" sets `email_prompted_at`; the prompt comes back after 7 days. Saving sends the verification link
  and continues; the user doesn't have to verify before continuing.
- In an app login the page is branded and translated like the login page. It's skipped for the shared demo
  account and when `email.enabled` is false.

### 6.7 Starting from the app

Users start these flows from cashflow or tally; the forms themselves stay on aio, so passwords are only ever
typed into aio (RFC-001 chose hosted pages for this reason).

- **App links:**
  - "Lupa kata sandi?" next to the app's own "Masuk" button, opening aio's forgot-password page.
  - "Ubah kata sandi & email" in the app's menu for logged-in users, opening aio's settings for password
    and email.
- **Branded standalone pages:** these links aren't part of a login, so there is no auth request to take the
  branding from. They pass `client_id` and `return_to` instead:
  `https://aio…/forgot-password?client_id=cashflow&return_to=https://kas…/`.
  - Branding (name, icon, colour) still comes from the registered client, set by an admin, so a crafted link
    can only pick between registered apps, not restyle the page. Language comes from `ui_locales`, as in
    logins.
  - `return_to` must match one of that client's registered redirect or post-logout URIs (same origin);
    anything else is dropped and the page links to aio's home instead. This prevents open redirects.
  - After the change the user gets a "Back to Cashflow" button to `return_to`.
- **Guides:** cashflow's still says passwords need at least 8 characters; with aio it's 5. Both guides explain
  "Lupa kata sandi?", adding an email, and why these pages are on aio's address (§6.8).

### 6.8 Telling users why the address changes

Logins, password pages and the email prompt open on `aio.opanmustopah.com`, not the app's address. Users should
expect this, and learn that it is the only other address where their password belongs.

- **On aio's branded pages:** a short notice under the header, above the form (not only in the footer):

  > 🔒 Your Cashflow account is managed by All-in-one, so this page is on **aio.opanmustopah.com**.
  > Only enter your password on this address.

  > 🔒 Akun Cashflow kamu dikelola oleh All-in-one, jadi halaman ini ada di **aio.opanmustopah.com**.
  > Masukkan kata sandi hanya di alamat ini.

  The host is taken from `auth.oidc.issuer`, never from the request, so it always shows the real address.
  The existing footer ("Login secured by All-in-one" and "One account for …") stays.
- **In the apps, before leaving:** under "Masuk" and next to "Lupa kata sandi?" and "Ubah kata sandi & email":
  "Kamu akan dibawa ke aio.opanmustopah.com, tempat akun kamu dikelola." The host comes from `AIO_ISSUER`.
- **In emails:** sent as "All-in-one" and naming the app the user knows: "You asked to reset the password for
  the account you use in Cashflow. Your account is managed by All-in-one (aio.opanmustopah.com)." When the
  request didn't come from an app, the email names all registered apps the user has logged in to, or none.
- The aio-side notice also applies to today's login pages, so it can ship before the email work.

## 7. Security considerations

- **Temporary password in an inbox:** anyone who can read the user's email can reset the password; that's
  true of every email-based reset. Mitigated by the 30-minute expiry, single use, the forced change, 2FA still
  applying, and the "your password was changed" notice.
- **No lockout:** requesting a reset never disables the current password (§6.4).
- **Enumeration:** forgot-password answers don't reveal whether an account or email exists. Email changes in
  settings answer with a generic message for taken addresses.
- **Typos:** only verified emails receive temporary passwords, so a mistyped address gets a verification link
  that is never used, not a working password.
- **Admin path:** the admin never sees the temporary password. Marking an email verified requires the user
  to have written from it; this is a manual check, documented in the runbook.
- **App sessions** started before a reset stay valid until they expire or the user logs out; ending them
  needs back-channel logout (RFC-001 follow-up). aio's own sessions are revoked on the change.
- **Sender account:** the Brevo SMTP key can send mail as `mail.opanmustopah.com`; it lives only in the
  Kubernetes secret and is never logged.

## 8. Rollout

1. aio: the address notice on branded pages (§6.8), which can ship first.
2. aio: email package and config, migration 13, email verification and editable settings, forgot-password
   flow and pages, admin reset, prompt, metrics, swagger, tests (fake sender), README section.
3. Homelab: add the Brevo SMTP key to `all-in-one-secrets` (`smtp_password`, mapped to
   `ALLINONE_EMAIL_SMTP_PASSWORD`), set `email.*` and `auth.support_email` in the configmap.
4. Tell users (cashflow and tally): add an email in aio so you can reset your password yourself.
5. cashflow and tally: the in-app links and leaving notice (§6.7, §6.8) and guide updates.

## 9. Open questions

- **Replies:** `mail.opanmustopah.com` has no MX record, so replies to aio's emails bounce. Add an optional
  `email.reply_to` (e.g. the support address)?
- **Lost 2FA device:** out of scope here; needs its own admin action with stricter identity checks.

Settled with the operator (2026-10-19):
- Email instead of WhatsApp; temporary passwords instead of codes or links.
- Gmail SMTP with an app password. Replaced by Brevo from `All-in-one by Opan <noreply@mail.opanmustopah.com>`
  (2026-10-29).
- Emails must be verified before they receive temporary passwords.
- The add-email prompt can be skipped ("Later"); email stays optional at signup.
- Users start from the app, but password forms stay on aio (2026-10-21).
- Branded pages say the account is managed by All-in-one and that the address is aio.opanmustopah.com (2026-10-21).

## 10. Decision

Pending review.

## 11. Implementation notes (aio, 2026-10-24)

Where the build differs from or adds to the design above:

- **Forgot password answers first, then works.** The lookup, limits, temporary password and email run after
  the response, so neither the answer nor its timing says anything about the account (§6.4 said "similar
  timing"). The admin path and email confirmation send in the request and report send failures (502).
- **`server.public_url`** is the base for links in emails, defaulting to `auth.oidc.issuer`, so email works
  without OIDC.
- **`/account`** is a new branded page (password and email) that apps link to, instead of aio's settings
  page, so app users never land in aio's dashboard. The login page in front of it is branded too: pages read
  the app from `?client_id=` on themselves or inside `?next=`.
- **Standalone branding** is `GET /api/v1/oidc/apps/{id}?return_to=` (public). `return_to` must have the same
  scheme and host as one of the client's registered redirect or post-logout URIs.
- **Restricted session**: the JWT middleware allows only `GET /users/me`, `POST /users/password/set` and
  `DELETE /sessions`, and answers `403 password_change_required` otherwise; the app hand-off page sends such
  users to `/set-password?next=<hand-off>`. `/users/me` reports `password_change_required`, and aio's own
  pages redirect on it.
- **Temporary passwords** are 12 characters from `abcdefghjkmnpqrstuvwxyz23456789`, stored with bcrypt, and
  only compared when the typed password has that shape. Logging in with the real password voids pending
  ones. Admin-issued ones record `issued_by`.
- **Signup with email on**: the address waits in `email_verifications` and becomes the account's email only
  when confirmed, so nobody can squat an address. Confirming an address clears an unconfirmed copy of it on
  another account; a confirmed copy elsewhere makes confirmation fail ("can't be used").
- **`users.email` for existing accounts** stays as it was but counts as unverified until confirmed (by link or
  by the admin), so admin-set emails from before this change don't receive temporary passwords.
- **Metrics:** `aio_authnz_password_resets_total{source,result}`, `aio_authnz_email_verifications_total{result}`,
  `aio_email_sent_total{kind,result}`, and `logins_total{result="temp_password"}` (docs/metrics.md).
- **Tests:** end-to-end handler tests on SQLite with every migration, plus a browser run against Postgres
  with a STARTTLS mail catcher (20 checks: both recovery paths, branded pages, return_to checks, admin UI).
