package handler

import (
	"github.com/all-in-one/internal/observability"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

type handlerMetrics struct {
	configChanged metric.Int64Counter
}

func newHandlerMetrics() *handlerMetrics {
	m := observability.Meter("ratelimit")

	configChanged, _ := m.Int64Counter("aio.ratelimit.config.changed",
		metric.WithDescription("Number of admin rate limit config changes"),
	)

	return &handlerMetrics{
		configChanged: configChanged,
	}
}

func configChangedAttr(target, action string) metric.AddOption {
	return metric.WithAttributes(
		attribute.String("target", target),
		attribute.String("action", action),
	)
}
