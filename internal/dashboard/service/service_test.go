package service

import (
	"context"
	"errors"
	"testing"

	chatModel "github.com/all-in-one/internal/chat/model"
	chatRepo "github.com/all-in-one/internal/chat/repository"
	listingModel "github.com/all-in-one/internal/listing/model"
	listingRepo "github.com/all-in-one/internal/listing/repository"
	"github.com/all-in-one/internal/rbac"
	shortenerModel "github.com/all-in-one/internal/shortener/model"
	shortenerRepo "github.com/all-in-one/internal/shortener/repository"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Fakes embed the repository interface and override only the single method the
// aggregator calls; the rest are promoted from the (nil) embedded interface and
// are never invoked in these tests.

type fakeTopicRepo struct {
	listingRepo.TopicRepository
	topics []listingModel.Topic
	err    error
}

func (f fakeTopicRepo) GetAll(_ context.Context, _ uuid.UUID) ([]listingModel.Topic, error) {
	return f.topics, f.err
}

type fakeListingStorage struct {
	listingRepo.Storage
	repo listingRepo.TopicRepository
}

func (f fakeListingStorage) TopicRepo() listingRepo.TopicRepository { return f.repo }

type fakeSessionRepo struct {
	chatRepo.SessionRepository
	sessions []chatModel.ChatSession
	err      error
}

func (f fakeSessionRepo) GetAllByUserID(_ context.Context, _ uuid.UUID) ([]chatModel.ChatSession, error) {
	return f.sessions, f.err
}

type fakeInviteRepo struct {
	chatRepo.InviteRepository
	invites []chatModel.ChatInvite
	err     error
}

func (f fakeInviteRepo) GetPendingByInviteeID(_ context.Context, _ uuid.UUID) ([]chatModel.ChatInvite, error) {
	return f.invites, f.err
}

type fakeChatStorage struct {
	chatRepo.Storage
	session chatRepo.SessionRepository
	invite  chatRepo.InviteRepository
}

func (f fakeChatStorage) SessionRepo() chatRepo.SessionRepository { return f.session }
func (f fakeChatStorage) InviteRepo() chatRepo.InviteRepository   { return f.invite }

type fakeShortLinkRepo struct {
	shortenerRepo.ShortLinkRepository
	total uint32
	err   error
}

func (f fakeShortLinkRepo) ListByOwner(_ context.Context, _ string, _, _ uint32) ([]shortenerModel.ShortLink, uint32, error) {
	return nil, f.total, f.err
}

type fakeShortenerStorage struct {
	shortenerRepo.Storage
	repo shortenerRepo.ShortLinkRepository
}

func (f fakeShortenerStorage) ShortLinkRepo() shortenerRepo.ShortLinkRepository { return f.repo }

type fakeResolver struct {
	isAdmin  bool
	features []string
	err      error
}

func (f fakeResolver) EffectiveFeatures(_ context.Context, _ uuid.UUID) (bool, uuid.UUID, string, []string, error) {
	return f.isAdmin, uuid.Nil, "", f.features, f.err
}

func newTestAggregator(resolver AccessResolver, listing listingRepo.Storage) *aggregator {
	return &aggregator{
		listing:   listing,
		chat:      fakeChatStorage{session: fakeSessionRepo{sessions: make([]chatModel.ChatSession, 2)}, invite: fakeInviteRepo{invites: make([]chatModel.ChatInvite, 1)}},
		shortener: fakeShortenerStorage{repo: fakeShortLinkRepo{total: 5}},
		resolver:  resolver,
	}
}

func okListing() listingRepo.Storage {
	return fakeListingStorage{repo: fakeTopicRepo{topics: make([]listingModel.Topic, 3)}}
}

func TestAggregator_Summary(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()

	t.Run("admin sees every section with correct counts", func(t *testing.T) {
		agg := newTestAggregator(fakeResolver{isAdmin: true}, okListing())

		got, err := agg.Summary(ctx, userID)
		require.NoError(t, err)

		require.NotNil(t, got.Listing)
		assert.Equal(t, 3, got.Listing.Topics)
		require.NotNil(t, got.Chat)
		assert.Equal(t, 2, got.Chat.Conversations)
		assert.Equal(t, 1, got.Chat.PendingInvites)
		require.NotNil(t, got.Shortener)
		assert.Equal(t, 5, got.Shortener.Links)
	})

	t.Run("regular user sees only granted features", func(t *testing.T) {
		agg := newTestAggregator(fakeResolver{features: []string{rbac.FeatureShortener}}, okListing())

		got, err := agg.Summary(ctx, userID)
		require.NoError(t, err)

		assert.Nil(t, got.Listing)
		assert.Nil(t, got.Chat)
		require.NotNil(t, got.Shortener)
		assert.Equal(t, 5, got.Shortener.Links)
	})

	t.Run("resolver failure propagates", func(t *testing.T) {
		agg := newTestAggregator(fakeResolver{err: errors.New("resolver down")}, okListing())

		_, err := agg.Summary(ctx, userID)
		require.Error(t, err)
	})

	t.Run("count failure is best-effort: section present with zero count", func(t *testing.T) {
		badListing := fakeListingStorage{repo: fakeTopicRepo{err: errors.New("db down")}}
		agg := newTestAggregator(fakeResolver{isAdmin: true}, badListing)

		got, err := agg.Summary(ctx, userID)
		require.NoError(t, err)

		require.NotNil(t, got.Listing)
		assert.Equal(t, 0, got.Listing.Topics)
	})
}
