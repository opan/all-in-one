# Development Guide

Reference for local development, debugging, and common operational tasks.

---

## Local Setup

### Backend

```bash
go mod tidy
go run ./cmd/all-in-one db:migrate up
go run ./cmd/all-in-one db:seed     # optional sample data
go run ./cmd/all-in-one server
```

API available at `http://localhost:8080`.

### Frontend

```bash
cd web
npm install
npm run dev
```

Dev server at `http://localhost:5173`. Vite proxies `/api` to `localhost:8080` automatically.

---

## Environment & Config

Copy and adjust `config/config.yml` for local overrides, or export environment variables:

```bash
export ALLINONE_AUTH_JWT_SECRET="$(openssl rand -hex 32)"
export ALLINONE_AUTH_TOTP_ENCRYPTION_KEY="$(openssl rand -hex 32)"
```

Setting `direct_auth_enabled: true` lets you bypass JWT by passing `x-direct-auth-username: <username>` — only use this locally.

---

## Database

### Migrations

```bash
go run ./cmd/all-in-one db:migrate up     # apply pending migrations
go run ./cmd/all-in-one db:migrate down   # roll back last migration
```

Migration files live in `db/migrations/`.

### Seeding

```bash
go run ./cmd/all-in-one db:seed
```

### Local SQLite queries

```bash
sqlite3 all-in-one.db
sqlite3 all-in-one.db "SELECT * FROM users;"
```

> Do not delete `all-in-one.db` — it may contain data not reproducible from seed.

### PostgreSQL Setup

For local development, start the bundled Postgres container:

```bash
docker compose up -d postgres
```

For a manual setup (local or production), run the following as a PostgreSQL superuser (`psql -U postgres`):

```sql
-- Create the role/user
CREATE ROLE allinone WITH LOGIN PASSWORD 'replace-with-strong-password';

-- Create the database
CREATE DATABASE allinone OWNER allinone ENCODING 'UTF8';

-- Connect to the database
\c allinone

-- Revoke default public access
REVOKE ALL ON SCHEMA public FROM PUBLIC;

-- Grant schema access to the app role
GRANT USAGE ON SCHEMA public TO allinone;
GRANT CREATE ON SCHEMA public TO allinone;  -- required for golang-migrate to create schema_migrations

-- Grant privileges on existing tables and sequences
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO allinone;
GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO allinone;

-- Apply grants automatically to future tables/sequences created by migrations
ALTER DEFAULT PRIVILEGES IN SCHEMA public
    GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO allinone;
ALTER DEFAULT PRIVILEGES IN SCHEMA public
    GRANT USAGE, SELECT ON SEQUENCES TO allinone;
```

> After the initial migration run you can optionally tighten the lockdown:
> ```sql
> REVOKE CREATE ON SCHEMA public FROM allinone;
> ```

Then point the app at the Postgres instance via `config/config.yml` or env vars, and run migrations:

```bash
ALLINONE_STORAGE_TYPE=postgres \
ALLINONE_STORAGE_POSTGRES_USER=allinone \
ALLINONE_STORAGE_POSTGRES_PASSWORD=your-password \
ALLINONE_STORAGE_POSTGRES_DBNAME=allinone \
go run ./cmd/all-in-one db:migrate up
```

### Migrating data between SQLite and PostgreSQL

Both databases must be fully migrated before running a transfer, and the destination must be empty.

```bash
# SQLite → PostgreSQL
make db-transfer ARGS="--direction sqlite-to-pg --confirm"

# PostgreSQL → SQLite
make db-transfer ARGS="--direction pg-to-sqlite --confirm"
```

Both `storage.sqlite` and `storage.postgres` must be configured (or set via env vars) regardless of direction — the command opens both connections simultaneously. The SQLite path defaults to `all-in-one.db`.

---

### Disable 2FA for a local account

If you locked yourself out after enabling 2FA, reset it directly in the database:

```bash
sqlite3 all-in-one.db "UPDATE users SET totp_enabled=0, totp_secret_encrypted=NULL, totp_verified_at=NULL WHERE username='your_username';"
```

No restart required — the next login will skip the 2FA challenge.

---

## Copying the SQLite Database from a Running Pod

The production image is distroless — no shell, no `cat`, no `tar`. Neither `kubectl cp` nor `kubectl exec` with a shell work directly. The solution is to inject an ephemeral `busybox` container that shares the pod's process namespace, then use `kubectl cp` targeting that container.

### Copy the database out of the pod

**1. Get the pod name:**

```bash
kubectl get pod -n app -l app=all-in-one
```

**2. Inject an ephemeral busybox container:**

```bash
kubectl debug -n app <pod-name> --image=busybox --target=all-in-one -- sleep 3600 &
sleep 5
```

The `kubectl` command exits after creating the ephemeral container; `sleep 3600` keeps it alive inside the pod.

**3. Get the ephemeral container name (printed in step 2 output, e.g. `debugger-hjl77`):**

```bash
kubectl get pod -n app <pod-name> -o jsonpath='{.spec.ephemeralContainers[-1].name}'
```

**4. Copy the database using the ephemeral container:**

```bash
kubectl cp -n app -c <debug-container-name> \
  <pod-name>:/proc/1/root/data/all-in-one.db \
  ./prod-all-in-one.db
```

`/proc/1/root/` is the main container's filesystem, visible from the ephemeral container via the shared process namespace.

**5. Verify the file:**

```bash
file ./prod-all-in-one.db
# SQLite 3.x database, ...
```

### Copy a modified database back into the pod

Only needed if you ran write queries (`INSERT`, `UPDATE`, `DELETE`). Scale the deployment to zero first to avoid concurrent writes corrupting the file.

```bash
kubectl scale deployment all-in-one -n app --replicas=0

# Wait for the pod to terminate, then copy back
kubectl cp -n app -c <debug-container-name> \
  ./prod-all-in-one.db \
  <pod-name>:/proc/1/root/data/all-in-one.db

kubectl scale deployment all-in-one -n app --replicas=1
```

---

## Swagger / API Docs

Swagger UI is at `http://localhost:8080/swagger/index.html` when the server is running.

Regenerate after modifying endpoints:

```bash
go install github.com/swaggo/swag/cmd/swag@latest
swag init -g cmd/all-in-one/main.go -o docs --parseDependency --parseInternal
```

---

## Security Scanning

The CI pipeline runs the following scans on every push:

| Tool | What it checks |
|---|---|
| `govulncheck` | Known Go CVEs in stdlib and dependencies |
| `gosec` | Go SAST (high severity, medium confidence) |
| `gitleaks` | Leaked secrets in git history |
| `npm audit` | Frontend dependency vulnerabilities (high+) |
| `trivy` | Filesystem scan (HIGH/CRITICAL) |

Run `govulncheck` locally:

```bash
go install golang.org/x/vuln/cmd/govulncheck@latest
govulncheck ./...
```
