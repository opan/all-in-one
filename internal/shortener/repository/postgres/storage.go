package postgres

import (
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

type storage struct {
	db            *sqlx.DB
	shortLinkRepo *shortLinkRepository
}

func NewFromDB(db *sqlx.DB) *storage {
	return &storage{
		db:            db,
		shortLinkRepo: newShortLinkRepository(db),
	}
}

func (s *storage) ShortLinkRepo() *shortLinkRepository { return s.shortLinkRepo }
func (s *storage) Close() error                        { return nil }
