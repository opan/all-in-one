package sqlite

import (
	"github.com/all-in-one/internal/authnz/repository"
	"github.com/all-in-one/internal/config"
	"github.com/jmoiron/sqlx"
)

type storage struct {
	db          *sqlx.DB
	userRepo    *userRepository
	sessionRepo *sessionRepository
}

func NewStorage(db *sqlx.DB, config config.Config) *storage {
	return &storage{
		db:          db,
		userRepo:    newUserRepository(db),
		sessionRepo: newSessionRepository(db),
	}
}

func (s *storage) UserRepo() repository.UserRepository {
	return s.userRepo
}

func (s *storage) SessionRepo() repository.SessionRepository {
	return s.sessionRepo
}

func (s *storage) Close() error {
	return s.db.Close()
}
