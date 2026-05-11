# Shortener feature-app — progress tracker

Started: 2026-05-10
Status: **Complete — all phases P0–P6 done**

## Decisions locked in

- **Contract**: protobuf as source of truth, REST/JSON transport (no gRPC).
- **Code generation**: `crypto/rand` → 7-char base62 + UNIQUE-collision retry (max 5).
- **Auth model (v1)**: JWT required for create/list/update/delete; `GET /r/{code}` is public.
- **v1 features**: expiration + disable flag, per-user rate limit on create, URL safety check (scheme allowlist + private-IP rejection).
- **Identifier**: ULID (26-char Crockford base32) stored as `TEXT` in SQLite and `TEXT` in PostgreSQL — same column type both backends, no native-UUID divergence to manage. Bonus: lexicographic order = chronological order, so "list newest first" can index on `id` alone.
- **Sidebar**: shortener entry sits alongside Listing and Chat (no submenu).
- **Public-create flag**: `shortener.public_create_enabled` config (default `false`). When flipped on later for the standalone app, the create handler also mounts on the public router behind a stricter rate limit. No code change needed at flip time.
- **Deferred**: click analytics (just count + last_accessed for v1; full analytics later via OSS GA-alternative).
- **Why this shape**: a future standalone shortener web app will call the same AIO backend over the same REST/JSON endpoints, reusing the generated TS types. Single source of truth, no separate gRPC server to operate.

## Milestones

- **M1 (this tracker)**: auto-generated 7-char base62 codes only. Auth required to create; `/r/{code}` public.
- **M2 — custom slugs ("premium")**: the `code` field on `CreateShortLinkRequest` becomes user-settable. Validation: `^[a-zA-Z0-9_-]{3,32}$`, reserved-prefix check, uniqueness check, premium-tier gate. Contract is reserved now (see proto sketch) so the wire format won't break when M2 lands.

## Architecture

```
proto/shortener/v1/shortener.proto    ← contract (source of truth)
        │
        ├─ buf generate ──→ internal/shortener/pb/v1/*.pb.go   (Go structs)
        └─ buf generate ──→ web/src/lib/pb/shortener/v1/*.ts   (TS types)

internal/shortener/
  ├─ model/              domain entities + URL validation
  ├─ repository/         interface + sqlite + factory (matches listing/)
  ├─ service/            business logic, code generation, rate limiter
  ├─ handler/            REST handlers; map proto ↔ JSON via protojson
  ├─ codec/              protojson helpers
  └─ middleware/         per-app rate limit (token bucket)

cmd/all-in-one/server/server.go       ← register routes
db/migrations/sqlite3/05_create_short_links.{up,down}.sql
web/src/routes/shortener/             ← Svelte UI
web/src/lib/shortener-api.ts          ← typed client using generated TS
```

## Proto contract (sketch)

`proto/shortener/v1/shortener.proto`:
```proto
syntax = "proto3";
package shortener.v1;

import "google/protobuf/timestamp.proto";

message ShortLink {
  string id = 1;                              // ULID
  string code = 2;                            // base62, 7 chars
  string target_url = 3;
  optional int64 owner_id = 4;
  google.protobuf.Timestamp created_at = 5;
  optional google.protobuf.Timestamp expires_at = 6;
  bool is_active = 7;
  uint64 click_count = 8;
  optional google.protobuf.Timestamp last_accessed_at = 9;
}

message CreateShortLinkRequest {
  string target_url = 1;
  optional google.protobuf.Timestamp expires_at = 2;
  // Reserved for M2 custom-slug feature. Server returns 400 if set in M1.
  optional string custom_code = 3;
}
message CreateShortLinkResponse { ShortLink link = 1; }

message ListShortLinksRequest  { uint32 page = 1; uint32 page_size = 2; }
message ListShortLinksResponse { repeated ShortLink links = 1; uint32 total = 2; }

message UpdateShortLinkRequest {
  string code = 1;
  optional bool is_active = 2;
  optional google.protobuf.Timestamp expires_at = 3;
}
message UpdateShortLinkResponse { ShortLink link = 1; }

message DeleteShortLinkRequest  { string code = 1; }
message DeleteShortLinkResponse {}
```

Tooling: `buf` (`buf.yaml`, `buf.gen.yaml`), with plugins:
- Go: `buf.build/protocolbuffers/go`
- TS: `buf.build/bufbuild/es`

`Makefile` target `make proto` runs `buf generate`. Generated files are committed.

## REST surface

| Method | Path                                | Auth   | Notes |
|--------|-------------------------------------|--------|-------|
| POST   | `/api/v1/shortener/links`           | JWT    | Body = `CreateShortLinkRequest` (protojson). 201 on success. |
| GET    | `/api/v1/shortener/links`           | JWT    | Owner-scoped, paginated. |
| GET    | `/api/v1/shortener/links/{code}`    | JWT    | Owner-only. |
| PATCH  | `/api/v1/shortener/links/{code}`    | JWT    | Toggle active / change expiry. |
| DELETE | `/api/v1/shortener/links/{code}`    | JWT    | Owner-only. |
| GET    | `/r/{code}`                         | public | 302 redirect; bumps `click_count`, sets `last_accessed_at`. 404 missing/disabled, 410 expired. |

Resolve path lives outside `/api/v1` so the public URL stays short.

## Database — migration 05

