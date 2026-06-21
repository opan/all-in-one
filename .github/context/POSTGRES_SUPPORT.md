# Plan: Support PostgreSQL (15+) as a storage backend

> **Audience:** an implementing engineer/model with no prior context. This doc is
> self-contained — exact code, SQL, and file-by-file edits are included so it can
> be executed with minimal guesswork.

---

## Status

- [x] §3 Config changes
- [x] §4 Central storage (postgres.go + storage.go + sqlite.go FK fix)
- [x] §5 Unify connection wiring (server.go + service signatures + factories)
- [x] §6 Per-driver Postgres repository packages
- [x] §7 Postgres migrations (db/migrations/postgres/)
- [x] §8 Dependencies (go get lib/pq + go mod vendor)
- [x] §9 docker-compose.yml (added postgres service to existing file)
- [x] §10 docs/metrics.md update
- [ ] §12 Verification pass (requires running postgres)

---

## 1. Context & Goal

The app currently runs on **SQLite only**:
- `internal/storage/storage.go` `NewStorage()` **panics** on any non-`sqlite` type.
- Config has no Postgres fields (`internal/config/config.go`, `config/config.yml`).
- All repository SQL lives in per-app `repository/sqlite/` packages using `?`
  placeholders; listing uses `LastInsertId()`.
- Migrations exist only under `db/migrations/sqlite3/` (5 versions, up+down).
- `internal/chat/repository/factory.go` and `internal/listing`/`shortener`
  factories already carry a `postgres` stub returning *"not yet implemented"*.

**Goal:** running with `storage.type: postgres` runs the entire app against a
PostgreSQL **15+** server. SQLite remains the default and must keep working.

**Confirmed design decisions (from the user):**
1. **Per-driver repositories** — add `repository/postgres/` packages alongside the
   existing `sqlite` ones (do NOT make repos driver-neutral with `Rebind`).
2. **Unify connections** — all apps consume the single shared `*sqlx.DB` from
   central storage (authnz already does; convert listing/chat/shortener).
3. **Ship docker-compose** — a `postgres:15` service for local dev.

**Driver:** `github.com/lib/pq` (the CLAUDE.md-preferred Postgres driver). Register
it with a blank import; sqlx driver name is `"postgres"`.

---

## 2. Dialect translation rules (apply in every `postgres/` repo)

When translating a `sqlite/` repo file to its `postgres/` twin, apply ALL of:

| # | SQLite (current) | PostgreSQL (target) |
|---|---|---|
| R1 | Positional `?` placeholders | `$1, $2, $3, …` (numbered, in order) |
| R2 | `INSERT … ; LastInsertId()` | `INSERT … RETURNING id`, scanned via `QueryRowxContext(...).Scan(&id)` |
| R3 | `is_active = 1` (bool-as-int literal) | `is_active = TRUE` |
| R4 | timestamps formatted to RFC3339 strings before insert, parsed back after | insert `time.Time` directly; scan TIMESTAMP columns straight into `time.Time` struct fields. **Delete** the dead `var createdAt, updatedAt string` + `time.Parse(...)` lines (see §6.2) |
| R5 | Named-param inserts via `NamedExecContext` (`:field`) | **No change** — sqlx rewrites named params per-driver automatically |
| R6 | `r.db.BeginTxx`, transaction helpers, `Execer`/`getExecCtx` types | replicate verbatim (logic is dialect-independent) |
| R7 | imports `_ "github.com/mattn/go-sqlite3"`, `package sqlite` | imports `_ "github.com/lib/pq"`, `package postgres` |

Everything else (struct names, method signatures, interfaces, logging, error
wrapping, `sql.ErrNoRows` handling) is copied **unchanged** so the factory can
swap packages transparently.

---

## 3. Config changes

### 3.1 `internal/config/config.go`
Add the Postgres config type and embed it in `StorageConfig` (currently
config.go:62-72):

```go
type StorageConfig struct {
	Type     string         `mapstructure:"type"`     // "memory" | "sqlite" | "postgres"
	Memory   MemoryConfig   `mapstructure:"memory"`
	SQLite   SQLiteConfig   `mapstructure:"sqlite"`
	Postgres PostgresConfig `mapstructure:"postgres"` // used for postgres storage
}

type PostgresConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	DBName   string `mapstructure:"dbname"`
	SSLMode  string `mapstructure:"sslmode"` // disable | require | verify-full
}

// DSN builds a lib/pq connection string.
func (c PostgresConfig) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		c.Host, c.Port, c.User, c.Password, c.DBName, c.SSLMode,
	)
}
```

