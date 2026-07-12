# URL Shortener

## Overview

The shortener feature lets authenticated users create short links (`/r/{code}`) and manage them (list, update, delete). The redirect endpoint (`GET /r/{code}`) is public — no login required to follow a link.

## Rate Limiting

As of ADR-011, the shortener no longer has its own limiter — its two rate-limited endpoints are registered
as targets of the platform **ratelimit app-feature** (`internal/ratelimit`), so their limits are DB-backed,
runtime-editable from the admin UI (`/admin/ratelimit`) or API, and enforced by the shared limiter
middleware. Throttle counters are in-memory fixed-window buckets (single-instance only, reset on restart).

### Create target — `shortener.link.create`

| Property | Value |
|---|---|
| Endpoint | `POST /api/v1/shortener/links` |
| Scope (key) | User ID (authenticated) — falls back to client IP if unauthenticated |
| Kind | throttle |
| Default limit | 100 creates per 15 minutes |

Each user gets its own bucket. Exceeding the limit returns `429 Too Many Requests` with a `Retry-After` header.

### Resolve target — `shortener.link.resolve`

| Property | Value |
|---|---|
| Endpoint | `GET /r/{code}` (public redirect, root router — outside `/api/v1`) |
| Scope (key) | Client IP |
| Kind | throttle |
| Default limit | 300 requests per 1 minute per IP |

Keyed **per client IP** (changed from per-short-code in ADR-011): an abusive client hammering many codes is
throttled, while a legitimately popular link is not globally capped. Because it is IP-scoped, accuracy behind
a reverse proxy depends on `ratelimit.trust_proxy_headers` (ADR-009). Exceeding the limit returns `429 Too
Many Requests` with a `Retry-After` header.

### Endpoints without rate limiting

List (`GET /api/v1/shortener/links`), get (`GET /api/v1/shortener/links/{code}`), update (`PATCH /api/v1/shortener/links/{code}`), and delete (`DELETE /api/v1/shortener/links/{code}`) are authenticated endpoints with no additional rate limiting beyond JWT validation.

## Config Reference

Rate limits are **not** configured here — manage them via the ratelimit admin API/UI (see
`docs/adr/RATE_LIMITING_ADR.md`).

```yaml
shortener:
  code_length: 7                  # Length of generated short codes (base62)
  max_create_retries: 5           # Retries on UNIQUE collision before returning 500
  url:
    max_length: 2048              # Max target URL length in characters
    allowed_schemes: ["http", "https"]
    blocked_hosts: []             # Hostnames to reject (e.g. internal services)
```

## Security Notes

- Short codes are generated using `crypto/rand` (7 base62 chars, ~3.5T keyspace).
- URL validation rejects non-http(s) schemes and hostnames resolving to private/loopback ranges (SSRF protection).
- The redirect response uses `302` (not `301`) with `Cache-Control: private, no-store` so disabled or expired links are not cached by intermediaries.
- Ownership checks use a JOIN query rather than a two-step get-then-check to avoid TOCTOU races.
- Non-existent and unauthorized link lookups both return `404` to avoid existence leaks.