```sql
-- 05_create_short_links.up.sql
CREATE TABLE short_links (
  id                TEXT PRIMARY KEY,
  code              TEXT NOT NULL UNIQUE,
  target_url        TEXT NOT NULL,
  owner_id          INTEGER REFERENCES users(id) ON DELETE SET NULL,
  created_at        DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  expires_at        DATETIME,
  is_active         BOOLEAN NOT NULL DEFAULT 1,
  click_count       INTEGER NOT NULL DEFAULT 0,
  last_accessed_at  DATETIME
);
CREATE INDEX idx_short_links_owner_id  ON short_links(owner_id);
CREATE INDEX idx_short_links_expires   ON short_links(expires_at) WHERE expires_at IS NOT NULL;
```

`UNIQUE(code)` makes collision retry surface as constraint violation → service retries up to 5x.

## Security considerations

1. **URL safety check** (`model/validate.go`):
   - Parse with `net/url`; require absolute URL.
   - Scheme allowlist: `http`, `https` only. Reject `javascript:`, `data:`, `file:`, `ftp:`.
   - Reject hostnames resolving to private/loopback ranges (RFC1918, `127/8`, `::1`, `169.254/16`, `0.0.0.0`) — blocks SSRF stepping-stone abuse.
   - Optional host blocklist (config-driven, empty by default).
   - Max length 2048.
2. **Short-code generation** (`service/code.go`):
   - `crypto/rand` → 7 base62 chars (~3.5T keyspace).
   - Reserved-prefix list (`api`, `r`, `admin`, etc.) — codes never collide with routes.
   - Retry up to 5x on UNIQUE collision; then 500.
3. **Open-redirect hardening**: 302 (not 301), `Cache-Control: private, no-store`, so disabled/expired links aren't cached by intermediaries.
4. **Rate limiting** (`internal/shortener/middleware/ratelimit.go`):
   - Token bucket, in-memory (single-instance AIO is fine).
   - 100 creates / 15 min per `(owner_id || ip)`.
   - 429 + `Retry-After`.
   - Applied only to create/update — resolve has a much higher cap (e.g. 600/min/IP).
5. **Authorization**: management handlers assert `link.owner_id == ctxUserID`; mismatch returns 404 (not 403) to avoid existence leak.
6. **Click-count update**: single atomic SQL —
   `UPDATE short_links SET click_count=click_count+1, last_accessed_at=? WHERE code=? AND is_active=1 AND (expires_at IS NULL OR expires_at > ?)`.
   RowsAffected==0 → 404/410. No select-then-update race.
7. **CORS**: extend `server.go` CORS config with a `cors.allowed_origins` list in `config.yml`; empty default = same-origin only. Standalone app's domain added later.
8. **Logging**: zerolog fields `module=shortener`, `code`, `owner_id`, `request_id`. Host only at info level; full URL at debug.
9. **Future**: introduce `X-Client` header (`aio-web`, `shortener-web`) for analytics segmentation and a future API-key path distinct from JWT.

## Config additions

```yaml
shortener:
  code_length: 7
  max_create_retries: 5
  public_create_enabled: false      # flip true when standalone app goes live
  rate_limit:
    creates_per_window: 100         # authenticated bucket
    window_minutes: 15
    public_creates_per_window: 20   # stricter bucket when public_create_enabled=true
  url:
    max_length: 2048
    allowed_schemes: ["http", "https"]
    blocked_hosts: []
  redirect_path: "/r"
```

## Phases

| Phase | Scope | Depends on | Status |
|-------|-------|------------|--------|
| **P0 — Proto & codegen** | `buf.yaml`, `buf.gen.yaml`, first `.proto`, `make proto`, `.gitignore`, commit generated stubs. | — | ✓ |
| **P1 — Backend skeleton** | `internal/shortener/{model,repository,service,handler,codec}` packages compiling; migration 05; routes wired in `server.go` (stubs returning 501). | P0 | ✓ |
| **P2 — Create + Resolve** | `POST /links` (auth) + `GET /r/{code}` (public). URL validator + base62 generator + collision retry. Atomic click-count update. | P1 | ✓ |
| **P3 — List/Get/Update/Delete** | Owner-scoped management endpoints; pagination. | P2 | ✓ |
| **P4 — Security hardening** | Rate-limit middleware; reserved-code list; private-IP rejection; CORS config. | P3 | ✓ |
| **P5 — Frontend** | `web/src/routes/shortener/` (list + create + edit), typed client `shortener-api.ts` using generated TS, sidebar entry. | P3 | ✓ |
| **P6 — Tests** | Table-driven service tests with mockery; repo tests on in-memory sqlite; handler integration test for redirect path. | alongside P2–P5 | ✓ |

## Resolved questions (2026-05-10)

1. **Custom slugs** → M2 ("premium" feature). M1 contract reserves `custom_code` field number; server rejects with 400 if set.
2. **ID type** → ULID, stored as `TEXT` in both SQLite and PostgreSQL. `users.id` stays `INTEGER` (different table, no need to migrate).
3. **Sidebar** → alongside Listing/Chat, no submenu.
4. **Public-create flag** → wired now as `shortener.public_create_enabled`, default `false`. Stricter rate-limit bucket applies when enabled.

## Notes for future sessions

- This file is the resume point. Update phase statuses as work progresses.
- ADR written: `.github/context/SHORTENER_IMPLEMENTATION_PLAN.md`.
- Generated proto files will be committed; do not regenerate without `make proto`.
