// Package observability wires OpenTelemetry into the application.
//
// Init configures the global TracerProvider, MeterProvider, and propagators
// based on the provided TelemetryConfig. When cfg.Enabled is false the global
// noop providers are left in place and a no-op shutdown function is returned,
// so callers can defer the shutdown unconditionally.
package observability

import (
	"context"
	"fmt"
	"time"

	"github.com/all-in-one/internal/config"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
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

	// --- Traces ---
	traceExporterOpts := []otlptracehttp.Option{
		otlptracehttp.WithEndpoint(cfg.OTLPEndpoint),
	}
	if cfg.OTLPInsecure {
		traceExporterOpts = append(traceExporterOpts, otlptracehttp.WithInsecure())
	}
	traceExporter, err := otlptrace.New(ctx, otlptracehttp.NewClient(traceExporterOpts...))
	if err != nil {
		return noopShutdown, fmt.Errorf("create otlp trace exporter: %w", err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(traceExporter),
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

	// --- Metrics ---
	metricExporterOpts := []otlpmetrichttp.Option{
		otlpmetrichttp.WithEndpoint(cfg.OTLPEndpoint),
	}
	if cfg.OTLPInsecure {
		metricExporterOpts = append(metricExporterOpts, otlpmetrichttp.WithInsecure())
	}
	metricExporter, err := otlpmetrichttp.New(ctx, metricExporterOpts...)
	if err != nil {
		return noopShutdown, fmt.Errorf("create otlp metric exporter: %w", err)
	}

	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExporter,
			sdkmetric.WithInterval(cfg.MetricInterval),
		)),
		sdkmetric.WithResource(res),
	)
	otel.SetMeterProvider(mp)

	return func(ctx context.Context) error {
		shutdownCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		traceErr := tp.Shutdown(shutdownCtx)
		metricErr := mp.Shutdown(shutdownCtx)
		if traceErr != nil {
			return traceErr
		}
		return metricErr
	}, nil
}

// Tracer returns a tracer scoped to the given instrumentation name.
// Use observability.Tracer("chat") so spans group by otel.library.name.
func Tracer(name string) trace.Tracer {
	return otel.Tracer(name)
}

// Meter returns a meter scoped to the given instrumentation name.
func Meter(name string) metric.Meter {
	return otel.Meter(name)
}
