# OpenTelemetry Integration Plan

**Date**: 2026-05-24
**Status**: 📋 Planning Phase

## Goal

Integrate OpenTelemetry (OTel) into the all-in-one app for traces, log correlation, and metrics. Roll out incrementally in 4 phases.

---

## Codebase findings

**Bootstrap and routing (single binary, single mux):**
- `cmd/all-in-one/main.go` → `cmd/all-in-one/command/command.go` → `cmd/all-in-one/server/server.go`. Cobra `server` subcommand loads config, builds zerolog, and calls `server.Start()`.
- A single `mux.NewRouter()` is created in `server/server.go:98`. Public subrouter, JWT-authenticated subrouter, and a SPA fallback are wired off of it. Apps (`listing`, `authnz`, `chat`, `shortener`) each expose `RegisterRoutes` / `RegisterAuthenticatedRoutes` methods that bind handlers to the shared mux. CORS wraps the router at the outer edge (line 142-149). One HTTP server on `:8080`, not separate binaries per app.

**Middleware chain (request-scoped logger pattern):**
- `internal/http/http.go:LoggingMiddleware` generates a `request_id`, stuffs a child zerolog logger into `context.Context` under `logging.LoggerKey`, then logs `method/path/ip/duration_ms` on completion. All handlers downstream call `logging.GetLoggerFromContext(ctx)` to pick up that logger — this is the single place where per-request observability metadata is attached today.
- JWT middleware (`internal/authnz/middleware/jwt.go`) re-uses that ctx logger and attaches `auth.UserClaims` to ctx.
- The shortener has its own rate-limit middleware (`internal/shortener/middleware/ratelimit.go`).

**Database access:**
- `internal/storage/sqlite.go` opens `sqlx.Open("sqlite3", path)` and exposes `*sqlx.DB`. Each app's repository is constructed against that DB via `internal/<app>/repository/factory.go` and `sqlite/*_repository.go`.
- Repositories use `db.SelectContext` / `db.GetContext` / `db.ExecContext` throughout — **context already flows to the driver**, which is the key prerequisite for SQL instrumentation. No driver wrapping today.

**WebSocket (chat):**
- One long-lived WS endpoint `/api/v1/ws` (chat handler `HandleWebSocket`). Each connected user gets a `Client` with a per-connection `context.Context`+cancel and a `ReadPump`/`WritePump` goroutine pair. A central `Hub` (`internal/chat/websocket/hub.go`) dispatches `BroadcastMessage` events. Messages have a `Type` field (`message`, `typing`, `join`, `leave`, `invite_*`, `error`).

**Frontend:**
- SvelteKit + adapter-static (statically served by Go in production). API client is `web/src/lib/api.ts` (single `fetch` wrapper with cookie auth + 401 refresh). WebSocket client is `web/src/lib/websocket-client.ts`. No source-level tracing today.

**Observability today:** none beyond zerolog structured logs and a basic `/api/v1/health` JSON endpoint. No metrics endpoint, no tracing, no docker-compose, no collector. Deployments use Kubernetes manifests under `deployments/`.

---

## Scope recommendations

| Decision | Recommendation |
|---|---|
| Pillars | **Traces first**, then logs correlation, then metrics. Logs and traces give 90% of value; metrics last. |
| Exporter | **OTLP/HTTP** to a local **OTel Collector** (port 4318). Collector fans out to Jaeger (traces) and optionally Prometheus (metrics). HTTP, not gRPC: smaller dep tree, easier to debug, fine at this scale. |
| Frontend | **Backend only** for phases 1–3. Add browser SDK in phase 4 only if you need to correlate user-perceived latency. |
| Logs | **Keep zerolog**, add `trace_id` and `span_id` fields by enriching the per-request logger in middleware. Migrating zerolog → OTel logs SDK is not worth it — zerolog's API is too embedded, and the OTel Go logs SDK is the newest pillar. |
| Local dev | **docker-compose** with three services: `otel-collector`, `jaeger` (all-in-one image, UI at :16686), and `prometheus` (added in phase 3). One file, opt-in via `docker compose up -f docker-compose.otel.yml`. |
| Sampling | **ParentBased(TraceIDRatioBased(1.0))** in dev, **0.1** default in prod, override via env. Always sample on error (use tail-based sampling in the collector later if needed). |

---

## Phase 1 — Backend traces (HTTP + DB)

