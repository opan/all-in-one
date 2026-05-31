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

	return &handlerMetrics{
		loginsTotal:        loginsTotal,
		registrationsTotal: registrationsTotal,
		twoFAVerifications: twoFAVerifications,
		twoFAStateChanges:  twoFAStateChanges,
	}
}
