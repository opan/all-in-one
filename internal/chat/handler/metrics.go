package handler

import (
	"github.com/all-in-one/internal/observability"
	"go.opentelemetry.io/otel/metric"
)

type handlerMetrics struct {
	sessionsCreated   metric.Int64Counter
	sessionsDeleted   metric.Int64Counter
	messagesPersisted metric.Int64Counter
	invitesSent       metric.Int64Counter
	invitesResponded  metric.Int64Counter
	invitesCancelled  metric.Int64Counter
}

func newHandlerMetrics() *handlerMetrics {
	m := observability.Meter("chat")

	sessionsCreated, _ := m.Int64Counter("aio.chat.sessions.created",
		metric.WithDescription("Number of chat sessions created successfully"),
	)
	sessionsDeleted, _ := m.Int64Counter("aio.chat.sessions.deleted",
		metric.WithDescription("Number of chat sessions deleted successfully"),
	)
	messagesPersisted, _ := m.Int64Counter("aio.chat.messages.persisted",
		metric.WithDescription("Number of chat messages persisted successfully"),
	)
	invitesSent, _ := m.Int64Counter("aio.chat.invites.sent",
		metric.WithDescription("Number of chat invites sent successfully"),
	)
	invitesResponded, _ := m.Int64Counter("aio.chat.invites.responded",
		metric.WithDescription("Number of chat invite responses by result"),
	)
	invitesCancelled, _ := m.Int64Counter("aio.chat.invites.cancelled",
		metric.WithDescription("Number of chat invites cancelled successfully"),
	)

	return &handlerMetrics{
		sessionsCreated:   sessionsCreated,
		sessionsDeleted:   sessionsDeleted,
		messagesPersisted: messagesPersisted,
		invitesSent:       invitesSent,
		invitesResponded:  invitesResponded,
		invitesCancelled:  invitesCancelled,
	}
}
