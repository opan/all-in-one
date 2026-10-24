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
	passwordResets     metric.Int64Counter
	emailVerifications metric.Int64Counter
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

	passwordResets, _ := m.Int64Counter("aio.authnz.password_resets.total",
		metric.WithDescription("Temporary passwords requested, by source (self, admin) and result"),
	)
	emailVerifications, _ := m.Int64Counter("aio.authnz.email_verifications.total",
		metric.WithDescription("Email verification links sent and opened, by result"),
	)

	return &handlerMetrics{
		loginsTotal:        loginsTotal,
		registrationsTotal: registrationsTotal,
		twoFAVerifications: twoFAVerifications,
		twoFAStateChanges:  twoFAStateChanges,
		adminEmailUpdated:  adminEmailUpdated,
		adminUserBlocked:   adminUserBlocked,
		adminUserUnblocked: adminUserUnblocked,
		passwordResets:     passwordResets,
		emailVerifications: emailVerifications,
	}
}
