package repository

type sqliteStoreAdapter struct {
	userRepo    UserRepository
	sessionRepo SessionRepository
	totpRepo    TOTPRepository
	resetRepo   PasswordResetRepository
	emailRepo   EmailVerificationRepository
	storage     baseStorage
}

func (s *sqliteStoreAdapter) UserRepo() UserRepository {
	return s.userRepo
}

func (s *sqliteStoreAdapter) SessionRepo() SessionRepository {
	return s.sessionRepo
}

func (s *sqliteStoreAdapter) TOTPRepo() TOTPRepository {
	return s.totpRepo
}

func (s *sqliteStoreAdapter) PasswordResetRepo() PasswordResetRepository {
	return s.resetRepo
}

func (s *sqliteStoreAdapter) EmailVerificationRepo() EmailVerificationRepository {
	return s.emailRepo
}

func (s *sqliteStoreAdapter) Close() error {
	if s.storage != nil {
		return s.storage.Close()
	}
	return nil
}
