package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/all-in-one/internal/auth"
	"github.com/all-in-one/internal/chat/model"
	"github.com/all-in-one/internal/chat/websocket"
	httpHelper "github.com/all-in-one/internal/http"
	"github.com/all-in-one/internal/logging"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	ws "github.com/gorilla/websocket"
)

var upgrader = ws.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		// TODO: Implement proper origin checking for production
		return true
	},
}

// GetMessages returns message history for a chat session
// @Summary Get message history
// @Description Get all messages for a specific chat session with optional limit
// @Tags chat
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Session ID"
// @Param limit query int false "Maximum number of messages to return" default(100)
// @Success 200 {object} httpHelper.Response
// @Failure 401 {object} httpHelper.Response
// @Failure 403 {object} httpHelper.Response
// @Failure 404 {object} httpHelper.Response
// @Failure 500 {object} httpHelper.Response
// @Router /chats/{id}/messages [get]
func (h *Handler) GetMessages(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logging.GetLoggerFromContext(ctx)

	s, ok := auth.GetUserFromContext(ctx)
	if !ok {
		log.Error().Msg("Failed to get user from context")
		httpHelper.SendJSON(w, httpHelper.Response{
			Success: false,
			Error:   "Unauthorized",
		}, http.StatusUnauthorized)
		return
	}

	userID := s.UserID
	uuidUserID, err := uuid.Parse(userID)
	if err != nil {
		log.Error().Err(err).Str("user_id", userID).Msg("Invalid user ID format")
		httpHelper.SendJSON(w, httpHelper.Response{
			Success: false,
			Error:   "Unauthorized",
		}, http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(r)
	sessionID := vars["id"]

	// Verify user is a party in the session
	session, err := h.storage.SessionRepo().Get(ctx, sessionID)
	if err != nil {
		log.Error().Err(err).Str("session_id", sessionID).Msg("Failed to get session")
		httpHelper.SendJSON(w, httpHelper.Response{
			Success: false,
			Error:   "Session not found",
		}, http.StatusNotFound)
		return
	}

	// Check if user is an active participant
	isActive := false
	for _, p := range session.GetActiveParticipants() {
		if p.UserID == uuidUserID.String() {
			isActive = true
			break
		}
	}
	if !isActive {
		log.Warn().Str("session_id", sessionID).Str("user_id", userID).Msg("User not authorized for session")
		httpHelper.SendJSON(w, httpHelper.Response{
			Success: false,
			Error:   "Not authorized to access this session",
		}, http.StatusForbidden)
		return
	}

	// Get limit from query params
	limit := 100
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 {
			limit = parsedLimit
		}
	}

	messages, err := h.storage.MessageRepo().GetBySessionID(ctx, sessionID, limit)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get messages")
		httpHelper.SendJSON(w, httpHelper.Response{
			Success: false,
			Error:   "Failed to retrieve messages",
		}, http.StatusInternalServerError)
		return
	}

	httpHelper.SendJSON(w, httpHelper.Response{
		Success: true,
		Data:    messages,
	}, http.StatusOK)
}

// HandleWebSocket handles WebSocket connection upgrades
// @Summary WebSocket endpoint
// @Description Upgrade HTTP connection to WebSocket for real-time chat
// @Tags chat
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Session ID"
// @Router /chats/{id}/ws [get]
func (h *Handler) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logging.GetLoggerFromContext(ctx)

	s, ok := auth.GetUserFromContext(ctx)
	if !ok {
		log.Error().Msg("Failed to get user from context")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	userID := s.UserID
	uuidUserID, err := uuid.Parse(userID)
	if err != nil {
		log.Error().Err(err).Str("user_id", userID).Msg("Invalid user ID format")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(r)
	sessionID := vars["id"]

	// Verify user is a party in the session
	session, err := h.storage.SessionRepo().Get(ctx, sessionID)
	if err != nil {
		log.Error().Err(err).Str("session_id", sessionID).Msg("Failed to get session")
		http.Error(w, "Session not found", http.StatusNotFound)
		return
	}

	// Check if user is an active participant
	isActive := false
	for _, p := range session.GetActiveParticipants() {
		if p.UserID == uuidUserID.String() {
			isActive = true
			break
		}
	}
	if !isActive {
		log.Warn().Str("session_id", sessionID).Str("user_id", userID).Msg("User not authorized for session")
		http.Error(w, "Not authorized", http.StatusForbidden)
		return
	}

	// Upgrade HTTP connection to WebSocket
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Error().Err(err).Msg("Failed to upgrade WebSocket connection")
		return
	}

	// Get username (you may need to fetch from user service or context)
	username := s.Username // Placeholder - should fetch actual username

	// Create new client
	client := websocket.NewClient(h.hub, conn, sessionID, uuidUserID, username, *log)

	// Register client with hub
	h.hub.Register(client)

	// Start client pumps in separate goroutines
	go client.WritePump()
	go h.handleClientMessages(client)

	log.Info().
		Str("session_id", sessionID).
		Str("user_id", userID).
		Msg("WebSocket connection established")
}

