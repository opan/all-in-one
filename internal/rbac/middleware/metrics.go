package middleware

import (
	"github.com/all-in-one/internal/observability"
	"go.opentelemetry.io/otel/metric"
)

type authzMetrics struct {
	accessDenied metric.Int64Counter
}

func newAuthzMetrics() *authzMetrics {
	m := observability.Meter("rbac")

	accessDenied, _ := m.Int64Counter("aio.rbac.access.denied",
		metric.WithDescription("Number of requests denied by RBAC feature/admin gating"),
	)

	return &authzMetrics{
		accessDenied: accessDenied,
	}
}
