package repository

import (
	"github.com/all-in-one/internal/config"
	"github.com/all-in-one/internal/rbac/repository/postgres"
	"github.com/all-in-one/internal/rbac/repository/sqlite"
	"github.com/jmoiron/sqlx"
)

func NewRepo(db *sqlx.DB, config config.Config) (Storage, error) {
	switch config.Storage.Type {
	case "sqlite":
		s := sqlite.NewStorage(db, config)
		return &storeAdapter{
			featureRepo:      s.FeatureRepo(),
			groupRepo:        s.GroupRepo(),
			groupFeatureRepo: s.GroupFeatureRepo(),
			overrideRepo:     s.OverrideRepo(),
			userGroupRepo:    s.UserGroupRepo(),
			trx:              s,
		}, nil
	case "postgres":
		s := postgres.NewStorage(db, config)
		return &storeAdapter{
			featureRepo:      s.FeatureRepo(),
			groupRepo:        s.GroupRepo(),
			groupFeatureRepo: s.GroupFeatureRepo(),
			overrideRepo:     s.OverrideRepo(),
			userGroupRepo:    s.UserGroupRepo(),
			trx:              s,
		}, nil
	default:
		panic("unsupported storage type: " + config.Storage.Type)
	}
}