In `Load()` add defaults (near config.go:99) and env binds (near config.go:136):

```go
viper.SetDefault("storage.postgres.host", "localhost")
viper.SetDefault("storage.postgres.port", 5432)
viper.SetDefault("storage.postgres.sslmode", "disable")
// ...
viper.BindEnv("storage.postgres.host", "ALLINONE_STORAGE_POSTGRES_HOST")
viper.BindEnv("storage.postgres.port", "ALLINONE_STORAGE_POSTGRES_PORT")
viper.BindEnv("storage.postgres.user", "ALLINONE_STORAGE_POSTGRES_USER")
viper.BindEnv("storage.postgres.password", "ALLINONE_STORAGE_POSTGRES_PASSWORD")
viper.BindEnv("storage.postgres.dbname", "ALLINONE_STORAGE_POSTGRES_DBNAME")
viper.BindEnv("storage.postgres.sslmode", "ALLINONE_STORAGE_POSTGRES_SSLMODE")
```

(`fmt` is already imported in config.go.)

### 3.2 `config/config.yml`
Replace the commented `postgres:` block (config.yml:11-13) with:

```yaml
storage:
  type: "sqlite"  # Options: "memory", "sqlite", or "postgres"
  sqlite:
    db_path: "all-in-one.db"
  postgres:
    host: "localhost"
    port: 5432
    user: "allinone"
    password: "allinone"
    dbname: "allinone"
    sslmode: "disable"   # use "require"/"verify-full" in production
```

---

## 4. Central storage

### 4.1 New file `internal/storage/postgres.go`
Mirror `internal/storage/sqlite.go` exactly, swapping driver + migrate source:

```go
package storage

import (
	"fmt"

	"github.com/all-in-one/internal/config"
	"github.com/golang-migrate/migrate/v4"
	postgresMigrate "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/XSAM/otelsql"
	"go.opentelemetry.io/otel/attribute"
)

type postgresStorage struct {
	db *sqlx.DB
}

func NewPostgres(config config.Config) (*postgresStorage, error) {
	sqlDB, err := otelsql.Open("postgres", config.Storage.Postgres.DSN(),
		otelsql.WithAttributes(attribute.String("db.system", "postgresql")),
	)
	if err != nil {
		return nil, err
	}

	db := sqlx.NewDb(sqlDB, "postgres")

	otelsql.RegisterDBStatsMetrics(sqlDB,
		otelsql.WithAttributes(attribute.String("db.system", "postgresql")),
	)

	return &postgresStorage{db: db}, nil
}

func (s *postgresStorage) DB() *sqlx.DB { return s.db }

func (s *postgresStorage) newMigrator() (*migrate.Migrate, error) {
	driver, err := postgresMigrate.WithInstance(s.db.DB, &postgresMigrate.Config{})
	if err != nil {
		return nil, fmt.Errorf("failed to create migration driver: %w", err)
	}
	m, err := migrate.NewWithDatabaseInstance(
		"file://db/migrations/postgres",
		"postgres",
		driver,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create migrate instance: %w", err)
	}
	return m, nil
}

func (s *postgresStorage) MigrateUp() error {
	m, err := s.newMigrator()
	if err != nil {
		return err
	}
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("migration up failed: %w", err)
	}
	return nil
}

func (s *postgresStorage) MigrateDown(steps int) error {
	m, err := s.newMigrator()
	if err != nil {
		return err
	}
	if steps == 0 {
		if err := m.Down(); err != nil && err != migrate.ErrNoChange {
			return fmt.Errorf("migration down failed: %w", err)
		}
		return nil
	}
	if err := m.Steps(-steps); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("migration down %d step(s) failed: %w", steps, err)
	}
	return nil
}
```

### 4.2 `internal/storage/storage.go`
Add the case to `NewStorage` (storage.go:14-21):

```go
case "postgres":
	return NewPostgres(config)
```

