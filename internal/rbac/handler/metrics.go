package handler

import (
	"github.com/all-in-one/internal/observability"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

type handlerMetrics struct {
	groupsChanged     metric.Int64Counter
	userGroupAssigned metric.Int64Counter
	userOverridesSet  metric.Int64Counter
}

func newHandlerMetrics() *handlerMetrics {
	m := observability.Meter("rbac")

	groupsChanged, _ := m.Int64Counter("aio.rbac.groups.changed",
		metric.WithDescription("Number of admin group management actions (created/updated/deleted/features_set)"),
	)
	userGroupAssigned, _ := m.Int64Counter("aio.rbac.user_group.assigned",
		metric.WithDescription("Number of times an admin reassigned a user's group"),
	)
	userOverridesSet, _ := m.Int64Counter("aio.rbac.user_overrides.set",
		metric.WithDescription("Number of times an admin replaced a user's feature overrides"),
	)

	return &handlerMetrics{
		groupsChanged:     groupsChanged,
		userGroupAssigned: userGroupAssigned,
		userOverridesSet:  userOverridesSet,
	}
}

func groupChangeAttr(action string) metric.AddOption {
	return metric.WithAttributes(attribute.String("action", action))
}
