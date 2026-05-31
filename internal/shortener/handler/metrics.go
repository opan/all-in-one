package handler

import (
	"github.com/all-in-one/internal/observability"
	"go.opentelemetry.io/otel/metric"
)

type handlerMetrics struct {
	linksCreated  metric.Int64Counter
	linksResolved metric.Int64Counter
	linksDeleted  metric.Int64Counter
	rateLimited   metric.Int64Counter
}

func newHandlerMetrics() *handlerMetrics {
	m := observability.Meter("shortener")

	linksCreated, _ := m.Int64Counter("aio.shortener.links.created",
		metric.WithDescription("Number of short link creation attempts by result"),
	)
	linksResolved, _ := m.Int64Counter("aio.shortener.links.resolved",
		metric.WithDescription("Number of short link resolution attempts by result"),
	)
	linksDeleted, _ := m.Int64Counter("aio.shortener.links.deleted",
		metric.WithDescription("Number of short links deleted successfully"),
	)
	rateLimited, _ := m.Int64Counter("aio.shortener.rate_limited.total",
		metric.WithDescription("Number of requests rejected by the rate limiter by scope"),
	)

	return &handlerMetrics{
		linksCreated:  linksCreated,
		linksResolved: linksResolved,
		linksDeleted:  linksDeleted,
		rateLimited:   rateLimited,
	}
}
