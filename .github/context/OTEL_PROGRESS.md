# OpenTelemetry Integration — Progress Tracker

**Started**: 2026-05-24
**Current phase**: Phase 1 — Backend traces (HTTP + DB)
**Spec**: [OTEL_IMPLEMENTATION_PLAN.md](./OTEL_IMPLEMENTATION_PLAN.md)

---

## Phase 1 — Backend traces (HTTP + DB)

### Checklist
- [x] Add OTel Go module dependencies (`otel`, `otelmux`, `XSAM/otelsql`, `otlptracehttp`, `propagation`, `semconv v1.26.0`)
- [x] Create `internal/observability/otel.go` bootstrap (Init, Tracer, shutdown)
- [x] Add `TelemetryConfig` struct to `internal/config/config.go` with viper defaults + env bindings
- [x] Add `telemetry:` block to `config/config.yml`
- [x] Wire `observability.Init` into `cmd/all-in-one/server/server.go`
- [x] Register `otelmux.Middleware` before `LoggingMiddleware`; filter `/api/v1/health` and SPA static via `shouldTrace`
- [x] Replace `sqlx.Open("sqlite3", ...)` with `otelsql.Open(...) + sqlx.NewDb(...)` in `internal/storage/sqlite.go`
- [x] Add `docker-compose.otel.yml` with Jaeger all-in-one (OTLP receiver on 4318, UI on 16686)
- [x] Run `go mod tidy && go mod vendor`
- [x] Build and smoke-test: binary builds, server starts cleanly in both modes (disabled / enabled), endpoints respond, span export attempts visible in logs when no collector
- [ ] **PENDING (requires Docker):** run docker-compose, hit endpoints, verify spans appear in Jaeger UI

### Decisions log

| Decision | Choice | Rationale |
|---|---|---|
| Exporter transport | OTLP/HTTP | Smaller dep tree than gRPC, fine at this scale |
| Local dev backend | Jaeger all-in-one (no collector) | Simpler week-one setup; collector added in phase 3 |
| Sampling (default) | `1.0` (always sample) | Dev-friendly; lower in prod via env override |
| `service.name` | `"all-in-one"` | Single binary; per-app distinction via `otel.Tracer("listing")` etc. |
| Logs migration | Keep zerolog | Migration not worth it; trace_id correlation added in phase 2 |
| SQL instrumentation | `github.com/XSAM/otelsql` | The official contrib does not ship a database/sql instrumentation; XSAM/otelsql is the de facto standard |

### Notes
- Vendoring is enabled (`vendor/` dir committed). Must run `go mod vendor` after `go get`.
- sqlite3 driver is imported as `_` in `internal/listing/repository/sqlite/*.go` — driver is globally registered, so `otelsql.Open("sqlite3", ...)` will work.
- Migrator uses `s.db.DB` (the underlying `*sql.DB`) — `sqlx.NewDb(otelsqlDB, "sqlite3")` preserves this access path.

---

## Phase 2 — Log correlation + WebSocket tracing (DONE)
- [x] Enrich `LoggingMiddleware` with `trace_id` / `span_id`
- [x] Wrap `ResponseWriter` to capture status code (`statusResponseWriter` in `internal/http/http.go`)
- [x] Fix completion log to use context logger + add `status_code` field
- [x] Attach `user.id`, `session.id` span attributes in JWT middleware (no PII)
- [x] `HandleWebSocket`: mark otelmux span with `ws.upgrade=true`, `user.id`, `username`
- [x] `ReadPump`: per-message `chat.message.receive` spans with `messaging.system`, `chat.message.type`, `user.id`
- [x] `WritePump`: per-message `chat.message.send` spans linked to producer span via `trace.WithLinks`
- [x] Thread context through Hub: `BroadcastMessage.Ctx`, updated `Broadcast`/`BroadcastToUsers` signatures
- [x] Updated all callers in `message.go` and `invite.go`

## Phase 3 — Metrics (NOT STARTED)
- [ ] Extend bootstrap with `MeterProvider`
- [ ] DB pool metrics via `otelsql.RegisterDBStatsMetrics`
- [ ] WebSocket gauges (active connections, messages by type)
- [ ] Add `prometheus` to docker-compose; collector exposes `:8889/metrics`

## Phase 4 — Frontend (OPTIONAL, NOT STARTED)
- [ ] Browser SDK in `web/src/lib/otel.ts`
- [ ] Auto-instrument fetch in `api.ts`
