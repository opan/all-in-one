# Shortener — Architecture Decision Record

**Feature:** URL Shortener (M1)  
**Shipped:** 2026-05-10 → 2026-05-11  
**Status:** Production-ready (M1 complete)

---

## Context

All-in-one needed a URL shortener as a first-class feature-app alongside Listing and Chat. The primary driver beyond the immediate use case was forward-compatibility: a future standalone shortener web app must be able to call the same AIO backend over the same REST endpoints, reusing generated TypeScript types. That constraint shaped every decision below.

---

## Decisions

### 1. Protobuf as contract source of truth (REST/JSON transport)

**Decision:** Define the full API surface in `proto/shortener/v1/shortener.proto`. Use `buf generate` to emit Go structs and TypeScript types. Transport is REST/JSON — not gRPC.

**Alternatives considered:**
- Hand-written Go structs + hand-written TS interfaces: works, but diverges silently over time.
- Full gRPC: strong typing everywhere, but requires a gRPC-web proxy layer for browser clients and a separate gRPC server to operate in production.

**Why this:** Protobuf gives a single source of truth for both sides without the operational overhead of gRPC. The generated TS types (`web/src/lib/pb/`) can be imported directly by the future standalone shortener web app, eliminating manual type duplication. The `buf` toolchain (`buf.yaml`, `buf.gen.yaml`) makes codegen reproducible with `make proto`. Generated files are committed so CI has no `buf` dependency at build time.

---

### 2. Short-code format: 7-char base62 via crypto/rand

**Decision:** Generate codes using `crypto/rand` over the base62 alphabet (`[0-9A-Za-z]`). Fixed length of 7 characters (~3.5 trillion unique codes). Collision retry up to 5× on `UNIQUE` constraint violation, then 500.

**Alternatives considered:**
- Sequential integer + base62 encode: predictable, enumerable — bad for privacy.
- UUID fragment (first 7 hex chars): base16 only (~268M keyspace), shorter but more collision-prone, less URL-aesthetic.
- NanoID / ULIDs as codes: ULID is good for IDs but 26 chars is long for a short URL code.

**Why this:** `crypto/rand` is non-enumerable and the 3.5T keyspace is ample for M1. Collision surface at 5× retry is negligible. The UNIQUE constraint in SQLite surfaces constraint violations as a concrete error type (`sqlite3.ErrConstraintUnique`), making retry logic explicit and testable.

A reserved-code list (`api`, `r`, `admin`, `static`, `login`, `health`, etc.) ensures auto-generated codes never shadow application routes.

---

### 3. ULID as row identifier

**Decision:** Use ULID (26-char Crockford base32) as the primary key for `short_links`, stored as `TEXT` in both SQLite and PostgreSQL.

**Alternatives considered:**
- Auto-increment INTEGER: simple, but leaks row counts and doesn't port to distributed writes.
- UUID v4: non-sequential, requires `uuid` extension in PostgreSQL for native storage, diverges between SQLite (`TEXT`) and PostgreSQL (`UUID`).
- UUID v7 (time-ordered): good alternative, but ULID tooling (`github.com/oklog/ulid/v2`) was already vendored for another feature.

**Why this:** ULIDs are lexicographically sortable in the same order as insertion time, so "list newest first" can `ORDER BY id DESC` and benefit from the primary-key index without a separate `created_at` index. The `TEXT` type works identically in both SQLite and PostgreSQL, avoiding the `INTEGER` vs `UUID` divergence that `users.id` has.

Note: `users.id` is `TEXT` (UUID) for historical reasons; `short_links.owner_id` is also `TEXT` referencing it. No migration of the users table was required.

---

### 4. Auth model: JWT required for management, public resolve

**Decision:** `POST/GET/PATCH/DELETE /api/v1/shortener/links*` require a valid JWT. `GET /r/{code}` is public with no auth.

**Alternatives considered:**
- API keys for management: more fine-grained, but adds a key-management layer that's out of scope for M1.
- Public create with a CAPTCHA: deferred to the `public_create_enabled` config path.

**Why this:** Consistent with the rest of AIO (Listing, Chat all require JWT for writes). The `public_create_enabled` config flag is wired now (`default: false`) so enabling anonymous creates in the standalone app requires only a config flip — no code change.

**Ownership mismatch returns 404, not 403.** This prevents existence leaks: a user probing for codes belonging to others sees the same response as for nonexistent codes.

---

### 5. URL safety: scheme allowlist + private-IP rejection

**Decision:** Validate target URLs in `internal/shortener/handler/validate.go`:
1. Max length 2048.
2. Must be absolute URL parseable by `net/url`.
3. Scheme allowlist: `http`, `https` only — reject `javascript:`, `data:`, `file:`, `ftp:`.
4. Reject hostnames resolving to private/loopback ranges (RFC1918, `127/8`, `::1`, `169.254/16`, `0.0.0.0`).
5. Optional config-driven host blocklist.

**Order matters:** Scheme check runs before host-empty check. A URL like `javascript:alert(1)` has no host; checking host-empty first would return the wrong error message and could mask future scheme-based bypass attempts.

**Why private-IP rejection:** A shortener without this check becomes an SSRF stepping-stone — an attacker creates `http://10.0.0.1/metadata` and uses the public redirect to reach internal infrastructure from the server's network context.

---

### 6. Rate limiting: fixed-window in-memory

**Decision:** Fixed-window rate limiter in `internal/shortener/middleware/ratelimit.go`. Default: 100 creates per 15-minute window per `(owner_id || remote IP)`. Applied only to the create endpoint via `h.rateLimiter.Wrap(h.CreateShortLink)`.