// handleClientMessages processes incoming messages from a WebSocket client
func (h *Handler) handleClientMessages(client *websocket.Client) {
	defer func() {
		h.hub.Unregister(client)
	}()

	client.ReadPump()
}

// BroadcastMessage broadcasts a message to all clients in a session
// This is called when a message is persisted to the database
func (h *Handler) BroadcastMessage(sessionID string, chat model.ChatMessage) {
	payload := model.MessagePayload{
		ID:            chat.ID,
		ChatSessionID: chat.ChatSessionID,
		UserID:        chat.UserID,
		Username:      chat.Username,
		Message:       chat.Message,
		CreatedAt:     chat.CreatedAt,
	}

	wsMsg := model.WebSocketMessage{
		Type:      "message",
		Payload:   payload,
		Timestamp: time.Now(),
	}

	h.hub.Broadcast(sessionID, wsMsg)
}

// CreateMessage creates a new message (called via WebSocket or REST)
func (h *Handler) CreateMessage(ctx context.Context, sessionID string, userID uuid.UUID, username string, messageText string) (*model.ChatMessage, error) {
	log := logging.GetLoggerFromContext(ctx)

	// Verify user is a party in the session
	session, err := h.storage.SessionRepo().Get(ctx, sessionID)
	if err != nil {
		log.Error().Err(err).Str("session_id", sessionID).Msg("Failed to get session")
		return nil, err
	}

	// Check if user is an active participant
	isActive := false
	for _, p := range session.GetActiveParticipants() {
		if p.UserID == userID.String() {
			isActive = true
			break
		}
	}
	if !isActive {
		log.Warn().Str("session_id", sessionID).Str("user_id", userID.String()).Msg("User not authorized for session")
		return nil, httpHelper.ErrNotFound
	}

	// Create the message
	chat := model.ChatMessage{
		ID:            uuid.NewString(),
		ChatSessionID: sessionID,
		UserID:        userID.String(),
		Message:       messageText,
		Username:      username,
		CreatedAt:     time.Now(),
	}

	createdChat, err := h.storage.MessageRepo().Create(ctx, chat)
	if err != nil {
		log.Error().Err(err).Msg("Failed to create message")
		return nil, err
	}

	// Broadcast to all clients in the session
	h.BroadcastMessage(sessionID, createdChat)

	log.Info().Str("message_id", createdChat.ID).Str("session_id", sessionID).Msg("Message created and broadcast")

	return &createdChat, nil
}

// SearchUsers searches for users by username or name
// @Summary Search users
// @Description Search for users to invite to chat sessions
// @Tags chat
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param q query string true "Search query (username or name)"
// @Success 200 {object} httpHelper.Response
// @Failure 400 {object} httpHelper.Response
// @Failure 401 {object} httpHelper.Response
// @Failure 500 {object} httpHelper.Response
// @Router /users/search [get]
func (h *Handler) SearchUsers(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logging.GetLoggerFromContext(ctx)

	_, ok := auth.GetUserFromContext(ctx)
	if !ok {
		log.Error().Msg("Failed to get user from context")
		httpHelper.SendJSON(w, httpHelper.Response{
			Success: false,
			Error:   "Unauthorized",
		}, http.StatusUnauthorized)
		return
	}

	query := r.URL.Query().Get("q")
	if query == "" {
		httpHelper.SendJSON(w, httpHelper.Response{
			Success: false,
			Error:   "Search query is required",
		}, http.StatusBadRequest)
		return
	}

	// TODO: Implement actual user search
	// For now, return empty results as this requires authnz service integration
	type UserSearchResult struct {
		ID       string `json:"id"`
		Username string `json:"username"`
		Name     string `json:"name"`
		Email    string `json:"email"`
	}

	httpHelper.SendJSON(w, httpHelper.Response{
		Success: true,
		Data:    []UserSearchResult{},
		Message: "User search not yet implemented - integrate with authnz service",
	}, http.StatusOK)
}