### Dependencies (add to `go.mod`)
- `go.opentelemetry.io/otel`
- `go.opentelemetry.io/otel/sdk`
- `go.opentelemetry.io/otel/sdk/trace`
- `go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp`
- `go.opentelemetry.io/otel/semconv/v1.26.0`
- `go.opentelemetry.io/contrib/instrumentation/github.com/gorilla/mux/otelmux`
- `go.opentelemetry.io/contrib/instrumentation/database/sql/otelsql` (wraps any `database/sql` driver including the sqlite3 driver, and works with sqlx via `sqlx.NewDb`)
- `go.opentelemetry.io/otel/propagation`

(Run `go mod tidy && go mod vendor` afterwards — vendor is committed.)

### New file: `internal/observability/otel.go`
Create a small bootstrap package that exposes:
- `Init(ctx, cfg) (shutdown func(context.Context) error, err error)` — builds a `Resource` with `service.name`, `service.version`, `deployment.environment`; constructs OTLP/HTTP exporter; constructs `TracerProvider` with batch span processor + sampler; sets global `otel.SetTracerProvider` and `otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))`; returns a shutdown function.
- `Tracer(name string) trace.Tracer` — thin wrapper for callers.

Keep this isolated from `internal/logging` so it can be reused if you ever split binaries.

### `internal/config/config.go` additions

```go
Telemetry TelemetryConfig `mapstructure:"telemetry"`
```

Where `TelemetryConfig` has fields: `Enabled bool`, `ServiceName string` (default `"all-in-one"`), `ServiceVersion string`, `Environment string`, `OTLPEndpoint string` (default `"localhost:4318"`), `OTLPInsecure bool` (default `true`), `SampleRatio float64` (default `1.0`).
Set viper defaults; bind env vars (`ALLINONE_TELEMETRY_OTLP_ENDPOINT`, etc.).

### `config/config.yml` additions

```yaml
telemetry:
  enabled: false
  service_name: "all-in-one"
  service_version: "1.0.0"
  environment: "local"
  otlp_endpoint: "localhost:4318"
  otlp_insecure: true
  sample_ratio: 1.0
```

### `cmd/all-in-one/server/server.go` changes
1. Right after the logger is in scope and before `storage.NewStorage`, call `observability.Init(ctx, cfg.Telemetry)`. Defer the returned `shutdown(...)` with a 5-second timeout context. Bail on init error only if `cfg.Telemetry.Enabled` (log a warning otherwise so dev without a collector still works).
2. Replace `r := mux.NewRouter()` with `r := mux.NewRouter(); r.Use(otelmux.Middleware(cfg.Telemetry.ServiceName, otelmux.WithFilter(skipHealthAndStatic)))`. This produces one server span per request and pulls in `traceparent` headers if present.
3. Order matters — **`otelmux` middleware must be registered before `h.LoggingMiddleware`** so the logging middleware can read the span context out of `r.Context()`.
4. Health check (`/api/v1/health`) and the SPA file server should be filtered out via `otelmux.WithFilter` to avoid noise.

### Database instrumentation
In `internal/storage/sqlite.go`:
- Replace `sqlx.Open("sqlite3", path)` with:
  - `db, err := otelsql.Open("sqlite3", path, otelsql.WithAttributes(semconv.DBSystemSqlite))`
  - `sqlxDB := sqlx.NewDb(db, "sqlite3")`
- Also call `otelsql.RegisterDBStatsMetrics(db, ...)` later in phase 3 for connection-pool metrics.
- All existing `SelectContext`/`GetContext`/`ExecContext` calls automatically get a child span per query. The `ctx` already carries the server span, so the linkage is automatic.

### Outcome of phase 1
Every HTTP request produces: `HTTP <method> <route>` server span → N `sql.query` child spans. Visible in Jaeger UI. Zero handler changes required.

---

## Phase 2 — Log correlation + WebSocket tracing