**Alternatives considered:**
- Token bucket / sliding window: smoother, but more complex to implement correctly in-process.
- Redis-backed rate limiter: consistent across multiple instances, but AIO is single-instance; adds an external dependency.

**Why this:** AIO runs as a single process. A fixed-window in-memory limiter is correct, testable, and zero-dependency. A cleanup goroutine removes expired buckets every window duration to bound memory. The limiter key for authenticated requests is `user:<userID>`; for unauthenticated (future public-create path) it falls back to `ip:<remoteAddr>`.

**Bug noted and fixed:** Bucket was initialized with `count: 1`, so requests with `limit=0` (block all) were allowed once before blocking. Fixed to initialize with `count: 0`.

---

### 7. Redirect semantics: 302 + Cache-Control: private, no-store

**Decision:** `GET /r/{code}` returns `302 Found` with `Cache-Control: private, no-store`.

**Alternatives:**
- `301 Moved Permanently`: browser and CDN caches the redirect permanently. If a link is later disabled or expired, users with a cached 301 will continue being redirected indefinitely.

**Why this:** 302 is re-validated on every request, and `private, no-store` prevents any intermediary (CDN, ISP proxy) from caching the redirect. This is essential for the disable and expiration features to work reliably.

---

### 8. Atomic click-count update

**Decision:** A single `UPDATE` with the active/expiry guard in the `WHERE` clause:

```sql
UPDATE short_links
SET click_count = click_count + 1, last_accessed_at = ?
WHERE code = ?
  AND is_active = 1
  AND (expires_at IS NULL OR expires_at > ?)
```

`RowsAffected == 0` is treated as "not found or inactive/expired" and returns `ErrNotFound`.

**Why this:** A `SELECT` then `UPDATE` pattern introduces a TOCTOU race where status could change between the two statements. The single-statement approach is atomic at the SQL level. For a URL shortener the failure mode is acceptable — if `IncrementClick` fails (e.g., link disabled between `GetByCode` and `IncrementClick`), the redirect has already happened and the click is silently lost. This is logged at `WARN` level and does not fail the redirect.

---

### 9. Package layout

```
internal/shortener/
  handler/       HTTP handlers + URL validation + code generation + tests
  middleware/    Rate limiter
  model/         Domain struct (ShortLink)
  repository/    Interface + factory
  repository/sqlite/  SQLite implementation
  codec/         proto ↔ model conversion helpers
  service/       Thin wiring layer (delegates to handler)
  pb/v1/         Generated Go proto structs (committed)
```

**Why `handler/` holds code generation:** A circular import would result if `service/` held the ULID/base62 generators while also being imported by `handler/`. Moving generation helpers (`newULID`, `newShortCode`, `isReserved`, `isUniqueConstraintError`) into `handler/` resolves the cycle cleanly.

**`codec/`** exists to keep the proto-to-model mapping out of the handler. This matters for the future standalone app: the codec can be imported by any layer that needs to serialize to/from the wire format without pulling in HTTP handler logic.

---

### 10. Frontend: SvelteKit + typed client from generated TS

**Decision:** `web/src/lib/shortener-api.ts` is hand-written (not generated) but its types mirror the generated `shortener_pb.ts`. The `+page.ts` load function uses `apiLoad` (SSR-compatible fetch wrapper) to populate the initial list server-side.

**Why hand-written client:** The generated `@bufbuild/protobuf` runtime uses protobuf binary encoding. The REST transport uses JSON. Writing a thin typed client by hand against the JSON shape is less coupling than pulling in a protobuf-JSON bridge library for the frontend.

The `shortURL(code)` helper detects `typeof window === 'undefined'` to return a relative path during SSR, falling back to `window.location.origin` in the browser — necessary for SvelteKit's universal load functions.

---

## What was deferred (M2)

- **Custom slugs:** `optional string custom_code = 3` is reserved in the proto. The server returns `400 Bad Request` if the field is set in M1. M2 implementation: validate `^[a-zA-Z0-9_-]{3,32}$`, reserved-prefix check, uniqueness check, premium-tier gate.
- **Full click analytics:** Only `click_count` and `last_accessed_at` are stored for M1. A full analytics pipeline (referrer, country, device) is deferred to a later milestone via an OSS GA-alternative.
- **Public create:** Config flag is wired (`shortener.public_create_enabled: false`). Enabling it mounts the create handler on the public router with the stricter rate-limit bucket. No code change required.
- **PostgreSQL migration:** Migration 05 is SQLite-only for now. The `owner_id TEXT` column type is the same in both backends so the migration will port without schema changes. The partial index `WHERE expires_at IS NOT NULL` is supported in PostgreSQL.

---

## Test coverage

| Layer | Test file | Approach |
|-------|-----------|----------|
| URL validation | `handler/validate_test.go` | 15 table-driven cases |
| Code generation | `handler/code_test.go` | length, alphabet, uniqueness (100 samples), ULID format, reserved codes |
| Handlers (unit) | `handler/shortlink_test.go` | 19 tests using mockery-generated mock repo |
| Rate limiter | `middleware/ratelimit_test.go` | 7 tests: allow/block/reset/key-isolation/Wrap |
| Repository | `repository/sqlite/shortlink_repository_test.go` | 17 integration tests against `:memory:` SQLite |
| Handler (integration) | `handler/integration_test.go` | 10 end-to-end tests: create→resolve, click count, disable, expire, delete, ownership |