// SendMessage creates a new message in a chat session
// @Summary Send a message
// @Description Send a new message to a chat session
// @Tags chat
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Session ID"
// @Param message body object true "Message content"
// @Success 201 {object} httpHelper.Response
// @Failure 400 {object} httpHelper.Response
// @Failure 401 {object} httpHelper.Response
// @Failure 403 {object} httpHelper.Response
// @Failure 404 {object} httpHelper.Response
// @Failure 500 {object} httpHelper.Response
// @Router /chats/{id}/messages [post]
func (h *Handler) SendMessage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logging.GetLoggerFromContext(ctx)

	s, ok := auth.GetUserFromContext(ctx)
	if !ok {
		log.Error().Msg("Failed to get user from context")
		httpHelper.SendJSON(w, httpHelper.Response{
			Success: false,
			Error:   "Unauthorized",
		}, http.StatusUnauthorized)
		return
	}

	userID := s.UserID
	uuidUserID, err := uuid.Parse(userID)
	if err != nil {
		log.Error().Err(err).Str("user_id", userID).Msg("Invalid user ID format")
		httpHelper.SendJSON(w, httpHelper.Response{
			Success: false,
			Error:   "Unauthorized",
		}, http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(r)
	sessionID := vars["id"]

	// Verify session exists and user is a participant
	session, err := h.storage.SessionRepo().Get(ctx, sessionID)
	if err != nil {
		log.Error().Err(err).Str("session_id", sessionID).Msg("Failed to get session")
		httpHelper.SendJSON(w, httpHelper.Response{
			Success: false,
			Error:   "Session not found",
		}, http.StatusNotFound)
		return
	}

	// Check if user is an active participant
	isActive := false
	for _, p := range session.GetActiveParticipants() {
		if p.UserID == uuidUserID.String() {
			isActive = true
			break
		}
	}
	if !isActive {
		log.Warn().Str("session_id", sessionID).Str("user_id", userID).Msg("User not authorized for session")
		httpHelper.SendJSON(w, httpHelper.Response{
			Success: false,
			Error:   "Not authorized to send messages to this session",
		}, http.StatusForbidden)
		return
	}

	// Parse request body
	var req struct {
		Message string `json:"message"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Error().Err(err).Msg("Failed to decode request body")
		httpHelper.SendJSON(w, httpHelper.Response{
			Success: false,
			Error:   "Invalid request body",
		}, http.StatusBadRequest)
		return
	}

	if req.Message == "" {
		httpHelper.SendJSON(w, httpHelper.Response{
			Success: false,
			Error:   "Message content is required",
		}, http.StatusBadRequest)
		return
	}

	// Create message
	message := model.ChatMessage{
		ChatSessionID: session.ID,
		UserID:        uuidUserID.String(),
		Message:       req.Message,
		CreatedAt:     time.Now(),
		SentAt:        time.Now(),
	}

	createdMessage, err := h.storage.MessageRepo().Create(ctx, message)
	if err != nil {
		log.Error().Err(err).Msg("Failed to create message")
		httpHelper.SendJSON(w, httpHelper.Response{
			Success: false,
			Error:   "Failed to send message",
		}, http.StatusInternalServerError)
		return
	}

	log.Info().Str("session_id", sessionID).Str("message_id", createdMessage.ID).Msg("Message created")

	httpHelper.SendJSON(w, httpHelper.Response{
		Success: true,
		Data:    createdMessage,
		Message: "Message sent successfully",
	}, http.StatusCreated)
}
