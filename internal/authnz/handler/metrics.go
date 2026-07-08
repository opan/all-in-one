package handler

import (
	"github.com/all-in-one/internal/observability"
	"go.opentelemetry.io/otel/metric"
)

type handlerMetrics struct {
	loginsTotal        metric.Int64Counter
	registrationsTotal metric.Int64Counter
	twoFAVerifications metric.Int64Counter
	twoFAStateChanges  metric.Int64Counter
	adminEmailUpdated  metric.Int64Counter
	adminUserBlocked   metric.Int64Counter
	adminUserUnblocked metric.Int64Counter
}

func newHandlerMetrics() *handlerMetrics {
	m := observability.Meter("authnz")

	loginsTotal, _ := m.Int64Counter("aio.authnz.logins.total",
		metric.WithDescription("Number of login attempts by result"),
	)
	registrationsTotal, _ := m.Int64Counter("aio.authnz.registrations.total",
		metric.WithDescription("Number of user registration attempts by result"),
	)
	twoFAVerifications, _ := m.Int64Counter("aio.authnz.2fa.verifications.total",
		metric.WithDescription("Number of 2FA verification attempts by method and result"),
	)
	twoFAStateChanges, _ := m.Int64Counter("aio.authnz.2fa.state_changes.total",
		metric.WithDescription("Number of 2FA state changes by action"),
	)
	adminEmailUpdated, _ := m.Int64Counter("aio.admin.user_email_updated.total",
		metric.WithDescription("Number of times an admin updated a user's email"),
	)
	adminUserBlocked, _ := m.Int64Counter("aio.admin.user_blocked.total",
		metric.WithDescription("Number of times an admin blocked a user"),
	)
	adminUserUnblocked, _ := m.Int64Counter("aio.admin.user_unblocked.total",
		metric.WithDescription("Number of times an admin unblocked a user"),
	)

	return &handlerMetrics{
		loginsTotal:        loginsTotal,
		registrationsTotal: registrationsTotal,
		twoFAVerifications: twoFAVerifications,
		twoFAStateChanges:  twoFAStateChanges,
		adminEmailUpdated:  adminEmailUpdated,
		adminUserBlocked:   adminUserBlocked,
		adminUserUnblocked: adminUserUnblocked,
	}
}
