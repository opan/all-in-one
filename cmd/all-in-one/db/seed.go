package db

import (
	"context"
	"fmt"

	authnzRepo "github.com/all-in-one/internal/authnz/repository"
	authnzSeed "github.com/all-in-one/internal/authnz/seed"
	"github.com/all-in-one/internal/config"
	listingRepo "github.com/all-in-one/internal/listing/repository"
	listingSeed "github.com/all-in-one/internal/listing/seed"
	"github.com/all-in-one/internal/storage"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

type Opts struct {
	Config config.Config
	Logger zerolog.Logger
}

// Run executes the database seeding process
func Run(opts Opts) error {
	ctx := context.Background()
	log := opts.Logger

	log.Info().Msg("Starting database seeding process...")

	// Initialize storage
	store, err := storage.NewStorage(opts.Config)
	if err != nil {
		return fmt.Errorf("failed to create storage: %w", err)
	}

	db := store.DB()

	// Run migrations first to ensure schema is up to date
	log.Info().Msg("Running database migrations...")
	store.Migrate()

	// Initialize authnz repository
	log.Info().Msg("Initializing authnz repository...")
	authnzStorage, err := authnzRepo.NewRepo(db, opts.Config)
	if err != nil {
		return fmt.Errorf("failed to create authnz repository: %w", err)
	}

	// Seed users
	log.Info().Msg("Seeding users...")
	if err := authnzSeed.SeedUsers(ctx, authnzStorage, log); err != nil {
		return fmt.Errorf("failed to seed users: %w", err)
	}

	// Get the first user (admin) to use as the owner of topics
	users, err := authnzStorage.UserRepo().GetAll(ctx)
	if err != nil {
		return fmt.Errorf("failed to get users: %w", err)
	}

	if len(users) == 0 {
		return fmt.Errorf("no users found after seeding")
	}

	adminUserID := users[0].ID
	log.Info().Str("user_id", adminUserID.String()).Msg("Using first user as topic owner")

	// Initialize listing repository
	log.Info().Msg("Initializing listing repository...")
	listingStorage, err := listingRepo.NewStorage(ctx, opts.Config, log)
	if err != nil {
		return fmt.Errorf("failed to create listing repository: %w", err)
	}
	defer listingStorage.Close()

	// Seed topics and items
	log.Info().Msg("Seeding topics and items...")
	if err := listingSeed.SeedTopicsAndItems(ctx, listingStorage, adminUserID, log); err != nil {
		return fmt.Errorf("failed to seed topics and items: %w", err)
	}

	log.Info().Msg("Database seeding completed successfully!")
	return nil
}

// SeedWithUserID seeds data with a specific user ID (useful for testing)
func SeedWithUserID(opts Opts, userID uuid.UUID) error {
	ctx := context.Background()
	log := opts.Logger

	log.Info().Str("user_id", userID.String()).Msg("Starting database seeding with specific user ID...")

	// Initialize storage
	store, err := storage.NewStorage(opts.Config)
	if err != nil {
		return fmt.Errorf("failed to create storage: %w", err)
	}

	db := store.DB()

	// Run migrations first
	log.Info().Msg("Running database migrations...")
	store.Migrate()

	// Initialize authnz repository
	authnzStorage, err := authnzRepo.NewRepo(db, opts.Config)
	if err != nil {
		return fmt.Errorf("failed to create authnz repository: %w", err)
	}

	// Seed users
	log.Info().Msg("Seeding users...")
	if err := authnzSeed.SeedUsers(ctx, authnzStorage, log); err != nil {
		return fmt.Errorf("failed to seed users: %w", err)
	}

	// Initialize listing repository
	log.Info().Msg("Initializing listing repository...")
	listingStorage, err := listingRepo.NewStorage(ctx, opts.Config, log)
	if err != nil {
		return fmt.Errorf("failed to create listing repository: %w", err)
	}
	defer listingStorage.Close()

	// Seed topics and items with provided user ID
	log.Info().Msg("Seeding topics and items...")
	if err := listingSeed.SeedTopicsAndItems(ctx, listingStorage, userID, log); err != nil {
		return fmt.Errorf("failed to seed topics and items: %w", err)
	}

	log.Info().Msg("Database seeding completed successfully!")
	return nil
}
