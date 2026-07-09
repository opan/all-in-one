package repository

import (
	"github.com/all-in-one/internal/config"
	"github.com/all-in-one/internal/ratelimit/repository/postgres"
	"github.com/all-in-one/internal/ratelimit/repository/sqlite"
	"github.com/jmoiron/sqlx"
)

func NewRepo(db *sqlx.DB, config config.Config) (Storage, error) {
	switch config.Storage.Type {
	case "sqlite":
		s := sqlite.NewStorage(db, config)
		return &storeAdapter{ruleRepo: s.RuleRepo(), trx: s}, nil
	case "postgres":
		s := postgres.NewStorage(db, config)
		return &storeAdapter{ruleRepo: s.RuleRepo(), trx: s}, nil
	default:
		panic("unsupported storage type: " + config.Storage.Type)
	}
}
