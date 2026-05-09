# All-in-One

A multi-functional full-stack web application built for learning purposes.

- **Backend:** Go 1.25+, REST API
- **Frontend:** Svelte 5+, TypeScript 5.9+, Vite 7+

## Features

- **Listing** — Create topics with custom JSON schemas, manage items within topics
- **Chat** — Real-time chat with WebSocket support, invite system
- **Authentication** — JWT-based auth (username/password) with session management
- **Two-Factor Authentication (2FA)** — Opt-in TOTP-based 2FA (Google Authenticator, Authy, etc.)

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
│   ├── config/             # Configuration loading (Viper)
│   ├── http/               # Shared HTTP helpers and middleware
│   └── storage/            # Database connection and migration runner
└── web/                    # Frontend (SvelteKit, Tailwind, shadcn-svelte)
```

## Prerequisites

- Go 1.25+
- Node.js 22.19+, npm 10+
- SQLite (via `go-sqlite3`, requires CGO — `gcc` must be installed)

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
go run ./cmd/all-in-one server            # Start the HTTP server
go run ./cmd/all-in-one db:migrate up     # Apply all pending migrations
go run ./cmd/all-in-one db:migrate down   # Roll back migrations
go run ./cmd/all-in-one db:seed           # Seed sample users, topics, and chat data
```

## Configuration

Configuration is loaded from `config/config.yml`. All values can be overridden via environment variables using the prefix `ALLINONE_` and replacing `.` with `_` (e.g., `auth.jwt_secret` → `ALLINONE_AUTH_JWT_SECRET`).

### Full config reference

```yaml
server:
  port: 8080

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
```

### Environment variable reference

| Variable | Description |
|---|---|
| `ALLINONE_AUTH_JWT_SECRET` | JWT signing secret |
| `ALLINONE_AUTH_TOTP_ENCRYPTION_KEY` | 32-byte hex key for 2FA secret encryption |
| `ALLINONE_AUTH_SECURE_COOKIE` | `true` in production with HTTPS |
| `ALLINONE_SERVER_PORT` | HTTP server port (default: `8080`) |
| `ALLINONE_STORAGE_TYPE` | `sqlite` (default) |
| `ALLINONE_STORAGE_SQLITE_DB_PATH` | Path to SQLite file (default: `all-in-one.db`) |
| `ALLINONE_LOG_LEVEL` | Log level (default: `debug`) |

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

## API Endpoints

All endpoints are under `/api/v1/`.

### Public

| Method | Path | Description |
|---|---|---|
| `GET` | `/health` | Health check |
| `POST` | `/users` | Register a new user |
| `POST` | `/sessions` | Login (returns JWT cookies) |
| `POST` | `/sessions/refresh` | Refresh access token |
| `GET` | `/sessions/verify` | Verify current session |
| `POST` | `/sessions/2fa/verify` | Verify TOTP code during login |
| `POST` | `/sessions/2fa/recovery` | Use a recovery code during login |

### Authenticated (requires valid session)

| Method | Path | Description |
|---|---|---|
| `GET` | `/users/me` | Get current user profile |
| `POST` | `/users/reset_password` | Change password |
| `DELETE` | `/sessions` | Logout |
| `GET` | `/users/2fa/status` | Get 2FA status and remaining recovery codes |
| `POST` | `/users/2fa/setup` | Begin 2FA setup (returns QR code + secret + recovery codes) |
| `POST` | `/users/2fa/verify-setup` | Confirm 2FA setup with a TOTP code |
| `DELETE` | `/users/2fa` | Disable 2FA (requires current password) |
| `POST` | `/users/2fa/recovery-codes/regenerate` | Regenerate recovery codes (requires current password) |
| `GET` | `/listing/topics` | List topics |
| `POST` | `/listing/topics` | Create topic |
| `GET` | `/listing/topics/{id}` | Get topic |
| `PUT` | `/listing/topics/{id}` | Update topic |
| `DELETE` | `/listing/topics/{id}` | Delete topic |
| `GET` | `/listing/topics/{id}/items` | List items in topic |
| `POST` | `/listing/topics/{id}/items` | Create item |
| `PUT` | `/listing/topics/{id}/items/{itemId}` | Update item |
| `DELETE` | `/listing/topics/{id}/items/{itemId}` | Delete item |
| `GET` | `/chats` | List chat sessions |
| `POST` | `/chats` | Create chat session |
| `DELETE` | `/chats/{id}` | Delete chat session |
| `GET` | `/chats/{id}/messages` | Get messages |
| `POST` | `/chats/{id}/messages` | Send message |
| `POST` | `/chats/invites` | Send chat invite |
| `GET` | `/chats/invites/received` | List received invites |
| `GET` | `/chats/invites/sent` | List sent invites |
| `POST` | `/chats/invites/{id}/respond` | Accept or decline invite |
| `DELETE` | `/chats/invites/{id}` | Cancel invite |

WebSocket: `ws://localhost:8080/api/v1/ws?token=<access_token>`

## API Documentation (Swagger)

Swagger UI is available at `http://localhost:8080/swagger/index.html` when the server is running.

To regenerate after modifying endpoints:

```bash
# Install swag
go install github.com/swaggo/swag/cmd/swag@latest

# Regenerate
swag init -g cmd/all-in-one/main.go -o docs --parseDependency --parseInternal
```

## Production Checklist

- [ ] Set `ALLINONE_AUTH_JWT_SECRET` to a strong random value (never commit it)
- [ ] Set `ALLINONE_AUTH_TOTP_ENCRYPTION_KEY` to a strong random value (never commit it)
- [ ] Set `ALLINONE_AUTH_SECURE_COOKIE=true` (requires HTTPS — use a reverse proxy like Nginx or Caddy)
- [ ] Set `ALLINONE_LOG_LEVEL=info`
- [ ] Put the Go server behind a reverse proxy (Nginx / Caddy) for TLS termination
- [ ] Build the frontend: `cd web && npm run build` (output in `web/build/`, served by the Go server)

## Development Notes

- SQLite database file is `all-in-one.db` in the project root — do not delete it as it may contain unseeded data
- `direct_auth_enabled: true` skips JWT validation by passing `x-direct-auth-username: <username>` header — only use in development
- The frontend dev server proxies `/api` to `localhost:8080` via Vite config
