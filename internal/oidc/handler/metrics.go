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
	clients   metric.Int64Counter
	status    metric.Int64Counter
}

func newHandlerMetrics() *handlerMetrics {
	m := observability.Meter("oidc")
	completed, _ := m.Int64Counter("aio.oidc.login.completed",
		metric.WithDescription("Logins into other apps completed through aio's login page"))
	failed, _ := m.Int64Counter("aio.oidc.login.failed",
		metric.WithDescription("Logins into other apps that aio's login page could not complete"))
	clients, _ := m.Int64Counter("aio.oidc.clients.changed",
		metric.WithDescription("Admin changes to the apps allowed to log in through aio"))
	status, _ := m.Int64Counter("aio.oidc.user_status",
		metric.WithDescription("Admin status checks apps made for their users"))
	return &handlerMetrics{completed: completed, failed: failed, clients: clients, status: status}
}

func (m *handlerMetrics) loginCompleted(ctx context.Context, clientID string) {
	m.completed.Add(ctx, 1, metric.WithAttributes(attribute.String("client", clientID)))
}

func (m *handlerMetrics) loginFailed(ctx context.Context, reason string) {
	m.failed.Add(ctx, 1, metric.WithAttributes(attribute.String("reason", reason)))
}

func (m *handlerMetrics) clientChanged(ctx context.Context, action string) {
	m.clients.Add(ctx, 1, metric.WithAttributes(attribute.String("action", action)))
}

// userStatus records a status check; client is "" when the caller's
// credentials were rejected, so unverified ids never become label values.
func (m *handlerMetrics) userStatus(ctx context.Context, client, result string) {
	m.status.Add(ctx, 1, metric.WithAttributes(attribute.String("client", client), attribute.String("result", result)))
}
