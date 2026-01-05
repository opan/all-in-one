package seed

import (
	"context"
	"fmt"

	"github.com/all-in-one/internal/authnz/model"
	"github.com/all-in-one/internal/authnz/repository"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"golang.org/x/crypto/bcrypt"
)

// SeedUsers initializes sample users in the database
func SeedUsers(ctx context.Context, storage repository.Storage, log zerolog.Logger) error {
	userRepo := storage.UserRepo()

	// Check if users already exist
	existingUsers, err := userRepo.GetAll(ctx)
	if err != nil {
		return fmt.Errorf("failed to check existing users: %w", err)
	}

	if len(existingUsers) > 0 {
		log.Info().Msg("Users already exist, skipping seed")
		return nil
	}

	// Define sample users
	users := []struct {
		username string
		email    string
		name     string
		password string
	}{
		{
			username: "admin",
			email:    "admin@example.com",
			name:     "Administrator",
			password: "admin123",
		},
		{
			username: "user",
			email:    "user@example.com",
			name:     "Test User",
			password: "user123",
		},
		{
			username: "demo",
			email:    "demo@example.com",
			name:     "Demo User",
			password: "demo123",
		},
	}

	// Create users
	for _, u := range users {
		// Hash password
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(u.password), bcrypt.DefaultCost)
		if err != nil {
			return fmt.Errorf("failed to hash password for user %s: %w", u.username, err)
		}

		user := model.User{
			ID:           uuid.New(),
			Username:     u.username,
			Email:        u.email,
			Name:         u.name,
			PasswordHash: string(hashedPassword),
		}

		if err := userRepo.Create(ctx, user); err != nil {
			return fmt.Errorf("failed to create user %s: %w", u.username, err)
		}

		log.Info().
			Str("username", u.username).
			Str("email", u.email).
			Msg("User created successfully")
	}

	log.Info().Int("count", len(users)).Msg("Successfully seeded users")
	return nil
}
