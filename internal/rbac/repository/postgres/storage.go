package postgres

import (
	"context"

	"github.com/all-in-one/internal/config"
	"github.com/all-in-one/internal/query"
	"github.com/jmoiron/sqlx"
)

type storage struct {
	db               *sqlx.DB
	featureRepo      *featureRepository
	groupRepo        *groupRepository
	groupFeatureRepo *groupFeatureRepository
	overrideRepo     *overrideRepository
	userGroupRepo    *userGroupRepository
}

func NewStorage(db *sqlx.DB, config config.Config) *storage {
	return &storage{
		db:               db,
		featureRepo:      newFeatureRepository(db),
		groupRepo:        newGroupRepository(db),
		groupFeatureRepo: newGroupFeatureRepository(db),
		overrideRepo:     newOverrideRepository(db),
		userGroupRepo:    newUserGroupRepository(db),
	}
}

func (s *storage) FeatureRepo() *featureRepository           { return s.featureRepo }
func (s *storage) GroupRepo() *groupRepository               { return s.groupRepo }
func (s *storage) GroupFeatureRepo() *groupFeatureRepository { return s.groupFeatureRepo }
func (s *storage) OverrideRepo() *overrideRepository         { return s.overrideRepo }
func (s *storage) UserGroupRepo() *userGroupRepository       { return s.userGroupRepo }

func (s *storage) CreateTrx(ctx context.Context) (query.QueryOptions, error) {
	return createTrx(ctx, s.db)
}

func (s *storage) Close() error {
	return s.db.Close()
}
