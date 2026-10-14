# All-in-One

A multi-functional full-stack web application built for learning purposes.

- **Backend:** Go 1.26+, REST API
- **Frontend:** Svelte 5+, TypeScript 5.9+, Vite 7+

## Features

- **Listing** — Create topics with custom JSON schemas, manage items within topics
- **Chat** — Real-time chat with WebSocket support, invite system
- **Authentication** — JWT-based auth (username/password) with session management
- **Two-Factor Authentication (2FA)** — Opt-in TOTP-based 2FA (Google Authenticator, Authy, etc.)
- **Login for other apps (OpenID Connect)** — Other first-party apps (e.g. cashflow) log users in with their aio account; off by default, see [below](#login-for-other-apps-openid-connect)
- **Observability** — OpenTelemetry traces (Jaeger) and metrics (Prometheus) via OTel Collector

## Project Structure

```
all-in-one/
├── cmd/all-in-one/         # CLI entrypoints (server, migrate, seed)
├── config/config.yml       # Default configuration
├── db/migrations/          # SQL migration files
├── internal/
│   ├── auth/               # Password hashing, TOTP crypto utilities
│   ├── authnz/             # Authentication & authorization (handlers, middleware, repository)
│   ├── chat/               # Chat app (WebSocket, sessions, invites)
│   ├── listing/            # Listing app (topics, items, JSON schema forms)
│   ├── oidc/               # OpenID Connect provider (login for other apps)
│   ├── config/             # Configuration loading (Viper)
│   ├── http/               # Shared HTTP helpers and middleware
│   └── storage/            # Database connection and migration runner
└── web/                    # Frontend (SvelteKit, Tailwind, shadcn-svelte)
```

## Prerequisites

- Go 1.26+
- Node.js 22.19+, npm 10+
- SQLite (via `go-sqlite3`, requires CGO — `gcc` must be installed)
- Docker (optional — for the local observability stack)

## Quick Start

### 1. Backend

```bash
# Install Go dependencies
go mod tidy

# Run database migrations
go run ./cmd/all-in-one db:migrate up

# (Optional) Seed with sample data
go run ./cmd/all-in-one db:seed

# Start the server
go run ./cmd/all-in-one server
```

The API is available at `http://localhost:8080`.

### 2. Frontend

```bash
cd web
npm install
npm run dev
```

The frontend dev server runs at `http://localhost:5173` and proxies API requests to the backend automatically.

## CLI Commands

```bash
go run ./cmd/all-in-one server                                        # Start the HTTP server
go run ./cmd/all-in-one db:migrate up                                 # Apply all pending migrations
go run ./cmd/all-in-one db:migrate down                               # Roll back migrations
go run ./cmd/all-in-one db:migrate down --steps 1                     # Roll back one migration
go run ./cmd/all-in-one db:seed                                       # Seed sample users, topics, and chat data
go run ./cmd/all-in-one db:transfer --direction sqlite-to-pg --confirm  # Copy data from SQLite → PostgreSQL
go run ./cmd/all-in-one db:transfer --direction pg-to-sqlite --confirm  # Copy data from PostgreSQL → SQLite
go run ./cmd/all-in-one oidc:client:create --id <id> --name <name> --redirect-uri <url>  # Register an app for OIDC login (prints its secret once)
go run ./cmd/all-in-one oidc:client:list                             # List registered apps
go run ./cmd/all-in-one oidc:client:update <id> --brand-color '#0f766e' --icon 💰  # Change an app's name or branding
go run ./cmd/all-in-one oidc:client:revoke <id>                      # Stop an app from logging users in
go run ./cmd/all-in-one users:import [--apply]                       # Move cashflow's existing accounts into aio (dry run without --apply)
```

> **`db:transfer` prerequisites:**
> - Both databases must have all schema migrations applied before running.
> - The destination must be empty — existing rows cause constraint failures.
> - `--confirm` is required for both directions.
> - Both `storage.sqlite` and `storage.postgres` must be configured regardless of direction (the command opens both connections). The SQLite path defaults to `all-in-one.db`. Set PostgreSQL credentials via config or env vars:
> ```bash
> ALLINONE_STORAGE_POSTGRES_HOST=localhost \
> ALLINONE_STORAGE_POSTGRES_USER=allinone \
> ALLINONE_STORAGE_POSTGRES_PASSWORD=allinone \
> ALLINONE_STORAGE_POSTGRES_DBNAME=allinone \
> go run ./cmd/all-in-one db:transfer --direction sqlite-to-pg --confirm
> ```

## Configuration

Configuration is loaded from `config/config.yml`. All values can be overridden via environment variables using the prefix `ALLINONE_` and replacing `.` with `_` (e.g., `auth.jwt_secret` → `ALLINONE_AUTH_JWT_SECRET`).

### Full config reference

```yaml
server:
  port: 8080
  swagger_enabled: false          # Set to true locally to enable /swagger/index.html
  allowed_origins: []             # CORS allowed origins. Defaults to ["*"] when empty

storage:
  type: "sqlite"          # "sqlite" or "postgres" (postgres is stubbed)
  sqlite:
    db_path: "all-in-one.db"

log:
  level: "debug"          # "debug", "info", "warn", "error"

http:
  timeout: 30             # Request timeout in seconds

auth:
  jwt_secret: ""                  # REQUIRED — secret for signing JWT tokens
  totp_encryption_key: ""         # REQUIRED — 64-char hex string (32 bytes) for encrypting 2FA secrets
  secure_cookie: false            # Set to true in production (requires HTTPS)
  direct_auth_enabled: false      # Dev only — bypass auth via x-direct-auth-username header
  oidc:
    enabled: false                # Serve aio as an OpenID Connect provider for other apps (single replica only)
    issuer: ""                    # aio's public base URL, e.g. https://auth.example.com (required when enabled)
    auth_request_ttl: 10m         # How long a user has to finish aio's login page after an app sends them
    access_token_lifetime: 15m    # Access tokens (userinfo endpoint)
    id_token_lifetime: 5m         # ID tokens (apps read them once, at login)

shortener:
  code_length: 7                  # Length of generated short codes
  max_create_retries: 5           # Retries on UNIQUE collision before giving up
  # Rate limits for link creation/resolution are managed by the ratelimit
  # app-feature (admin API / DB), not here — see ratelimit: below.
  url:
    max_length: 8192              # Max target URL length
    allowed_schemes: ["http", "https"]
    blocked_hosts: []             # Hostnames to reject (e.g. internal services)

ratelimit:
  enabled: true                # master kill switch; per-target limits/toggles live in the DB (admin-editable)
  cache_refresh_interval: 30s  # backstop refresh of the in-memory rule cache (writes also reload immediately)
  cleanup_interval: 1h         # how often expired daily-quota counter rows are pruned
  counter_retention_days: 3    # how many days of daily-quota counter rows to keep
  timezone: "UTC"              # timezone daily-quota calendar-day buckets are computed in
  trust_proxy_headers: false   # set true only behind a sanitizing reverse proxy that sets X-Forwarded-For

telemetry:
  enabled: false                  # Toggle OTel on/off without rebuilding
  service_name: "all-in-one"
  service_version: "1.0.0"
  environment: "local"            # local | staging | production
  otlp_endpoint: "localhost:4318" # host:port (no scheme) of OTLP/HTTP receiver
  otlp_insecure: true             # Set false in production with TLS
  sample_ratio: 1.0               # 0.0–1.0. Use 0.1 in production.
  metric_interval: 15s            # How often metrics are pushed to the collector
```

### Environment variable reference

| Variable | Description |
|---|---|
| `ALLINONE_AUTH_JWT_SECRET` | JWT signing secret |
| `ALLINONE_AUTH_TOTP_ENCRYPTION_KEY` | 32-byte hex key for 2FA secret encryption |
| `ALLINONE_AUTH_SECURE_COOKIE` | `true` in production with HTTPS |
| `ALLINONE_AUTH_OIDC_ENABLED` | `true` to serve aio as an OpenID Connect provider (default: `false`) |
| `ALLINONE_AUTH_OIDC_ISSUER` | aio's public base URL, required when OIDC is enabled |
| `ALLINONE_SERVER_PORT` | HTTP server port (default: `8080`) |
| `ALLINONE_SERVER_SWAGGER_ENABLED` | `true` to enable Swagger UI (default: `false`) |
| `ALLINONE_STORAGE_TYPE` | `sqlite` (default) |
| `ALLINONE_STORAGE_SQLITE_DB_PATH` | Path to SQLite file (default: `all-in-one.db`) |
| `ALLINONE_LOG_LEVEL` | Log level (default: `debug`) |
| `ALLINONE_TELEMETRY_ENABLED` | `true` to enable OTel traces + metrics (default: `false`) |
| `ALLINONE_TELEMETRY_OTLP_ENDPOINT` | OTLP/HTTP receiver `host:port` (default: `localhost:4318`) |
| `ALLINONE_TELEMETRY_ENVIRONMENT` | Deployment environment tag (default: `local`) |
| `ALLINONE_TELEMETRY_SAMPLE_RATIO` | Trace sampling ratio `0.0–1.0` (default: `1.0`) |
| `ALLINONE_TELEMETRY_METRIC_INTERVAL` | Metrics push interval (default: `15s`) |

### Generating secrets

Both `jwt_secret` and `totp_encryption_key` must be set before the server starts. Generate them with:

```bash
# JWT secret (any strong random string works)
openssl rand -hex 32

# TOTP encryption key (must be exactly 64 hex chars / 32 bytes)
openssl rand -hex 32
```

Use different values for each. For production, set them as environment variables rather than committing them to `config.yml`:

```bash
export ALLINONE_AUTH_JWT_SECRET="$(openssl rand -hex 32)"
export ALLINONE_AUTH_TOTP_ENCRYPTION_KEY="$(openssl rand -hex 32)"
```

### Kubernetes Secrets

When deploying to Kubernetes, store secrets in a `Secret` resource. Use `--from-literal` so `kubectl` handles base64 encoding correctly (avoids trailing newline issues).

```bash
# fish shell
set JWT (openssl rand -hex 32)
set TOTP (openssl rand -hex 32)

kubectl create secret generic all-in-one-secrets \
  -n app \
  --from-literal=jwt-secret="$JWT" \
  --from-literal=totp_encryption_secret="$TOTP" \
  --dry-run=client -o yaml | kubectl apply -f -
```

```bash
# bash/zsh
kubectl create secret generic all-in-one-secrets \
  -n app \
  --from-literal=jwt-secret="$(openssl rand -hex 32)" \
  --from-literal=totp_encryption_secret="$(openssl rand -hex 32)" \
  --dry-run=client -o yaml | kubectl apply -f -
```

Using `--dry-run=client -o yaml | kubectl apply -f -` patches the existing secret in-place without deleting it. To update a single key without touching others, fetch the existing value first:

```fish
# fish — update only totp_encryption_secret, preserve jwt-secret
set JWT (kubectl get secret all-in-one-secrets -n app -o jsonpath='{.data.jwt-secret}' | base64 -d)
set TOTP (openssl rand -hex 32)

kubectl create secret generic all-in-one-secrets \
  -n app \
  --from-literal=jwt-secret="$JWT" \
  --from-literal=totp_encryption_secret="$TOTP" \
  --dry-run=client -o yaml | kubectl apply -f -
```

Verify the value is exactly 64 characters:

```bash
kubectl get secret all-in-one-secrets -n app \
  -o jsonpath='{.data.totp_encryption_secret}' | base64 -d | wc -c
# Must print 64
```

## Login for other apps (OpenID Connect)

aio can act as an OpenID Connect provider, so other first-party apps log users in with their aio account instead of keeping their own passwords. cashflow is the first app to use it. It is **off by default**.

```
App "Log in" → aio /api/v1/oauth2/authorize → aio's login or signup page (shown as the app)
           ← app /callback?code=…          ← aio (or straight back if already logged in to aio)
App exchanges the code for an ID token, verifies it, and starts its own session.
```

Design decisions are in [docs/adr/OIDC_PROVIDER_ADR.md](docs/adr/OIDC_PROVIDER_ADR.md) and the background in [RFC-001](docs/rfc/RFC-001-central-user-management.md). cashflow's side (its config and rollout) is in [cashflow's README](https://github.com/opan/cashflow/blob/main/README.en.md#logging-in-through-all-in-one-optional).

### Turning it on

```yaml
auth:
  oidc:
    enabled: true
    issuer: "https://aio.example.com"
```

- `issuer` is aio's public base URL, the one both browsers and apps reach. It must be absolute and without a path. Use `http://` only for local development.
- On start, aio runs its migrations (the `oidc_clients` table) and creates an RSA signing key in the database. The private key is encrypted with `auth.totp_encryption_key`: if that key changes, aio refuses to start.
- Check it with `curl https://aio.example.com/.well-known/openid-configuration`.

| Endpoint | Path |
|---|---|
| Discovery | `/.well-known/openid-configuration` |
| Authorize, token, userinfo, keys (JWKS), end_session, revoke | `/api/v1/oauth2/…` |
| Login hand-off used by aio's pages | `/api/v1/oidc/auth-requests/{id}` |
| Registered apps (admin only) | `/api/v1/oidc/clients` (see Swagger) |

### Registering an app

```bash
go run ./cmd/all-in-one oidc:client:create \
  --id cashflow --name Cashflow \
  --redirect-uri https://cashflow.example.com/auth/callback \
  --post-logout-redirect-uri https://cashflow.example.com/ \
  --brand-color '#0f766e' --icon 💰
```

The client secret is printed **once**; aio stores only its hash. Redirect URIs must be `https://`, except `http://` on `localhost` or a loopback IP, for development. `oidc:client:list`, `oidc:client:update` and `oidc:client:revoke` manage apps afterwards, as do the admin endpoints under `/api/v1/oidc/clients`.

In Kubernetes the image is distroless, but the binary can be run directly:

```bash
kubectl -n app exec deploy/all-in-one -- /app/all-in-one oidc:client:create --id cashflow ...
```

### What an app gets

- Authorization code flow only, with **PKCE (S256) required** and the client secret sent as HTTP Basic auth (`client_secret_basic`). No refresh tokens: apps start their own session after login.
- Scopes and claims: `openid` → `sub` (aio's user id), `profile` → `preferred_username` and `name`, `email` → `email` (when set, `email_verified: false`).
- `ui_locales` picks the language of aio's pages (`en` or `id`). `prompt=create` opens signup instead of login. `prompt=login` and `max_age` are refused.
- Logout: send the user to `end_session` with the ID token as `id_token_hint`. aio also ends its own session when the hint belongs to the user logged in to aio.
- The shared demo account (`demo_mode`) can't log in to other apps.

When an app sends users to aio, the login and signup pages show the app's name, icon, colour and language, with a small "Login secured by All-in-one" note. aio's own login page is unchanged.

### Moving an app's existing users

`users:import` copies cashflow's local accounts (username and bcrypt hash, as-is) into aio and links them, so users keep their passwords. It reads cashflow's database from `CASHFLOW_DATABASE_URL`, is a dry run without `--apply`, and writes nothing while any account needs a decision. See `users:import --help`, and [cashflow's runbook](https://github.com/opan/cashflow/blob/main/README.en.md#moving-existing-users-to-all-in-one), which runs it as a Kubernetes Job.

### Running it

- **Single replica.** Logins in progress (auth requests and codes) are kept in memory, so a login started on one pod and finished on another fails. During a `RollingUpdate` deploy, someone mid-login may see "this login link has expired" and has to start again; the `Recreate` strategy avoids even that.
- Metrics (`aio_oidc_login_completed_total`, `aio_oidc_login_failed_total`, …) are listed in [docs/metrics.md](docs/metrics.md).

### Troubleshooting

| Symptom | Cause |
|---|---|
| aio won't start: issuer error | `auth.oidc.issuer` is empty, relative, or has a path |
| aio won't start: signing key can't be decrypted | `auth.totp_encryption_key` changed since the key was created |
| "This login link has expired" | The user took longer than `auth_request_ttl`, aio restarted, or the login hopped between replicas |
| App gets `invalid_request` | Missing PKCE, or `prompt=login` / `max_age` sent |
| App gets `access_denied` after login | aio's callback ran in a browser without the cookie from the start of the login, or without the session of the user who logged in (e.g. the link was opened in another browser) |
| Logout form does nothing in the app | The app's Content-Security-Policy `form-action` must allow aio's origin |

## API Documentation (Swagger)

Swagger UI is disabled by default and not exposed in production. Enable it locally:

```bash
# via config
server:
  swagger_enabled: true

# or via env var
ALLINONE_SERVER_SWAGGER_ENABLED=true go run ./cmd/all-in-one server
```

Once enabled, Swagger UI is available at `http://localhost:8080/swagger/index.html`.

To regenerate after modifying endpoints:

```bash
# Install swag
go install github.com/swaggo/swag/cmd/swag@latest

# Regenerate
swag init -g cmd/all-in-one/main.go -o docs --parseDependency --parseInternal
```

## Observability (OpenTelemetry)

The app ships with OpenTelemetry instrumentation covering HTTP traces, SQL query traces, log correlation, WebSocket per-message spans, and metrics. It is **off by default** and adds zero overhead when disabled.

### What is instrumented

| Signal | What is tracked |
|---|---|
| Traces | Every HTTP request (`otelmux`), every SQL query (`otelsql`), per-message WebSocket spans |
| Logs | `trace_id` + `span_id` injected into every request log line for grep → Jaeger correlation |
| Span attrs | `user.id`, `session.id` on authenticated requests; `ws.upgrade`, `username` on WS connect |
| Metrics | `aio_chat_websocket_connections_active` (gauge), `aio_chat_websocket_messages_received/sent_total` (counters), DB connection pool stats |

### Local setup

Requires Docker. The `docker-compose.yml` at the project root starts the full observability stack:

```
App (go run) → OTel Collector :4318 → Jaeger  :16686  (traces)
                                     → Prometheus :9090 (metrics)
```

**Step 1 — Start the observability stack:**

```bash
docker compose up -d
```

**Step 2 — Run the server with telemetry enabled:**

```bash
ALLINONE_TELEMETRY_ENABLED=true make run-backend
```

**Step 3 — Generate some traffic** (login, browse, send a chat message), then open:

- **Jaeger UI:** http://localhost:16686 — select service `all-in-one`, search for traces
- **Prometheus:** http://localhost:9090 — query `aio_chat_websocket_connections_active` or `aio_chat_websocket_messages_received_total`

**Stop the stack:**

```bash
docker compose down
```

### Telemetry config reference

| Setting | Default | Description |
|---|---|---|
| `telemetry.enabled` | `false` | Master toggle — `false` means zero OTel overhead |
| `telemetry.otlp_endpoint` | `localhost:4318` | OTLP/HTTP receiver (no `http://` prefix) |
| `telemetry.sample_ratio` | `1.0` | `1.0` = always sample. Use `0.1` in production |
| `telemetry.metric_interval` | `15s` | How often metrics are pushed to the collector |
| `telemetry.environment` | `local` | Attached to every span as `deployment.environment` |

### Production notes

- Set `ALLINONE_TELEMETRY_OTLP_ENDPOINT` to your collector's address (e.g. `otel-collector.internal:4318`)
- Lower `ALLINONE_TELEMETRY_SAMPLE_RATIO` to `0.1` to avoid trace volume at scale
- The JWT token appears in the WS upgrade URL (`?token=...`). Strip it at the collector using a `transform` processor or filter the attribute in your backend before indexing

---

## Production Checklist

- [ ] Set `ALLINONE_AUTH_JWT_SECRET` to a strong random value (never commit it)
- [ ] Set `ALLINONE_AUTH_TOTP_ENCRYPTION_KEY` to a strong random value (never commit it)
- [ ] Set `ALLINONE_AUTH_SECURE_COOKIE=true` (requires HTTPS — use a reverse proxy like Nginx or Caddy)
- [ ] Set `ALLINONE_LOG_LEVEL=info`
- [ ] Ensure `ALLINONE_SERVER_SWAGGER_ENABLED` is unset or `false` (default)
- [ ] Put the Go server behind a reverse proxy (Nginx / Caddy) for TLS termination
- [ ] Build the frontend: `cd web && npm run build` (output in `web/build/`, served by the Go server)
- [ ] If using SQLite, set the Kubernetes deployment strategy to `Recreate` (see note below)
- [ ] If enabling telemetry, set `ALLINONE_TELEMETRY_OTLP_ENDPOINT` and lower `ALLINONE_TELEMETRY_SAMPLE_RATIO` to `0.1`
- [ ] If enabling OIDC, set `ALLINONE_AUTH_OIDC_ISSUER` to aio's public `https://` URL and run a single replica (see [Running it](#running-it))

### SQLite and Kubernetes Deployment Strategy

SQLite uses file-level locking that only works on a local filesystem. Running two pods against the same SQLite file simultaneously — even briefly — risks write corruption and data loss.

The default Kubernetes deployment strategy is `RollingUpdate`, which keeps the old pod running until the new one is ready. This means **two pods will be running at the same time during every deploy**, which is unsafe with SQLite.

Set the strategy to `Recreate` to terminate the old pod before the new one starts:

```yaml
spec:
  strategy:
    type: Recreate
```

This causes a brief downtime during deploys, which is acceptable for a single-replica SQLite setup. If zero-downtime deploys are required, migrate to PostgreSQL.

## Development Notes

- SQLite database file is `all-in-one.db` in the project root — do not delete it as it may contain unseeded data
- `direct_auth_enabled: true` skips JWT validation by passing `x-direct-auth-username: <username>` header — only use in development
- The frontend dev server proxies `/api` to `localhost:8080` via Vite config

### Querying the SQLite Database in a Pod

The image is distroless, so `kubectl cp` and `kubectl exec` do not work directly. See [docs/development.md](docs/development.md#copying-the-sqlite-database-from-a-running-pod) for the full step-by-step using an ephemeral `busybox` container.