### 4.3 `internal/storage/sqlite.go` — keep FK enforcement after unification
Shortener relies on FK cascade and today sets it in its own connection string
(`internal/shortener/repository/sqlite/storage.go:26`,
`...DBPath + "?_foreign_keys=on"`). Once apps share the central pool, append the
same flag to the central SQLite DSN (sqlite.go:22):

```go
sqlDB, err := otelsql.Open("sqlite3", config.Storage.SQLite.DBPath+"?_foreign_keys=on", ...)
```

---

## 5. Unify connection wiring onto the shared pool

`cmd/all-in-one/server/server.go` already builds `store` then `db := store.DB()`
(server.go:76-82) and passes `db` only to authnz (server.go:90). Pass it to all:

```go
asvc, err := authnzSvc.NewService(ctx, db, s.config, s.log)         // unchanged
lsvc, err := listingSvc.NewService(ctx, db, s.config, s.log)        // + db
csvc, err := chatSvc.NewService(ctx, db, s.config, s.log)           // + db
ssvc, err := shortenerSvc.NewService(ctx, db, s.config, s.log)      // + db
```

### 5.1 Service constructor signature changes
Add `db *sqlx.DB` (import `github.com/jmoiron/sqlx`) as the 2nd param, matching
authnz's existing `NewService(ctx, db, config, log)`:
- `internal/listing/service/service.go:21`
- `internal/chat/service/service.go:23`
- `internal/shortener/service/service.go:19`

Inside each, replace the storage construction call:
- listing service.go:22 `repository.NewStorage(ctx, config, log)` → `repository.NewStorage(db, config, log)`
- chat service.go:25 `repository.NewStorage(ctx, config, log)` → `repository.NewStorage(db, config, log)`
- shortener service.go:20 `repository.NewStorage(ctx, config, log)` → `repository.NewStorage(db, config, log)`

### 5.2 Repository factory changes (the per-driver switch)
Model all factories after `internal/authnz/repository/factory.go` (`NewRepo(db, config)`):
each takes the shared `db`, switches on `config.Storage.Type`, and builds repos
from `sqlite.New…(db, …)` **or** `postgres.New…(db, …)`.

**authnz** (`internal/authnz/repository/factory.go`): add
```go
case "postgres":
	storage := postgres.NewStorage(db, config)
	return &sqliteStoreAdapter{
		userRepo: storage.UserRepo(), sessionRepo: storage.SessionRepo(), totpRepo: storage.TOTPRepo(),
	}, nil
```

**listing** (`internal/listing/repository/factory.go`): change signature to
`NewStorage(db *sqlx.DB, config config.Config, log zerolog.Logger)`, drop the
`ctx`. The `sqlite` branch builds from the passed `db` (see §5.3). Add a
`postgres` branch using `postgres.NewStorage(db, log)`.

**chat** (`internal/chat/repository/factory.go` + `sqlite.go`): change
`NewStorage`/`NewSQLiteStorage` to accept `db`. **Remove the inner
`storage.NewStorage(config)` call** at `internal/chat/repository/sqlite.go:28`
(it currently re-opens a second central connection) — use the passed `db`
instead. Add a `NewPostgresStorage(db, log)` peer.

**shortener** (`internal/shortener/repository/factory.go`): it already exposes
`NewStorageFromDB(db)` (factory.go:26). Change `NewStorage` to take `db` and
route both `sqlite`/`postgres` cases through the `*FromDB` constructors
(add `postgres.NewFromDB(db)`).

### 5.3 Remove per-app self-opened connections
- `internal/listing/repository/sqlite/storage.go:24` currently does
  `sqlx.Open("sqlite3", config.Storage.SQLite.DBPath)`. Change `NewStorage` to
  accept the shared `db` (add a `NewFromDB(db)` if cleaner) and build
  `itemRepo`/`topicRepo` from it. Do NOT open a new connection.
- `internal/shortener/repository/sqlite/storage.go` — same; the `?_foreign_keys=on`
  it added is now handled centrally (§4.3).

---

## 6. Per-driver Postgres repository packages

