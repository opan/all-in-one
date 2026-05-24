// Package observability wires OpenTelemetry into the application.
//
// Init configures the global TracerProvider and propagators based on the
// provided TelemetryConfig. When cfg.Enabled is false the global noop
// provider is left in place and a no-op shutdown function is returned, so
// callers can defer the shutdown unconditionally.
package observability

import (
	"context"
	"fmt"
	"time"

	"github.com/all-in-one/internal/config"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

type ShutdownFunc func(context.Context) error

func noopShutdown(context.Context) error { return nil }

func Init(ctx context.Context, cfg config.TelemetryConfig) (ShutdownFunc, error) {
	if !cfg.Enabled {
		return noopShutdown, nil
	}

	res, err := resource.New(ctx,
		resource.WithTelemetrySDK(),
		resource.WithFromEnv(),
		resource.WithAttributes(
			semconv.ServiceName(cfg.ServiceName),
			semconv.ServiceVersion(cfg.ServiceVersion),
			attribute.String("deployment.environment", cfg.Environment),
		),
	)
	if err != nil {
		return noopShutdown, fmt.Errorf("build otel resource: %w", err)
	}

	exporterOpts := []otlptracehttp.Option{
		otlptracehttp.WithEndpoint(cfg.OTLPEndpoint),
	}
	if cfg.OTLPInsecure {
		exporterOpts = append(exporterOpts, otlptracehttp.WithInsecure())
	}

	exporter, err := otlptrace.New(ctx, otlptracehttp.NewClient(exporterOpts...))
	if err != nil {
		return noopShutdown, fmt.Errorf("create otlp trace exporter: %w", err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.ParentBased(
			sdktrace.TraceIDRatioBased(cfg.SampleRatio),
		)),
	)

	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return func(ctx context.Context) error {
		shutdownCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		return tp.Shutdown(shutdownCtx)
	}, nil
}

// Tracer returns a tracer scoped to the given instrumentation name.
// Use observability.Tracer("chat") in app-specific code so spans group by
// otel.library.name when the binary hosts multiple sub-apps.
func Tracer(name string) trace.Tracer {
	return otel.Tracer(name)
}
