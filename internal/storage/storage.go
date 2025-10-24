package storage

import (
	"fmt"

	"github.com/all-in-one/internal/config"
	"github.com/all-in-one/internal/listing"
	"github.com/rs/zerolog"
)

type storage struct {
	config config.Config
	log    zerolog.Logger
}

type Storage interface {
	// Define methods that the storage should implement
}

func NewStorage(config config.Config, log zerolog.Logger) Storage {
	return &storage{
		config: config,
		log:    log,
	}
}

func (s *storage) SetupStorage() (listing.Service, error) {
	var listingSvc *listing.Service
	var err error

	c := s.config

	switch c.Storage.Type {
	case "sqlite":
		s.log.Info().Msgf("Using SQLite storage at path: %s", c.Storage.SQLite.DBPath)

		listingSvc, err = listing.NewSQLiteService(c.Storage.SQLite.DBPath)
		if err != nil {
			s.log.Error().Err(err).Msg("Failed to initialize SQLite storage")
		}
		defer func() {
			if err := listingSvc.Close(); err != nil {
				s.log.Error().Err(err).Msg("Error closing SQLite storage")
			}
		}()
	case "memory":
		s.log.Info().Msg("Initializing in-memory storage")
		listingSvc = listing.NewMemoryService()
	default:
		s.log.Error().Msgf("Unknown storage type. Supported types: memory, sqlite")
		err = fmt.Errorf("unknown storage type: %s", c.Storage.Type)
	}
	return *listingSvc, err
}

func (s *storage) GetStorageType() string {
	return s.config.Storage.Type
}
