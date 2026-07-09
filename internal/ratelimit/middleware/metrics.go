package middleware

import (
	"github.com/all-in-one/internal/observability"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

type limiterMetrics struct {
	rejected metric.Int64Counter
	errors   metric.Int64Counter
}

func newLimiterMetrics() *limiterMetrics {
	m := observability.Meter("ratelimit")

	rejected, _ := m.Int64Counter("aio.ratelimit.rejected",
		metric.WithDescription("Number of requests rejected by rate limiting"),
	)
	errors, _ := m.Int64Counter("aio.ratelimit.errors",
		metric.WithDescription("Number of rate limit counter/store errors (fail-open)"),
	)

	return &limiterMetrics{
		rejected: rejected,
		errors:   errors,
	}
}

// otelAttr is a one-line shorthand for the metric.WithAttributes(attribute.String(...))
// call every Add() site in this package needs.
func otelAttr(key, value string) metric.AddOption {
	return metric.WithAttributes(attribute.String(key, value))
}
