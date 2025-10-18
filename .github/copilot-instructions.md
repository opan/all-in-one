# GitHub Copilot Instructions

## Project Overview

All-in-one is an app that contains multiple application. User can start this app in different form, for example to run a listing app we can run (`all-in-one listing <options>`). And more app will be added.

- Full-stack web application.
- **Backend:** Go (1.25+), REST API, stored in the project root.
- **Frontend:** Svelte 5+ with TypeScript 5.9+, source under `web/`.

## Project Layout

.
├── cmd/ # Cobra CLI entrypoints
├── config/ # viper configuration files (e.g. config.yaml)
├── internal/ # application internal packages
│ ├── handlers/ # HTTP handlers
│ ├── services/ # business logic
│ ├── repositories/ # data access (SQLite)
│ ├── models/ # domain models
│ └── server/ # server setup & middleware
├── web/ # Svelte + TypeScript frontend
│ ├── src/
│ ├── static/
│ └── package.json
├── go.mod
├── go.sum
└── main.go

## Backend (Go) Guidelines
- **Go version:** 1.25+
- **Libraries to prefer:**
  - `github.com/mattn/go-sqlite3` (SQLite driver)
  - `github.com/spf13/viper` (configuration)
  - `github.com/spf13/cobra` (CLI)
  - `github.com/rs/zerolog` (structured logging)
  - `github.com/jmoiron/sqlx` (database access)
- Use idiomatic Go: small functions, explicit error handling, `context.Context` passed to handlers/services.
- Use dependency injection (no globals) where practical.
- Each app might support multiple storage backends (e.g., SQLite, in-memory). Abstract storage access via repository interfaces.
- In general, organize code:
  - `internal/common/` for code that can be shared between app.
  - `internal/<app-name>` for code specific to an app. Each app can have its own sub-packages as needed
  - `internal/<app-name>/handler/` for HTTP handlers
  - `internal/<app-name>/service/` for business logic
  - `internal/<app-name>/model/` for domain models
  - `internal/<app-name>/repository/` for data access. 

- Database access should use `github.com/jmoiron/sqlx` with prepared statements and migrations (if applicable).
- Configuration:
  - Default config file: `config/config.yaml`
  - Allow env var overrides via `viper`
- Logging:
  - Use `zerolog` with structured fields such as `request_id`, `module`, `error`.
- REST API:
  - Base path: `/api/v1/`
  - JSON-only responses
  - Standard response envelope:
    ```json
    {
      "success": true,
      "data": <object|null>,
      "error": <string|null>
    }
    ```
  - Use middleware for logging, error handling, and request ID.

## Frontend (Svelte + TypeScript) Guidelines
- **Svelte:** 5+
- **TypeScript:** 5.9+
- Frontend code located in the `web/` directory.
- Use `<script lang="ts">` in Svelte components.
- Keep an API client wrapper (e.g., `web/src/lib/api.ts`) and define shared TS interfaces for request/response shapes.
- Use Svelte stores for shared state.
- Configure API base URL via environment variable (e.g. `VITE_API_BASE_URL` or preferred solution for your build tooling).
- Minimal routing and components under `web/src/` (e.g., `components/`, `routes/` or `pages/` according to your router).

## CLI & Commands
- Use `cobra` to implement CLI entrypoints under `cmd/`.
- Example commands:
  - `serve` — run the HTTP server
  - `migrate` — run DB migrations (optional)
  - `version` — print app version

## Example Commands (dev)

```bash
# Backend (run)
go run main.go serve

# Backend (build)
go build -o bin/server main.go

# Frontend (dev)
cd web
npm install
npm run dev

# Frontend (build)
cd web
npm run build
Copilot Hints
```

When Copilot generates or suggests code:

- Prefer modular, testable Go code over single-file implementations.
- Use context.Context for request boundaries and cancellations.
- Use zerolog for all logs, include context fields.
- Use viper to read config/config.yaml and environment variables.
- Put API endpoints under /api/v1/....
- Generate frontend code under web/src/ only; use TypeScript types for all API interactions.

Keep examples minimal and idiomatic.

Minimal config/config.yaml example

```yaml
server:
  port: 8080

database:
  path: "data/app.db"
```

Response Format Expectations
Successful JSON responses should populate success: true, data: <...>, error: null.

On errors, use success: false, data: null, and a descriptive error string.

HTTP status codes must match the result (e.g., 200, 201, 400, 404, 500).

## Notes

Backend code lives in the repository root (top-level Go modules).

Frontend source code must live under web/.

Keep dependencies minimal and prefer the listed libraries for core functionality.