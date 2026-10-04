package handler

import (
	"context"

	"github.com/all-in-one/internal/observability"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

type handlerMetrics struct {
	completed metric.Int64Counter
	failed    metric.Int64Counter
}

func newHandlerMetrics() *handlerMetrics {
	m := observability.Meter("oidc")
	completed, _ := m.Int64Counter("aio.oidc.login.completed",
		metric.WithDescription("Logins into other apps completed through aio's login page"))
	failed, _ := m.Int64Counter("aio.oidc.login.failed",
		metric.WithDescription("Logins into other apps that aio's login page could not complete"))
	return &handlerMetrics{completed: completed, failed: failed}
}

func (m *handlerMetrics) loginCompleted(ctx context.Context, clientID string) {
	m.completed.Add(ctx, 1, metric.WithAttributes(attribute.String("client", clientID)))
}

func (m *handlerMetrics) loginFailed(ctx context.Context, reason string) {
	m.failed.Add(ctx, 1, metric.WithAttributes(attribute.String("reason", reason)))
}