### Log correlation
Modify `internal/http/http.go:LoggingMiddleware`:
- After `ctx := r.Context()`, extract `spanCtx := trace.SpanContextFromContext(ctx)`.
- If `spanCtx.IsValid()`, add `.Str("trace_id", spanCtx.TraceID().String()).Str("span_id", spanCtx.SpanID().String())` to the per-request logger.
- Keep `request_id` for human readability, but `trace_id` becomes the join key with Jaeger.
- Also enrich the completion log line (`Request completed`) with the same fields, plus `http.status_code` (you'll need to wrap `http.ResponseWriter` to capture the status — small utility worth adding).

### Span attributes / context enrichment
- In `internal/authnz/middleware/jwt.go`, after the token validates, call `trace.SpanFromContext(ctx).SetAttributes(attribute.String("user.id", userClaims.UserID), attribute.String("session.id", userClaims.SessionID))`. **Do not** add the email or username to spans by default (PII).
- Never put the raw JWT or any password into a span attribute.

### WebSocket tracing strategy
**Two spans per connection, one short span per message.** Long-lived spans (entire WS lifetime) are an antipattern in most tracing backends because they pin in-memory state and Jaeger UI struggles with them.

In `internal/chat/handler/message.go:HandleWebSocket`:
1. The HTTP upgrade request is already covered by `otelmux`. Add an attribute `ws.upgrade=true` so it's filterable, and `End` that span as soon as the upgrade completes.
2. Start a **new root span** `chat.websocket.session` with attributes `user.id`, `session.id`, `username`, then immediately end it after recording connection-established. Use it as a link target later.
3. Pass the WS client's `context.Context` down. In `Client.ReadPump`, after `c.conn.ReadMessage()` succeeds, **start a fresh span per inbound message** (`chat.message.receive`) with `messaging.system=websocket`, `chat.message.type=<wsMsg.Type>`, `chat.session.id=<...>`. Pass that ctx to `c.messageHandler`. Span lifetime = message processing + DB write + hub broadcast.
4. For outbound (`WritePump`), wrap each send in a `chat.message.send` span. The hub's `broadcastMessage` should accept a `context.Context` so the producer's trace context propagates — update `Hub.Broadcast` / `BroadcastToUsers` signatures.

### Outcome of phase 2
Logs in stdout carry `trace_id` for easy grep → Jaeger. WS messages produce per-message spans, not 30-minute monsters.

---

## Phase 3 — Metrics (RED for HTTP, USE for DB)

### Dependencies
- `go.opentelemetry.io/otel/sdk/metric`
- `go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp`
- Optionally `go.opentelemetry.io/contrib/instrumentation/runtime` for goroutine/GC metrics.

### Bootstrap
Extend `internal/observability/otel.go` to also construct a `MeterProvider` with OTLP/HTTP metric exporter and a periodic reader (default 15s).

### Instrumentation points
- **HTTP RED:** `otelmux` already records `http.server.request.duration` (Histogram), `http.server.request.body.size`, `http.server.response.body.size`. Free.
- **DB USE:** `otelsql.RegisterDBStatsMetrics(db.DB, otelsql.WithAttributes(...))` records `db.client.connections.usage`, `idle`, `wait_count`, etc.
- **WebSocket gauges:** add a custom meter in the chat package — `chat.websocket.connections.active` (UpDownCounter on Hub register/unregister), `chat.websocket.messages.sent` (Counter), `chat.websocket.messages.received` (Counter, by `type` attribute).
- **Business counters** (optional): `listing.items.created`, `shortener.links.created`, `shortener.links.resolved`, `authnz.logins.{success,failure}`.

### Cardinality guardrails
- **Never** label metrics with `user.id`, `session.id`, `request_id`, IP, or URL with path parameters. Use the mux route template (`/chats/{id}`) — `otelmux` does this automatically. Confirm by inspecting an emitted span and a metric label set.
- For shortener `resolve` metrics, do **not** include the `code` value — count by status_code or by `existed=true|false` instead.

### docker-compose addition for phase 3
Add `prometheus` service that scrapes the collector's `/metrics` endpoint, and optionally `grafana` for dashboards.

---

## Phase 4 — Frontend (optional)

### Dependencies (in `web/`)
- `@opentelemetry/api`
- `@opentelemetry/sdk-trace-web`
- `@opentelemetry/exporter-trace-otlp-http`
- `@opentelemetry/instrumentation-fetch`
- `@opentelemetry/instrumentation-xml-http-request`
- `@opentelemetry/context-zone`
- `@opentelemetry/resources`
- `@opentelemetry/semantic-conventions`

### Wire-up
- New file `web/src/lib/otel.ts`: initialise `WebTracerProvider` with `ZoneContextManager`, register `FetchInstrumentation` with `propagateTraceHeaderCorsUrls: [/.*/]` and `clearTimingResources: true`. Export it as a side-effect from `web/src/routes/+layout.ts` so it loads once.
- Update `web/src/lib/api.ts:apiClient` so it adds `traceparent` automatically — the fetch instrumentation does this for you, you just need to ensure the collector and the Go backend's `otelmux` agree on the `TraceContext` propagator (which is the default).
- WebSocket tracing in the browser: there's no auto-instrumentation — manually start a span per send in `web/src/lib/websocket-client.ts:send()` and inject the trace context into a `headers` field on the WS message envelope. The Go side reads it back. This is more work; skip unless you actually need end-to-end WS visibility.

### CORS
`server.go` CORS config currently allows any header (`AllowedHeaders: []string{"*"}`), so `traceparent` and `tracestate` propagation Just Works. No change needed.

---

## Local dev tooling

Add `docker-compose.otel.yml` and `.docker/otel-collector-config.yml`:

- **otel-collector** (otel/opentelemetry-collector-contrib): receives OTLP/HTTP on 4318. Pipeline: `otlp` → `batch` → `[jaeger, prometheus, logging]`. Use `debug` exporter in dev to dump to stdout.
- **jaeger** (jaegertracing/all-in-one): UI on :16686, OTLP receiver disabled (let the collector forward).
- **prometheus** (prom/prometheus, phase 3): scrapes collector's `:8889/metrics`.
- **grafana** (phase 3, optional): port :3000, datasources for Jaeger + Prometheus.

Run with `docker compose -f docker-compose.otel.yml up -d`. In `config/config.yml` set `telemetry.enabled: true` and `telemetry.otlp_endpoint: localhost:4318`. To toggle in dev: `ALLINONE_TELEMETRY_ENABLED=true go run ./cmd/all-in-one server`.

For "I just want to see traces fast", `jaeger` all-in-one alone (which now has a built-in OTLP receiver on 4318) works without a collector. Recommend that for first-week experimentation, then graduate to the collector when you add metrics.

---

## Risks / gotchas

- **CGO sqlite3 + otelsql:** `otelsql.Open` returns a `*sql.DB`, which you wrap with `sqlx.NewDb` — this preserves the `mattn/go-sqlite3` driver. Verify migrations still work (the migrator in `storage/sqlite.go:newMigrator` reaches in for `s.db.DB` — it should keep working because `sqlx.NewDb` exposes the underlying `*sql.DB`).
- **Vendor directory:** because the repo vendors deps, **every** OTel package and transitive must end up in `vendor/`. Run `go mod tidy && go mod vendor` after adding deps. Don't forget `golang.org/x/net` and `google.golang.org/grpc` transitive bloat if you ever switch from HTTP to gRPC.
- **Span attribute PII:** auth flows touch usernames, emails, JWTs. Audit `internal/authnz/handler/*.go` and `internal/authnz/service/*.go` before sprinkling `SetAttributes`. Use a centralized helper that whitelists allowed attributes.
- **Rate-limit middleware:** `internal/shortener/middleware/ratelimit.go` runs before handler logic — if you want it captured in the span, register `otelmux` outermost (which is the recommendation above) and add an inner `trace.SpanFromContext(ctx).AddEvent("rate_limited")` on the rejection path.
- **WebSocket span explosion:** in a busy chat, every message is a span. The 10% sampling default in prod will tame this, but consider `chat.websocket.messages.received` Counter as the primary signal and trace only a fraction. Use `Sampler` overrides for the chat path if needed (head-based via `samplers.ParentBased` + a custom name-based sampler).
- **Performance overhead:** `otelmux` + `otelsql` cost ~1-3% in microbenchmarks. The batch span processor's default 512-span queue is fine for this app. Don't change defaults until you measure.
- **Sampling on error:** OTel's stable Go SDK doesn't do tail-based sampling client-side. Use the OTel Collector's `tail_sampling` processor when you need "always sample on 5xx" — that's a phase-3+ collector config change, not a code change.
- **WebSocket auth via query parameter:** the JWT is in `?token=...`. **Filter that out** of HTTP span attributes — `otelmux` may capture the full URL. Use `otelmux.WithSpanOptions` and a custom span name formatter, or strip `r.URL.RawQuery` in a thin pre-middleware. Same risk for `r.RemoteAddr` if you treat it as PII.
- **Health check noise:** without filtering, `/api/v1/health` will dominate trace volume. Filter early via `otelmux.WithFilter`.
- **Single binary, multiple apps:** keep `service.name` as one value (`all-in-one`) but use `otel.Tracer("listing")`, `otel.Tracer("chat")`, etc., so spans carry a per-app `otel.library.name` you can group on. When/if you split binaries later, just change `service.name` per binary.

---

## Critical files for implementation

- `/opt/personal/gojek/github/all-in-one/cmd/all-in-one/server/server.go`
- `/opt/personal/gojek/github/all-in-one/internal/http/http.go`
- `/opt/personal/gojek/github/all-in-one/internal/storage/sqlite.go`
- `/opt/personal/gojek/github/all-in-one/internal/config/config.go`
- `/opt/personal/gojek/github/all-in-one/internal/chat/websocket/client.go`
- `/opt/personal/gojek/github/all-in-one/internal/chat/websocket/hub.go`

---

## Next steps

1. Confirm scope: phase 1 only first, or commit to phases 1+2 together?
2. Confirm dev backend: Jaeger all-in-one for week one, then full collector?
3. Create `OTEL_PROGRESS.md` tracker once implementation starts.
4. Implement phase 1, verify a trace in Jaeger, then move to phase 2.
