package postgres

import (
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/rs/zerolog"
)

type Storage struct {
	DB          *sqlx.DB
	SessionRepo *sessionRepository
	MessageRepo *messageRepository
	InviteRepo  *inviteRepository
	Log         zerolog.Logger
}

func NewStorage(db *sqlx.DB, log zerolog.Logger) *Storage {
	return &Storage{
		DB:          db,
		SessionRepo: newSessionRepository(db, log),
		MessageRepo: newMessageRepository(db, log),
		InviteRepo:  newInviteRepository(db, log),
		Log:         log,
	}
}
