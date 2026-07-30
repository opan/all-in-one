package service

import (
	"context"

	chatRepo "github.com/all-in-one/internal/chat/repository"
	"github.com/all-in-one/internal/dashboard/handler"
	"github.com/all-in-one/internal/dashboard/model"
	listingRepo "github.com/all-in-one/internal/listing/repository"
	"github.com/all-in-one/internal/logging"
	"github.com/all-in-one/internal/rbac"
	shortenerRepo "github.com/all-in-one/internal/shortener/repository"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/rs/zerolog"
)

// AccessResolver reports a user's effective RBAC state. Satisfied by
// *rbac/service.Resolver.
type AccessResolver interface {
	EffectiveFeatures(ctx context.Context, userID uuid.UUID) (isAdmin bool, groupID uuid.UUID, groupName string, featureKeys []string, err error)
}

// Service exposes the dashboard summary endpoint. It is a pure aggregator over
// the already-constructed app storages (no DB handle of its own).
type Service struct {
	Handler *handler.Handler
}

func NewService(listing listingRepo.Storage, chat chatRepo.Storage, shortener shortenerRepo.Storage, resolver AccessResolver, log zerolog.Logger) *Service {
	agg := &aggregator{
		listing:   listing,
		chat:      chat,
		shortener: shortener,
		resolver:  resolver,
	}
	return &Service{Handler: handler.NewHandler(agg, log)}
}

func (s *Service) RegisterAuthenticatedRoutes(router *mux.Router) {
	s.Handler.RegisterAuthenticatedRoutes(router)
}

// aggregator implements handler.SummaryProvider by counting each app's rows for
// the requesting user, gated by the features the RBAC resolver reports.
type aggregator struct {
	listing   listingRepo.Storage
	chat      chatRepo.Storage
	shortener shortenerRepo.Storage
	resolver  AccessResolver
}

// Summary gathers per-feature counts. Feature access is resolved once up front;
// individual counts are best-effort — a per-app failure is logged and its count
// left at 0 rather than failing the whole dashboard.
func (a *aggregator) Summary(ctx context.Context, userID uuid.UUID) (model.Summary, error) {
	log := logging.GetLoggerFromContext(ctx)

	isAdmin, _, _, features, err := a.resolver.EffectiveFeatures(ctx, userID)
	if err != nil {
		return model.Summary{}, err
	}

	can := func(key string) bool {
		if isAdmin {
			return true
		}
		for _, f := range features {
			if f == key {
				return true
			}
		}
		return false
	}

	var summary model.Summary

	if can(rbac.FeatureListing) {
		topics, err := a.listing.TopicRepo().GetAll(ctx, userID)
		if err != nil {
			log.Error().Err(err).Msg("dashboard: failed to count listing topics")
		}
		summary.Listing = &model.ListingStats{Topics: len(topics)}
	}

	if can(rbac.FeatureChat) {
		stats := &model.ChatStats{}
		sessions, err := a.chat.SessionRepo().GetAllByUserID(ctx, userID)
		if err != nil {
			log.Error().Err(err).Msg("dashboard: failed to count chat sessions")
		}
		stats.Conversations = len(sessions)

		invites, err := a.chat.InviteRepo().GetPendingByInviteeID(ctx, userID)
		if err != nil {
			log.Error().Err(err).Msg("dashboard: failed to count pending invites")
		}
		stats.PendingInvites = len(invites)
		summary.Chat = stats
	}

	if can(rbac.FeatureShortener) {
		_, total, err := a.shortener.ShortLinkRepo().ListByOwner(ctx, userID.String(), 1, 1)
		if err != nil {
			log.Error().Err(err).Msg("dashboard: failed to count short links")
		}
		summary.Shortener = &model.ShortenerStats{Links: int(total)}
	}

	return summary, nil
}
