package repository

type sqliteStoreAdapter struct {
	userRepo    UserRepository
	sessionRepo SessionRepository
	storage     baseStorage
}

func (s *sqliteStoreAdapter) UserRepo() UserRepository {
	return s.userRepo
}

func (s *sqliteStoreAdapter) SessionRepo() SessionRepository {
	return s.sessionRepo
}

func (s *sqliteStoreAdapter) Close() error {
	return s.storage.Close()
}
