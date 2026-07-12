package postgres

import (
	"context"

	"github.com/all-in-one/internal/config"
	"github.com/all-in-one/internal/query"
	"github.com/jmoiron/sqlx"
)

type storage struct {
	db          *sqlx.DB
	ruleRepo    *ruleRepository
	counterRepo *counterRepository
}

func NewStorage(db *sqlx.DB, config config.Config) *storage {
	return &storage{
		db:          db,
		ruleRepo:    newRuleRepository(db),
		counterRepo: newCounterRepository(db),
	}
}

func (s *storage) RuleRepo() *ruleRepository       { return s.ruleRepo }
func (s *storage) CounterRepo() *counterRepository { return s.counterRepo }

func (s *storage) CreateTrx(ctx context.Context) (query.QueryOptions, error) {
	return createTrx(ctx, s.db)
}

func (s *storage) Close() error {
	return s.db.Close()
}
