# URL Shortener

## Overview

The shortener feature lets authenticated users create short links (`/r/{code}`) and manage them (list, update, delete). The redirect endpoint (`GET /r/{code}`) is public — no login required to follow a link.

## Rate Limiting

Two independent rate limiters protect the shortener. Both use fixed-window in-memory buckets and are not shared across restarts or multiple instances (single-instance deployment only).

### Create limiter

| Property | Value |
|---|---|
| Endpoint | `POST /api/v1/shortener/links` |
| Key | User ID (authenticated) or client IP (anonymous) |
| Default limit | 100 creates per 15-minute window |
| Config | `shortener.rate_limit.creates_per_window`, `shortener.rate_limit.window_minutes` |

Each user (or IP for anonymous requests) gets its own independent bucket. Exceeding the limit returns `429 Too Many Requests` with a `Retry-After` header.

### Resolve limiter

| Property | Value |
|---|---|
| Endpoint | `GET /r/{code}` |
| Key | Short code (e.g. `resolve:abc1234`) |
| Default limit | 300 requests per 1-minute window per code |
| Config | `shortener.rate_limit.resolve_per_window`, `shortener.rate_limit.resolve_window_minutes` |

Each short link gets its own bucket. A flood targeting one link does not affect the redirect performance of other links. Exceeding the limit returns `429 Too Many Requests` with a `Retry-After` header.

### Endpoints without rate limiting

List (`GET /api/v1/shortener/links`), get (`GET /api/v1/shortener/links/{code}`), update (`PATCH /api/v1/shortener/links/{code}`), and delete (`DELETE /api/v1/shortener/links/{code}`) are authenticated endpoints with no additional rate limiting beyond JWT validation.

## Config Reference

```yaml
shortener:
  code_length: 7                  # Length of generated short codes (base62)
  max_create_retries: 5           # Retries on UNIQUE collision before returning 500
  public_create_enabled: false    # Allow unauthenticated link creation (not yet wired)
  rate_limit:
    creates_per_window: 100       # Max authenticated creates per window per user
    window_minutes: 15            # Window size for create rate limit
    public_creates_per_window: 20 # Max anonymous creates per window (when public_create_enabled=true)
    resolve_per_window: 300       # Max redirects per short code per window
    resolve_window_minutes: 1     # Window size for resolve rate limit
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
