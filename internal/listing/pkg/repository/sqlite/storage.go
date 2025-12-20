package sqlite

import (
	"context"
	"fmt"

	"github.com/all-in-one/internal/auth"
	"github.com/all-in-one/internal/config"
	"github.com/all-in-one/internal/listing/pkg/model"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
	"github.com/rs/zerolog"
)

// storage implements the Storage interface from repository package
type storage struct {
	db          *sqlx.DB
	log         zerolog.Logger
	itemRepo    *itemRepository
	topicRepo   *topicRepository
	sessionRepo *sessionRepository
	userRepo    *userRepository
}

// NewStorage creates a new SQLite-based storage
func NewStorage(ctx context.Context, config config.Config, log zerolog.Logger) (*storage, error) {
	db, err := sqlx.Open("sqlite3", config.Storage.SQLite.DBPath)
	if err != nil {
		return nil, err
	}

	log.Info().Msg("Starting database migration")
	err = dbMigrate(db)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to execute db migration: %w", err)
	}

	return &storage{
		db:          db,
		log:         log,
		itemRepo:    newItemRepository(db),
		topicRepo:   newTopicRepository(db),
		sessionRepo: newSessionRepository(db),
		userRepo:    newUserRepository(db),
	}, nil
}

func dbMigrate(db *sqlx.DB) error {
	migration := `CREATE TABLE IF NOT EXISTS topics (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			description TEXT,
			form_schema TEXT, -- JSON Form Schema in JSON format
			created_at TIMESTAMP,
			updated_at TIMESTAMP
		);

		CREATE TABLE IF NOT EXISTS items (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			topic_id INTEGER,
			title TEXT NOT NULL,
			description TEXT,
			form_schema_values TEXT, -- values for JSON Form Schema in JSON format
			created_at TIMESTAMP,
			updated_at TIMESTAMP,
			FOREIGN KEY (topic_id) REFERENCES topics(id)
		);
		
		CREATE TABLE IF NOT EXISTS users (
			id TEXT PRIMARY KEY UNIQUE,
			username TEXT NOT NULL UNIQUE,
			email TEXT NOT NULL UNIQUE,
			name TEXT UNIQUE,
			password_hash TEXT NOT NULL,
			last_login TIMESTAMP,
			created_at TIMESTAMP,
			updated_at TIMESTAMP
		);

		CREATE TABLE IF NOT EXISTS sessions (
			id TEXT PRIMARY KEY UNIQUE,
			user_id TEXT NOT NULL,
			created_at TIMESTAMP,
			user_agent TEXT,
			access_token_expiry INTEGER, -- unix timestampt
			refresh_token_expiry INTEGER, -- unix timestampt
			FOREIGN KEY (user_id) REFERENCES users(id)
		);
		`

	_, err := db.Exec(migration)

	return err
}

// ItemRepo returns the item repository
func (s *storage) ItemRepo() *itemRepository {
	return s.itemRepo
}

func (s *storage) TopicRepo() *topicRepository {
	return s.topicRepo
}

func (s *storage) SessionRepo() *sessionRepository {
	return s.sessionRepo
}

func (s *storage) UserRepo() *userRepository {
	return s.userRepo
}

// Close closes the database connection
func (s *storage) Close() error {
	return s.db.Close()
}

func (s *storage) InitializeSampleData(ctx context.Context) int {
	users, err := s.userRepo.GetAll(ctx)
	if err != nil || len(users) > 0 {
		s.log.Error().Err(err).Msg("failed to fetch users or users already exist")
		return 0
	}

	uid, err := uuid.NewUUID()
	if err != nil {
		s.log.Error().Err(err).Msg("failed to generate admin user ID")
		return 0
	}

	pwd, _ := auth.HashPassword("randompass")

	admin := model.User{
		ID:           uid,
		Username:     "admin",
		Name:         "admin",
		PasswordHash: pwd,
	}

	err = s.userRepo.Create(ctx, admin)
	if err != nil {
		s.log.Error().Err(err).Msg("failed to create admin user")
		return 0
	}

	// Check if there's already data
	items, err := s.itemRepo.GetAll(ctx)
	if err != nil || len(items) > 0 {
		return 0 // Don't add sample data if there's an error or if data exists
	}

	sampleTopics := []model.Topic{
		{
			Name:        "Books to Read",
			Description: "A list of must-read books",
		},
		{
			Name:        "Movies to Watch",
			Description: "A collection of classic and modern movies",
		},
	}

	var sampleItems []model.Item

	for _, topic := range sampleTopics {
		createdTopic, err := s.topicRepo.Create(ctx, topic)
		if err != nil {
			return 0
		}

		sampleItems = []model.Item{
			{
				TopicID:     createdTopic.ID,
				Title:       "Sample Item 1",
				Description: "This is a sample item for testing",
			},
			{
				TopicID:     createdTopic.ID,
				Title:       "Sample Item 2",
				Description: "Another sample item with different content",
			},
			{
				TopicID:     createdTopic.ID,
				Title:       "Sample Item 3",
				Description: "Third sample item for demonstration",
			},
		}

		for _, item := range sampleItems {
			_, err := s.itemRepo.Create(ctx, item)
			if err != nil {
				return 0
			}
		}

	}

	return len(sampleTopics)
}
