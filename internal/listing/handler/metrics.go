package handler

import (
	"github.com/all-in-one/internal/observability"
	"go.opentelemetry.io/otel/metric"
)

type handlerMetrics struct {
	topicsCreated metric.Int64Counter
	topicsDeleted metric.Int64Counter
	itemsCreated  metric.Int64Counter
	itemsDeleted  metric.Int64Counter
}

func newHandlerMetrics() *handlerMetrics {
	m := observability.Meter("listing")

	topicsCreated, _ := m.Int64Counter("aio.listing.topics.created",
		metric.WithDescription("Number of topics created successfully"),
	)
	topicsDeleted, _ := m.Int64Counter("aio.listing.topics.deleted",
		metric.WithDescription("Number of topics deleted successfully"),
	)
	itemsCreated, _ := m.Int64Counter("aio.listing.items.created",
		metric.WithDescription("Number of items created successfully"),
	)
	itemsDeleted, _ := m.Int64Counter("aio.listing.items.deleted",
		metric.WithDescription("Number of items deleted successfully"),
	)

	return &handlerMetrics{
		topicsCreated: topicsCreated,
		topicsDeleted: topicsDeleted,
		itemsCreated:  itemsCreated,
		itemsDeleted:  itemsDeleted,
	}
}
