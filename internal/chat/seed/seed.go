package seed

import (
	"context"
	"time"

	"github.com/all-in-one/internal/chat/model"
	"github.com/all-in-one/internal/chat/repository"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

// SeedChatData seeds chat sessions and messages with test data
func SeedChatData(ctx context.Context, storage repository.Storage, userIDs []uuid.UUID, log zerolog.Logger) error {
	if len(userIDs) < 2 {
		log.Warn().Msg("Need at least 2 users to seed chat data, skipping chat seeding")
		return nil
	}

	log.Info().Msg("Creating chat sessions...")

	// Session 1: 1-on-1 chat between user 0 and user 1
	session1, err := createSession(ctx, storage, []uuid.UUID{userIDs[0], userIDs[1]}, userIDs[0], log)
	if err != nil {
		return err
	}
	log.Info().Str("session_id", session1.ID).Msg("Created 1-on-1 session")

	// Add some messages to session 1
	messages1 := []struct {
		senderIdx int
		content   string
		offset    time.Duration
	}{
		{0, "Hey! How are you doing?", -10 * time.Minute},
		{1, "I'm doing great! Just finished the new chat feature.", -9 * time.Minute},
		{0, "Awesome! Can't wait to see it in action.", -8 * time.Minute},
		{1, "It's working really well. Real-time messaging is smooth.", -5 * time.Minute},
		{0, "Perfect! Let's test it more tomorrow.", -2 * time.Minute},
	}

	for _, msg := range messages1 {
		if err := createMessage(ctx, storage, session1.ID, userIDs[msg.senderIdx], msg.content, msg.offset, log); err != nil {
			return err
		}
	}
	log.Info().Str("session_id", session1.ID).Int("message_count", len(messages1)).Msg("Added messages to session 1")

	// Session 2: Group chat with 3+ users (if available)
	if len(userIDs) >= 3 {
		session2, err := createSession(ctx, storage, []uuid.UUID{userIDs[0], userIDs[1], userIDs[2]}, userIDs[0], log)
		if err != nil {
			return err
		}
		log.Info().Str("session_id", session2.ID).Msg("Created group session")

		messages2 := []struct {
			senderIdx int
			content   string
			offset    time.Duration
		}{
			{0, "Welcome to the project discussion group!", -30 * time.Minute},
			{1, "Thanks for adding me!", -28 * time.Minute},
			{2, "Happy to be here. What's on the agenda?", -25 * time.Minute},
			{0, "We need to discuss the new features for next sprint.", -20 * time.Minute},
			{1, "I can work on the backend APIs.", -18 * time.Minute},
			{2, "I'll handle the frontend components.", -15 * time.Minute},
			{0, "Great! Let's sync up tomorrow at 10 AM.", -10 * time.Minute},
		}

		for _, msg := range messages2 {
			if err := createMessage(ctx, storage, session2.ID, userIDs[msg.senderIdx], msg.content, msg.offset, log); err != nil {
				return err
			}
		}
		log.Info().Str("session_id", session2.ID).Int("message_count", len(messages2)).Msg("Added messages to session 2")
	}

	// Session 3: Another 1-on-1 (if we have a 4th user)
	if len(userIDs) >= 4 {
		session3, err := createSession(ctx, storage, []uuid.UUID{userIDs[0], userIDs[3]}, userIDs[0], log)
		if err != nil {
			return err
		}
		log.Info().Str("session_id", session3.ID).Msg("Created another 1-on-1 session")

		messages3 := []struct {
			senderIdx int
			content   string
			offset    time.Duration
		}{
			{0, "Hey, can you review the PR when you get a chance?", -60 * time.Minute},
			{3, "Sure! I'll take a look this afternoon.", -55 * time.Minute},
			{0, "Thanks! No rush, just whenever you have time.", -50 * time.Minute},
		}

		for _, msg := range messages3 {
			if err := createMessage(ctx, storage, session3.ID, userIDs[msg.senderIdx], msg.content, msg.offset, log); err != nil {
				return err
			}
		}
		log.Info().Str("session_id", session3.ID).Int("message_count", len(messages3)).Msg("Added messages to session 3")
	}

	log.Info().Msg("Chat seeding completed successfully")
	return nil
}

// createSession creates a chat session with the given participants
func createSession(ctx context.Context, storage repository.Storage, participants []uuid.UUID, createdBy uuid.UUID, log zerolog.Logger) (model.ChatSession, error) {
	// Convert UUIDs to strings and sort for hash
	participantIDs := make([]string, len(participants))
	for i, id := range participants {
		participantIDs[i] = id.String()
	}

	// Generate participant hash
	participantHash := model.GenerateParticipantHash(participantIDs)

	// Create participant objects
	sessionParticipants := make([]model.SessionParticipant, len(participants))
	for i, id := range participants {
		sessionParticipants[i] = model.SessionParticipant{
			UserID:   id.String(),
			JoinedAt: time.Now(),
		}
	}

	// Create session
	session := model.ChatSession{
		ID:              uuid.NewString(),
		ParticipantHash: participantHash,
		Status:          model.SessionStatusActive,
		CreatedBy:       createdBy.String(),
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
		Participants:    sessionParticipants,
	}

	createdSession, err := storage.SessionRepo().Create(ctx, session)
	if err != nil {
		log.Error().Err(err).Msg("Failed to create session")
		return model.ChatSession{}, err
	}

	return createdSession, nil
}

// createMessage creates a chat message in a session
func createMessage(ctx context.Context, storage repository.Storage, sessionID string, senderID uuid.UUID, content string, timeOffset time.Duration, log zerolog.Logger) error {
	timestamp := time.Now().Add(timeOffset)

	message := model.ChatMessage{
		ID:            uuid.NewString(),
		ChatSessionID: sessionID,
		UserID:        senderID.String(),
		Message:       content,
		CreatedAt:     timestamp,
		SentAt:        timestamp,
	}

	_, err := storage.MessageRepo().Create(ctx, message)
	if err != nil {
		log.Error().Err(err).Str("session_id", sessionID).Msg("Failed to create message")
		return err
	}

	return nil
}
