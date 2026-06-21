package postgres

import (
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/rs/zerolog"
)

type storage struct {
	db        *sqlx.DB
	log       zerolog.Logger
	itemRepo  *itemRepository
	topicRepo *topicRepository
}

func NewFromDB(db *sqlx.DB, log zerolog.Logger) *storage {
	return &storage{
		db:        db,
		log:       log,
		itemRepo:  newItemRepository(db),
		topicRepo: newTopicRepository(db),
	}
}

func (s *storage) ItemRepo() *itemRepository   { return s.itemRepo }
func (s *storage) TopicRepo() *topicRepository { return s.topicRepo }
func (s *storage) Close() error                { return nil }
