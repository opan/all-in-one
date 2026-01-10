# GitHub Copilot Instructions

## Project Overview

All-in-one is a multi-functional apps with the main goal for learning.

- Full-stack web application.
- **Backend:** Go (1.25+), REST API, stored in the project root.
- **Frontend:** Svelte 5+ with TypeScript 5.9+, source under `web/`.

## Tech stacks

### Backend (Go) Guidelines
- **Go version:** 1.25+
- **Libraries to prefer:**
  - `github.com/mattn/go-sqlite3` (SQLite driver)
  - `github.com/lib/pq` (PostgreSQL driver)
  - `github.com/spf13/viper` (configuration)
  - `github.com/spf13/cobra` (CLI)
  - `github.com/rs/zerolog` (structured logging)
  - `github.com/jmoiron/sqlx` (database access)
  - `golang.org/x/crypto/bcrypt` (for data encryption/password)
  - `github.com/golang-jwt/jwt/v5` (for JWT)
  - `github.com/gorilla/mux` (HTTP routing)
- Use idiomatic Go: small functions, explicit error handling, `context.Context` passed to handlers/services.
- Use dependency injection (no globals) where practical.
- Each app might support multiple storage backends (e.g., SQLite, PostgreSQL). Abstract storage access via repository interfaces.
- Current database supports
  - SQLite (via `github.com/mattn/go-sqlite3`)
  - PostgreSQL (via `github.com/lib/pq`)

#### Project structure
In general, organize code:
  - `bin/` for compiled binaries (if applicable).
  - `cmd/all-in-one/main.go` for CLI entrypoints.
  - `config/config.yml` for default configuration files for each app.
  - `internal/config/` for configuration related code.
  - `internal/<app-name>` for code specific to an app. Each app can have its own sub-packages as needed
  - `internal/<app-name>/handler/` for HTTP handlers. Each domain can have its own file, e.g.: item.go, user.go
  - `internal/<app-name>/service/` for business logic
  - `internal/<app-name>/model/` for domain models
  - `internal/<app-name>/repository/` for data access. 
  - `internal/<app-name>/<any>/` for other sub-packages as needed (e.g., authnz, middleware, util).
  - `pkg/` for any reusable packages that could be shared across multiple projects outside of this project (if applicable).
  - `web/` for frontend source code.

Each app will have its own package under `internal/` (e.g., `internal/todo`, `internal/note`).

#### Configuration
Configuration:
  - Default config file: `config/config.yml`
  - Allow env var overrides via `viper`

#### Testing
- Use table-driven tests whenever possible.
- Use `github.com/stretchr/testify` for assertions.
- Use `github.com/vektra/mockery` for generating mocks.

#### Middleware

Use middleware for common functionality:
  - Authentication and authorization
  - Request logging
  - Error handling
  - Request ID

#### Authentication
Support authentications:
  - JWT (combined with common auth username/password)

- Database access should use `github.com/jmoiron/sqlx` with prepared statements and migrations (if applicable).
- Use squirrel library to build SQL queries safely (where applicable).
- Logging:
  - Use `zerolog` with structured fields such as `request_id`, `module`, `error`.
- REST API:
  - Base path: `/api/v1/`
  - JSON-only responses
  - Standard response envelope (as stated in `internal/http/http.go`):
    ```json
    {
      "success": true,
      "message": <string|null>,
      "data": <object|null>,
      "error": <string|null>
    }
    ```
  - Use middleware for logging, error handling, and request ID.

#### Must have
- Need to have context that get passing down to DB calls, HTTP requests, and other as required via function parameter/argument.
- Integrate with Swagger for easier API documentation and manual testing when required.

#### List of apps

- `listing` - a simple item listing app with CRUD operations and user authentication.
- `authnz` - authentication and authorization module (JWT + username/password).
- `chat` - a simple chat app with WebSocket support.

### Frontend (Svelte + TypeScript) Guidelines
- **Svelte:** 5+
- **TypeScript:** 5.9+
- **Vite** 7+
- **node:**: 22.19+
- **npm:** 10+

#### Project structure
- `web/src/routes/` for SvelteKit routes.
- `web/src/lib/` for shared libraries (e.g., API client, stores).
- `web/src/components/` for reusable Svelte components.
- `web/src/lib/components/ui/` for shadcn-svelte components.

#### Coding guidelines
- Follows Svelte and TypeScript best practices.
- For authentication, refer to https://svelte.dev/docs/kit/auth.
- For performance, refer to https://svelte.dev/docs/kit/performance
- For SEO, refer to https://svelte.dev/docs/kit/seo
- For icons , refer to https://svelte.dev/docs/kit/icons
- For images, refer to https://svelte.dev/docs/kit/images
- For accessibility, refer to https://svelte.dev/docs/kit/accessibility
- Follow do's and don'ts for TypeScript here: https://www.typescriptlang.org/docs/handbook/declaration-files/do-s-and-don-ts.html

- Use `<script lang="ts">` in Svelte components.
- Keep an API client wrapper (e.g., `web/src/lib/api.ts`) and define shared TS interfaces for request/response shapes.
- Use Svelte stores for shared state.
- Configure API base URL via environment variable (e.g. `VITE_API_BASE_URL` or preferred solution for your build tooling).

#### Styling
- Use tailwindcss.
- Use shadcn-svelte (https://shadcn-svelte.com/docs) for component. To add new component library, e.g.: `npx shadcn-svelte@latest add dialog`

## CLI & Commands
- Use `cobra` to implement CLI entrypoints under `cmd/`.
- Example commands:
  - `serve` — run the HTTP server
  - `migrate` — run DB migrations (optional)
  - `version` — print app version

## Example Commands (dev)

```bash
# Backend (run) to run listing app
go run main.go all-in-one server

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