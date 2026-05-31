# OpenTelemetry Integration — Progress Tracker

**Started**: 2026-05-24
**Current phase**: Phase 3 — Metrics (DONE)
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

## Phase 3 — Metrics (DONE)
- [x] Add `sdk/metric` + `otlpmetrichttp` deps (`v1.43.0`)
- [x] Add `MetricInterval` to `TelemetryConfig` (config.go + config.yml + viper default + BindEnv)
- [x] Extend `observability.Init` with `MeterProvider` + dual shutdown + `Meter()` helper
- [x] `otelsql.RegisterDBStatsMetrics` in `storage/sqlite.go` (connection-pool metrics)
- [x] WebSocket metrics in `hub.go`: `aio.chat.websocket.connections.active` (gauge), `aio.chat.websocket.messages.received` (counter by type), `aio.chat.websocket.messages.sent` (counter)
- [x] Increment counters in `client.go` (`RecordMessageReceived` in ReadPump, `RecordMessageSent` in WritePump)
- [x] Upgrade `docker-compose.otel.yml` to Collector + Jaeger + Prometheus stack
- [x] Create `.docker/otel-collector-config.yml`
- [x] Create `.docker/prometheus.yml`
- [ ] **PENDING (requires Docker):** run stack, hit endpoints, verify `aio_chat_websocket_connections_active` and `aio_chat_websocket_messages_received_total` in Prometheus UI (:9090)

## Docker Compose (observability stack) — DONE
- [x] `docker-compose.yml` — single file: Collector + Jaeger + Prometheus (`docker compose up -d`)
- [x] Removed `docker-compose.otel.yml` (was redundant duplicate)
- [x] App runs locally with `ALLINONE_TELEMETRY_ENABLED=true go run ./cmd/all-in-one server`

## Phase 5 — Business Metrics (DONE)

All metric names use `aio.` prefix (renders as `aio_` in Prometheus). Counters are incremented in the handler layer at success/failure points.

### Listing
- [x] `aio.listing.topics.created` — counter, incremented on successful topic creation
- [x] `aio.listing.topics.deleted` — counter, incremented on successful topic deletion
- [x] `aio.listing.items.created` — counter, incremented on successful item creation
- [x] `aio.listing.items.deleted` — counter, incremented on successful item deletion

### Authnz
- [x] `aio.authnz.logins.total` — counter with `result` attribute (`success` | `invalid_credentials` | `user_not_found` | `2fa_required`)
- [x] `aio.authnz.registrations.total` — counter with `result` attribute (`success` | `failure`)
- [x] `aio.authnz.2fa.verifications.total` — counter with `method` (`totp` | `recovery_code`) and `result` (`success` | `failure`) attributes
- [x] `aio.authnz.2fa.state_changes.total` — counter with `action` attribute (`enabled` | `disabled`)

### Shortener
- [x] `aio.shortener.links.created` — counter with `result` attribute (`success` | `failure`)
- [x] `aio.shortener.links.resolved` — counter with `result` attribute (`success` | `not_found` | `expired` | `disabled`)
- [x] `aio.shortener.links.deleted` — counter, incremented on successful deletion
- [x] `aio.shortener.rate_limited.total` — counter with `scope` attribute (`create` | `resolve`)

### Chat
- [x] `aio.chat.sessions.created` — counter, incremented on successful chat session creation
- [x] `aio.chat.sessions.deleted` — counter, incremented on successful chat session deletion
- [x] `aio.chat.messages.persisted` — counter, incremented on successful message persistence (REST)
- [x] `aio.chat.invites.sent` — counter, incremented per invite sent (batch-aware)
- [x] `aio.chat.invites.responded` — counter with `result` attribute (`accepted` | `declined`)
- [x] `aio.chat.invites.cancelled` — counter, incremented on successful invite cancellation

### Verification queries (Prometheus at http://localhost:9090)
```
aio_listing_topics_created_total
aio_authnz_logins_total{result="success"}
aio_authnz_logins_total{result="invalid_credentials"}
aio_authnz_2fa_verifications_total{method="totp",result="failure"}
aio_shortener_links_resolved_total{result="not_found"}
aio_shortener_rate_limited_total{scope="create"}
aio_chat_invites_responded_total{result="accepted"}
```

## Phase 4 — Frontend (OPTIONAL, NOT STARTED)
- [ ] Browser SDK in `web/src/lib/otel.ts`
- [ ] Auto-instrument fetch in `api.ts`
