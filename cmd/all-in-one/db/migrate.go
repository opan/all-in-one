package db

import (
	"fmt"

	"github.com/all-in-one/internal/config"
	"github.com/all-in-one/internal/storage"
	"github.com/rs/zerolog"
)

type MigrateOpts struct {
	Config config.Config
	Logger zerolog.Logger
}

func RunMigrateUp(opts MigrateOpts) error {
	log := opts.Logger

	log.Info().Msg("Initializing storage...")
	store, err := storage.NewStorage(opts.Config)
	if err != nil {
		return fmt.Errorf("failed to create storage: %w", err)
	}

	log.Info().Msg("Running migrations up...")
	if err := store.MigrateUp(); err != nil {
		return fmt.Errorf("migrate up failed: %w", err)
	}

	log.Info().Msg("Migrations applied successfully")
	return nil
}

func RunMigrateDown(opts MigrateOpts, steps int) error {
	log := opts.Logger

	log.Info().Msg("Initializing storage...")
	store, err := storage.NewStorage(opts.Config)
	if err != nil {
		return fmt.Errorf("failed to create storage: %w", err)
	}

	if steps == 0 {
		log.Info().Msg("Rolling back all migrations...")
	} else {
		log.Info().Int("steps", steps).Msg("Rolling back migrations...")
	}

	if err := store.MigrateDown(steps); err != nil {
		return fmt.Errorf("migrate down failed: %w", err)
	}

	log.Info().Msg("Migrations rolled back successfully")
	return nil
}
