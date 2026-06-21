package postgres

import (
	"github.com/all-in-one/internal/config"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

type storage struct {
	db          *sqlx.DB
	userRepo    *userRepository
	sessionRepo *sessionRepository
	totpRepo    *totpRepository
}

func NewStorage(db *sqlx.DB, config config.Config) *storage {
	return &storage{
		db:          db,
		userRepo:    newUserRepository(db),
		sessionRepo: newSessionRepository(db),
		totpRepo:    newTOTPRepository(db),
	}
}

func (s *storage) UserRepo() *userRepository    { return s.userRepo }
func (s *storage) SessionRepo() *sessionRepository { return s.sessionRepo }
func (s *storage) TOTPRepo() *totpRepository    { return s.totpRepo }
func (s *storage) Close() error                 { return nil }