Create a `repository/postgres/` package for each app, one file per existing
`sqlite/` file, applying the §2 rules. **Replicate supporting types** (e.g.
listing's `queryOptions`, `Execer`, `getExecCtx`, `CreateTrx`) verbatim.

Files to create (mirror of existing `sqlite/` files):
- `internal/authnz/repository/postgres/`: `storage.go`, `user_repository.go`, `session_repository.go`, `totp_repository.go`
- `internal/listing/repository/postgres/`: `storage.go`, `item_repository.go`, `topic_repository.go`
- `internal/chat/repository/postgres/`: `storage.go` (peer of `sqlite.go` wiring), `session.go`, `message.go`, `invite.go`
- `internal/shortener/repository/postgres/`: `storage.go`, `shortlink_repository.go`

### 6.1 Concrete example — listing `topic_repository.go` (sqlite → postgres)
SQLite `Create` (topic_repository.go:96-118) uses RFC3339 strings + `LastInsertId()`.
Postgres twin:

```go
func (r *topicRepository) Create(ctx context.Context, topic model.Topic) (model.Topic, error) {
	now := time.Now().UTC()
	var id int
	err := r.db.QueryRowxContext(ctx, `
		INSERT INTO topics (user_id, name, description, created_at, updated_at, form_schema)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id
	`, topic.UserID.String(), topic.Name, topic.Description, now, now, topic.FormSchema).Scan(&id)
	if err != nil {
		return model.Topic{}, fmt.Errorf("unable to insert topic into db: %w", err)
	}
	topic.ID = id
	topic.CreatedAt = now
	topic.UpdatedAt = now
	return topic, nil
}
```

`GetAll`/`Get`/`Update`/`Delete`: identical bodies, only `?` → `$1…`. The
`SELECT *` calls scan straight into `model.Topic` (its `CreatedAt`/`UpdatedAt`
are `time.Time` — listing/model/topic.go:159, item.go:15 — which lib/pq returns
natively, so no parsing needed).

### 6.2 Listing read path — drop the dead string-parse (latent bug)
`internal/listing/repository/sqlite/item_repository.go:58-79` (and the topic
equivalent) declare `var createdAt, updatedAt string`, scan the row into the
struct via `GetContext(&item)`, then **overwrite** the correctly-scanned
timestamps with `time.Parse(time.RFC3339, createdAt)` on empty strings → zero
time. In the Postgres twin, simply omit those `var`/`time.Parse` lines and return
the struct as scanned. (Optional: fix the SQLite version too for consistency.)

### 6.3 Shortener boolean filter
`internal/shortener/repository/sqlite/shortlink_repository.go:155` `AND is_active = 1`
→ `AND is_active = TRUE` in the postgres twin.

---

## 7. Postgres migrations — `db/migrations/postgres/`

Create 5 up + 5 down files (same version prefixes as `db/migrations/sqlite3/`).
Full contents below.

### `01_create_init_table.up.sql`
```sql
CREATE TABLE IF NOT EXISTS users (
  id TEXT PRIMARY KEY,
  username TEXT NOT NULL UNIQUE,
  email TEXT NOT NULL UNIQUE,
  name TEXT UNIQUE,
  password_hash TEXT NOT NULL,
  last_login TIMESTAMP,
  created_at TIMESTAMP,
  updated_at TIMESTAMP
);

CREATE TABLE IF NOT EXISTS topics (
  id BIGINT GENERATED BY DEFAULT AS IDENTITY PRIMARY KEY,
  user_id TEXT NOT NULL,
  name TEXT NOT NULL,
  description TEXT,
  form_schema TEXT,
  created_at TIMESTAMP,
  updated_at TIMESTAMP,
  FOREIGN KEY (user_id) REFERENCES users(id)
);

CREATE TABLE IF NOT EXISTS items (
  id BIGINT GENERATED BY DEFAULT AS IDENTITY PRIMARY KEY,
  topic_id BIGINT,
  title TEXT NOT NULL,
  description TEXT,
  form_schema_values TEXT,
  created_at TIMESTAMP,
  updated_at TIMESTAMP,
  FOREIGN KEY (topic_id) REFERENCES topics(id)
);

CREATE TABLE IF NOT EXISTS sessions (
  id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL,
  created_at TIMESTAMP,
  user_agent TEXT,
  access_token_expiry BIGINT,
  refresh_token_expiry BIGINT,
  FOREIGN KEY (user_id) REFERENCES users(id)
);
```
> Note: `users` is created first so the FKs resolve (no need for SQLite's
> deferred-FK leniency). `id` columns that were SQLite `INTEGER AUTOINCREMENT`
> become `BIGINT GENERATED BY DEFAULT AS IDENTITY`; `INTEGER` token columns
> (token expiries) become `BIGINT`.

### `01_create_init_table.down.sql`
```sql
DROP TABLE IF EXISTS sessions;
DROP TABLE IF EXISTS items;
DROP TABLE IF EXISTS topics;
DROP TABLE IF EXISTS users;
```

### `02_create_chat_tables.up.sql`
Same as the SQLite version (the chat DDL is already PG-compatible — `TEXT` PKs,
`TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP`, composite PK, `ON DELETE
CASCADE`, and all 7 indexes copy verbatim). Reproduce
`db/migrations/sqlite3/02_create_chat_tables.up.sql` unchanged.

### `02_create_chat_tables.down.sql`
Copy `db/migrations/sqlite3/02_create_chat_tables.down.sql` verbatim (drops 7
indexes then 3 tables).

### `03_create_chat_invites_table.up.sql` / `.down.sql`
Copy the SQLite versions verbatim — already PG-compatible (TEXT PK, TIMESTAMP
defaults, FKs incl. `ON DELETE SET NULL`, 5 indexes).

### `04_add_2fa_tables.up.sql`
```sql
ALTER TABLE users ADD COLUMN totp_enabled BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE users ADD COLUMN totp_secret_encrypted TEXT;
ALTER TABLE users ADD COLUMN totp_verified_at TIMESTAMP;

CREATE TABLE IF NOT EXISTS recovery_codes (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    code_hash TEXT NOT NULL,
    used_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);
CREATE INDEX idx_recovery_codes_user_id ON recovery_codes(user_id);

CREATE TABLE IF NOT EXISTS totp_challenges (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL,
    expires_at TIMESTAMP NOT NULL,
    attempts INTEGER NOT NULL DEFAULT 0,
    user_agent TEXT,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);
CREATE INDEX idx_totp_challenges_user_id ON totp_challenges(user_id);
CREATE INDEX idx_totp_challenges_expires_at ON totp_challenges(expires_at);
```
> Only change vs SQLite: `totp_enabled INTEGER … DEFAULT 0` → `BOOLEAN … DEFAULT FALSE`.
> **Verify** the authnz user model field for `totp_enabled` is a Go `bool`; if the
> repo currently reads/writes 0/1 ints, the postgres repo must use `true/false`.

### `04_add_2fa_tables.down.sql`
Copy the SQLite down verbatim (PG supports `ALTER TABLE … DROP COLUMN`).

### `05_create_short_links.up.sql`
```sql
CREATE TABLE short_links (
    id               TEXT PRIMARY KEY,
    code             TEXT NOT NULL UNIQUE,
    target_url       TEXT NOT NULL,
    created_at       TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    expires_at       TIMESTAMP,
    is_active        BOOLEAN NOT NULL DEFAULT TRUE,
    click_count      INTEGER NOT NULL DEFAULT 0,
    last_accessed_at TIMESTAMP
);

CREATE TABLE short_link_owners (
    code    TEXT NOT NULL PRIMARY KEY REFERENCES short_links(code) ON DELETE CASCADE,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE
);

CREATE INDEX idx_short_links_expires       ON short_links(expires_at) WHERE expires_at IS NOT NULL;
CREATE INDEX idx_short_link_owners_user_id ON short_link_owners(user_id);
```
> Changes vs SQLite: `DATETIME` → `TIMESTAMP`; `BOOLEAN … DEFAULT 1` → `DEFAULT TRUE`.
> The partial index `WHERE expires_at IS NOT NULL` is valid PG and kept.

### `05_create_short_links.down.sql`
Copy the SQLite down verbatim.

---

## 8. Dependencies — `go.mod`

- Add `github.com/lib/pq` (run `go get github.com/lib/pq`).
- `github.com/golang-migrate/migrate/v4/database/postgres` ships with the already-present
  migrate v4.19.1 — it just needs the blank import in `postgres.go` (§4.1).
- Run `go mod tidy`.

---

## 9. Local dev tooling — `docker-compose.yml`

Create at repo root (OTel collector compose was previously deferred — if/when
added, put it in this same file):

```yaml
services:
  postgres:
    image: postgres:15
    container_name: all-in-one-postgres
    environment:
      POSTGRES_USER: allinone
      POSTGRES_PASSWORD: allinone
      POSTGRES_DB: allinone
    ports:
      - "5432:5432"
    volumes:
      - allinone_pgdata:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U allinone -d allinone"]
      interval: 5s
      timeout: 5s
      retries: 5

volumes:
  allinone_pgdata:
```

Run flow:
1. `docker compose up -d postgres`
2. set `ALLINONE_STORAGE_TYPE=postgres` (+ creds) or edit `config/config.yml`
3. `go run main.go all-in-one db migrate up`
4. `go run main.go all-in-one server`

---

## 10. Docs — `docs/metrics.md`

otelsql emits identical connection-pool metrics for both backends; only the
`db.system` resource attribute differs (`sqlite` vs `postgresql`). Add a line
noting the Postgres attribute value so the metrics doc stays accurate (CLAUDE.md
requires checking `metrics.md` on metric changes). No new metric names are added.

---

## 11. Critical files (summary)

- **Config:** `internal/config/config.go`, `config/config.yml`
- **Central storage:** `internal/storage/storage.go`, `internal/storage/postgres.go` (new), `internal/storage/sqlite.go` (FK flag)
- **Wiring:** `cmd/all-in-one/server/server.go`; `internal/{listing,chat,shortener}/service/service.go`
- **Factories:** `internal/{authnz,listing,chat,shortener}/repository/factory.go` (+ chat `sqlite.go`)
- **New repo packages:** `internal/{authnz,listing,chat,shortener}/repository/postgres/**`
- **Remove self-opened conns:** `internal/listing/repository/sqlite/storage.go`, `internal/shortener/repository/sqlite/storage.go`
- **Migrations:** `db/migrations/postgres/**` (5 up + 5 down)
- **Deps/tooling/docs:** `go.mod`, `docker-compose.yml` (new), `docs/metrics.md`

---

## 12. Verification

1. `docker compose up -d postgres` and wait for healthy.
2. Configure PG: `ALLINONE_STORAGE_TYPE=postgres` + `ALLINONE_STORAGE_POSTGRES_*`
   (or edit config.yml).
3. `go build ./...` and `go vet ./...` — must be clean.
4. Migrate: `go run main.go all-in-one db migrate up` — confirm all 5 versions
   apply and `schema_migrations` shows version 5, dirty=false. Then test rollback:
   `go run main.go all-in-one db migrate down` works.
5. `go run main.go all-in-one server` and exercise end-to-end:
   - **authnz:** register + login; enroll + verify 2FA (exercises users/sessions/
     recovery_codes/totp_challenges and the `totp_enabled` boolean).
   - **listing:** create topic + item (verifies `RETURNING id` yields non-zero IDs),
     list, update, delete.
   - **chat:** create session, send message, create + accept invite (verifies FK
     cascade + transactions).
   - **shortener:** create link, resolve `/r/{code}`, confirm the `is_active = TRUE`
     filter works and deleting a link cascades `short_link_owners`.
6. **Regression:** set `type: sqlite`, restart, smoke each app — confirm no behavior
   change and SQLite FK cascade still holds (shortener) after the central
   `?_foreign_keys=on` move.
7. `go test ./...` against SQLite. Shortener integration tests use SQLite `PRAGMA`
   and stay SQLite-scoped; do not run them against PG.

## 13. Risks / watch-outs
- **FK ordering:** PG enforces FKs immediately, so migration 01 must create
  `users` before `topics`/`sessions` (handled above).
- **`totp_enabled` type:** confirm the authnz model/repo treats it as `bool`, not
  `int`, before wiring the postgres repo (§7 migration 04 note).
- **Timestamp semantics:** columns are `TIMESTAMP` (no TZ); repos insert
  `time.Time` in UTC — keep insertion UTC to avoid drift.
- **Multiple central-storage instantiations:** chat currently calls
  `storage.NewStorage` internally; after §5.2 only `server.go` constructs storage
  once. Grep for stray `storage.NewStorage(` calls to ensure a single owner.
