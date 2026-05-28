package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
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

	// Upgrade HTTP connection to WebSocket
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Error().Err(err).Msg("Failed to upgrade WebSocket connection")
		return
	}

	// Get username
	username := s.Username

	// Mark the otelmux span for this upgrade request with WS-specific attributes.
	// The handler returns immediately after starting the pumps, so the span
	// lifetime correctly covers just the connection-establishment phase.
	trace.SpanFromContext(ctx).SetAttributes(
		attribute.Bool("ws.upgrade", true),
		attribute.String("user.id", userID),
		attribute.String("username", username),
	)

	// Create a new context for the WebSocket connection that's not tied to the HTTP request
	wsCtx, wsCancel := context.WithCancel(context.Background())

	// Copy request_id and other important values to the new context
	if requestID := ctx.Value("request_id"); requestID != nil {
		wsCtx = context.WithValue(wsCtx, "request_id", requestID)
	}

	// Create new client with message handler (no sessionID needed)
	client := websocket.NewClient(h.hub, conn, uuidUserID, username, h.handleWebSocketMessage, wsCtx, wsCancel, *log)

	// Register client with hub
	h.hub.Register(client)

	// Start client pumps in separate goroutines
	go client.WritePump()
	go h.handleClientMessages(client)

	log.Info().
		Str("user_id", userID).
		Msg("WebSocket connection established")
}

// handleWebSocketMessage processes incoming WebSocket messages
func (h *Handler) handleWebSocketMessage(ctx context.Context, client *websocket.Client, wsMsg model.WebSocketMessage) error {
	log := logging.GetLoggerFromContext(ctx)

	switch wsMsg.Type {
	case "message":
		// Extract message content and session ID from payload
		payloadMap, ok := wsMsg.Payload.(map[string]interface{})
		if !ok {
			return errors.New("invalid message payload format")
		}

		messageText, ok := payloadMap["message"].(string)
		if !ok || messageText == "" {
			return errors.New("message text is required")
		}

		// Extract sessionID from payload
		sessionID, ok := payloadMap["session_id"].(string)
		if !ok || sessionID == "" {
			return errors.New("session_id is required in message payload")
		}

		// Create and save the message (includes authorization check)
		_, err := h.CreateMessage(ctx, sessionID, client.GetUserID(), client.GetUsername(), messageText)
		if err != nil {
			log.Error().Err(err).Msg("Failed to create message from WebSocket")
			return err
		}

		log.Info().Str("session_id", sessionID).Msg("Message created from WebSocket")
		return nil

	case "typing":
		// Extract sessionID from typing payload
		payloadMap, ok := wsMsg.Payload.(map[string]interface{})
		if !ok {
			return errors.New("invalid typing payload format")
		}

		sessionID, ok := payloadMap["session_id"].(string)
		if !ok || sessionID == "" {
			return errors.New("session_id is required in typing payload")
		}

		// Verify user has access to this session before broadcasting typing
		session, err := h.storage.SessionRepo().Get(ctx, sessionID)
		if err != nil {
			log.Error().Err(err).Str("session_id", sessionID).Msg("Failed to get session for typing indicator")
			return err
		}

		// Check if user is an active participant
		isActive := false
		participants := []string{}
		for _, p := range session.GetActiveParticipants() {
			participants = append(participants, p.UserID)
			if p.UserID == client.GetUserID().String() {
				isActive = true
			}
		}

		if !isActive {
			log.Warn().Str("session_id", sessionID).Str("user_id", client.GetUserID().String()).Msg("User not authorized for session")
			return errors.New("not authorized for this session")
		}

		// Broadcast typing indicator to all participants
		h.hub.BroadcastToUsers(ctx, sessionID, wsMsg, participants)
		return nil

	default:
		log.Warn().Str("type", wsMsg.Type).Msg("Unknown WebSocket message type")
		return nil
	}
}

// handleClientMessages processes incoming messages from a WebSocket client
func (h *Handler) handleClientMessages(client *websocket.Client) {
	defer func() {
		h.hub.Unregister(client)
	}()

	client.ReadPump()
}

// BroadcastMessage broadcasts a message to all participants in a session
// This is called when a message is persisted to the database
func (h *Handler) BroadcastMessage(ctx context.Context, sessionID string, chat model.ChatMessage) {
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

	// Get session participants
	session, err := h.storage.SessionRepo().Get(ctx, sessionID)
	if err != nil {
		h.hub.Broadcast(ctx, sessionID, wsMsg) // Fallback to cached participants
		return
	}

	// Extract participant user IDs
	participants := []string{}
	for _, p := range session.GetActiveParticipants() {
		participants = append(participants, p.UserID)
	}

	// Cache participants for future use
	h.hub.CacheSessionParticipants(sessionID, participants)

	// Broadcast to all participants
	h.hub.BroadcastToUsers(ctx, sessionID, wsMsg, participants)
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
	h.BroadcastMessage(ctx, sessionID, createdChat)

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
// @Param limit query int false "Max results" default(10)
// @Success 200 {object} httpHelper.Response
// @Failure 400 {object} httpHelper.Response
// @Failure 401 {object} httpHelper.Response
// @Failure 500 {object} httpHelper.Response
// @Router /users/search [get]
func (h *Handler) SearchUsers(w http.ResponseWriter, r *http.Request) {
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

	userID, err := uuid.Parse(s.UserID)
	if err != nil {
		log.Error().Err(err).Msg("Failed to parse user ID")
		httpHelper.SendJSON(w, httpHelper.Response{
			Success: false,
			Error:   "Invalid user ID",
		}, http.StatusInternalServerError)
		return
	}

	query := r.URL.Query().Get("q")
	limit := 10
	if limitParam := r.URL.Query().Get("limit"); limitParam != "" {
		if parsedLimit, err := fmt.Sscanf(limitParam, "%d", &limit); err == nil && parsedLimit == 1 && limit > 0 && limit <= 50 {
			// use parsed limit
		} else {
			limit = 10 // fallback to default
		}
	}

	// If query is empty, return empty results
	if query == "" {
		httpHelper.SendJSON(w, httpHelper.Response{
			Success: true,
			Data:    []interface{}{},
		}, http.StatusOK)
		return
	}

	// Search users
	users, err := h.storage.SearchUsers(ctx, query, userID, limit)
	if err != nil {
		log.Error().Err(err).Msg("Failed to search users")
		httpHelper.SendJSON(w, httpHelper.Response{
			Success: false,
			Error:   "Failed to search users",
		}, http.StatusInternalServerError)
		return
	}

	log.Info().Str("query", query).Int("results", len(users)).Msg("User search completed")

	httpHelper.SendJSON(w, httpHelper.Response{
		Success: true,
		Data:    users,
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
