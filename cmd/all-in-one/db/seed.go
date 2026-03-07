package db

import (
	"context"
	"fmt"

	authnzRepo "github.com/all-in-one/internal/authnz/repository"
	authnzSeed "github.com/all-in-one/internal/authnz/seed"
	chatRepo "github.com/all-in-one/internal/chat/repository"
	chatSeed "github.com/all-in-one/internal/chat/seed"
	"github.com/all-in-one/internal/config"

	// listingRepo "github.com/all-in-one/internal/listing/repository"
	// listingSeed "github.com/all-in-one/internal/listing/seed"
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
	if err := store.MigrateUp(); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

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
	// log.Info().Msg("Initializing listing repository...")
	// listingStorage, err := listingRepo.NewStorage(ctx, opts.Config, log)
	// if err != nil {
	// 	return fmt.Errorf("failed to create listing repository: %w", err)
	// }
	// defer listingStorage.Close()

	// Seed topics and items
	// log.Info().Msg("Seeding topics and items...")
	// if err := listingSeed.SeedTopicsAndItems(ctx, listingStorage, adminUserID, log); err != nil {
	// 	return fmt.Errorf("failed to seed topics and items: %w", err)
	// }

	// Initialize chat repository
	log.Info().Msg("Initializing chat repository...")
	chatStorage, err := chatRepo.NewStorage(ctx, opts.Config, log)
	if err != nil {
		return fmt.Errorf("failed to create chat repository: %w", err)
	}

	// Collect user IDs for chat seeding
	userIDs := make([]uuid.UUID, len(users))
	for i, user := range users {
		userIDs[i] = user.ID
	}

	// Seed chat sessions and messages
	log.Info().Msg("Seeding chat sessions and messages...")
	if err := chatSeed.SeedChatData(ctx, chatStorage, userIDs, log); err != nil {
		return fmt.Errorf("failed to seed chat data: %w", err)
	}

	log.Info().Msg("Database seeding completed successfully!")
	return nil
}
