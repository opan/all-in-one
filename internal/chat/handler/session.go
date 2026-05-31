package handler

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/all-in-one/internal/auth"
	"github.com/all-in-one/internal/chat/model"
	httpHelper "github.com/all-in-one/internal/http"
	"github.com/all-in-one/internal/logging"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

// GetSessions returns all chat sessions for the current user
// @Summary Get all chat sessions
// @Description Get all chat sessions where the current user is a participant
// @Tags chat
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param limit query int false "Max results" default(20)
// @Param offset query int false "Offset for pagination" default(0)
// @Success 200 {object} httpHelper.Response
// @Failure 401 {object} httpHelper.Response
// @Failure 500 {object} httpHelper.Response
// @Router /chats [get]
func (h *Handler) GetSessions(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logging.GetLoggerFromContext(ctx)

	// Get user ID from context (set by JWT middleware)
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

	// Parse pagination parameters
	limit := 20 // default
	offset := 0 // default

	if limitParam := r.URL.Query().Get("limit"); limitParam != "" {
		if parsedLimit, parseErr := fmt.Sscanf(limitParam, "%d", &limit); parseErr == nil && parsedLimit == 1 && limit > 0 && limit <= 100 {
			// use parsed limit
		} else {
			limit = 20 // fallback to default
		}
	}

	if offsetParam := r.URL.Query().Get("offset"); offsetParam != "" {
		if parsedOffset, parseErr := fmt.Sscanf(offsetParam, "%d", &offset); parseErr == nil && parsedOffset == 1 && offset >= 0 {
			// use parsed offset
		} else {
			offset = 0 // fallback to default
		}
	}

	sessions, err := h.storage.SessionRepo().GetAllByUserIDWithPagination(ctx, userID, limit, offset)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get sessions")
		httpHelper.SendJSON(w, httpHelper.Response{
			Success: false,
			Error:   "Failed to retrieve sessions",
		}, http.StatusInternalServerError)
		return
	}

	httpHelper.SendJSON(w, httpHelper.Response{
		Success: true,
		Data:    sessions,
	}, http.StatusOK)
}

// GetSession returns a specific chat session
// @Summary Get a chat session
// @Description Get details of a specific chat session
// @Tags chat
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Session ID"
// @Success 200 {object} httpHelper.Response
// @Failure 401 {object} httpHelper.Response
// @Failure 403 {object} httpHelper.Response
// @Failure 404 {object} httpHelper.Response
// @Failure 500 {object} httpHelper.Response
// @Router /chats/{id} [get]
func (h *Handler) GetSession(w http.ResponseWriter, r *http.Request) {
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

	vars := mux.Vars(r)
	sessionID := vars["id"]

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
		if p.UserID == userID.String() {
			isActive = true
			break
		}
	}
	if !isActive {
		log.Warn().Str("session_id", sessionID).Str("user_id", s.UserID).Msg("User not authorized for session")
		httpHelper.SendJSON(w, httpHelper.Response{
			Success: false,
			Error:   "Not authorized to access this session",
		}, http.StatusForbidden)
		return
	}

	httpHelper.SendJSON(w, httpHelper.Response{
		Success: true,
		Data:    session,
	}, http.StatusOK)
}

// CreateSession creates a new chat session
// @Summary Create a chat session
// @Description Create a new chat session with specified participants
// @Tags chat
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body model.CreateSessionRequest true "Session details"
// @Success 201 {object} httpHelper.Response
// @Failure 400 {object} httpHelper.Response
// @Failure 401 {object} httpHelper.Response
// @Failure 500 {object} httpHelper.Response
// @Router /chats [post]
func (h *Handler) CreateSession(w http.ResponseWriter, r *http.Request) {
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

	var req model.CreateSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Error().Err(err).Msg("Failed to decode request body")
		httpHelper.SendJSON(w, httpHelper.Response{
			Success: false,
			Error:   "Invalid request body",
		}, http.StatusBadRequest)
		return
	}

	// Validate that at least one other participant is specified
	if len(req.Participants) == 0 {
		httpHelper.SendJSON(w, httpHelper.Response{
			Success: false,
			Error:   "At least one participant must be specified",
		}, http.StatusBadRequest)
		return
	}

	// Collect all participant IDs (including current user)
	participantIDs := []string{userID}

	for _, participantStr := range req.Participants {
		participantID, err := uuid.Parse(participantStr)
		if err != nil {
			log.Error().Err(err).Str("participant", participantStr).Msg("Invalid participant UUID")
			httpHelper.SendJSON(w, httpHelper.Response{
				Success: false,
				Error:   "Invalid participant ID: " + participantStr,
			}, http.StatusBadRequest)
			return
		}
		// Avoid duplicates
		isDuplicate := false
		for _, existingID := range participantIDs {
			if existingID == participantID.String() {
				isDuplicate = true
				break
			}
		}
		if !isDuplicate {
			participantIDs = append(participantIDs, participantID.String())
		}
	}

	// Generate participant hash for uniqueness
	participantHash := model.GenerateParticipantHash(participantIDs)

	// Check if session with these participants already exists
	existingSession, err := h.storage.SessionRepo().GetByParticipantHash(ctx, participantHash)
	if err == nil {
		// Session already exists, return it
		log.Info().Str("session_id", existingSession.ID).Msg("Session already exists with these participants")
		httpHelper.SendJSON(w, httpHelper.Response{
			Success: true,
			Message: "Session already exists",
			Data:    existingSession,
		}, http.StatusOK)
		return
	}

	// Create participant objects
	participants := make([]model.SessionParticipant, 0, len(participantIDs))
	for _, pID := range participantIDs {
		participants = append(participants, model.SessionParticipant{
			UserID: pID,
		})
	}

	// Create the session
	session := model.ChatSession{
		ID:              uuid.NewString(),
		ParticipantHash: participantHash,
		CreatedBy:       userID,
		Status:          model.SessionStatusActive,
		Participants:    participants,
	}

	createdSession, err := h.storage.SessionRepo().Create(ctx, session)
	if err != nil {
		log.Error().Err(err).Msg("Failed to create session")
		httpHelper.SendJSON(w, httpHelper.Response{
			Success: false,
			Error:   "Failed to create session",
		}, http.StatusInternalServerError)
		return
	}

	log.Info().Str("session_id", createdSession.ID).Msg("Session created successfully")

	h.metrics.sessionsCreated.Add(ctx, 1)

	httpHelper.SendJSON(w, httpHelper.Response{
		Success: true,
		Message: "Session created successfully",
		Data:    createdSession,
	}, http.StatusCreated)
}

// UpdateSession updates an existing chat session (e.g., add parties)
// @Summary Update a chat session
// @Description Update session details like adding new participants
// @Tags chat
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Session ID"
// @Param body body model.UpdateSessionRequest true "Update details"
// @Success 200 {object} httpHelper.Response
// @Failure 400 {object} httpHelper.Response
// @Failure 401 {object} httpHelper.Response
// @Failure 403 {object} httpHelper.Response
// @Failure 404 {object} httpHelper.Response
// @Failure 500 {object} httpHelper.Response
// @Router /chats/{id} [put]
func (h *Handler) UpdateSession(w http.ResponseWriter, r *http.Request) {
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

	vars := mux.Vars(r)
	sessionID := vars["id"]

	// Get existing session
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
		if p.UserID == userID.String() {
			isActive = true
			break
		}
	}
	if !isActive {
		log.Warn().Str("session_id", sessionID).Str("user_id", s.UserID).Msg("User not authorized for session")
		httpHelper.SendJSON(w, httpHelper.Response{
			Success: false,
			Error:   "Not authorized to update this session",
		}, http.StatusForbidden)
		return
	}

	var req model.UpdateSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Error().Err(err).Msg("Failed to decode request body")
		httpHelper.SendJSON(w, httpHelper.Response{
			Success: false,
			Error:   "Invalid request body",
		}, http.StatusBadRequest)
		return
	}

	// Add participants if provided
	if len(req.AddParticipants) > 0 {
		for _, participantStr := range req.AddParticipants {
			participantID, err := uuid.Parse(participantStr)
			if err != nil {
				log.Error().Err(err).Str("participant", participantStr).Msg("Invalid participant UUID")
				httpHelper.SendJSON(w, httpHelper.Response{
					Success: false,
					Error:   "Invalid participant ID: " + participantStr,
				}, http.StatusBadRequest)
				return
			}
			// Add participant to session
			err = h.storage.SessionRepo().AddParticipant(ctx, sessionID, participantID)
			if err != nil {
				log.Error().Err(err).Str("participant_id", participantStr).Msg("Failed to add participant")
				httpHelper.SendJSON(w, httpHelper.Response{
					Success: false,
					Error:   "Failed to add participant",
				}, http.StatusInternalServerError)
				return
			}
		}
		// Invalidate participant cache for this session
		h.hub.InvalidateSessionCache(sessionID)
	}

	// Update status if provided
	if req.Status != "" {
		session.Status = req.Status
	}

	updatedSession, err := h.storage.SessionRepo().Update(ctx, sessionID, session)
	if err != nil {
		log.Error().Err(err).Msg("Failed to update session")
		httpHelper.SendJSON(w, httpHelper.Response{
			Success: false,
			Error:   "Failed to update session",
		}, http.StatusInternalServerError)
		return
	}

	log.Info().Str("session_id", sessionID).Msg("Session updated successfully")

	httpHelper.SendJSON(w, httpHelper.Response{
		Success: true,
		Message: "Session updated successfully",
		Data:    updatedSession,
	}, http.StatusOK)
}

// DeleteSession soft deletes a chat session
// @Summary Delete a chat session
// @Description Soft delete a chat session (archives it)
// @Tags chat
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Session ID"
// @Success 200 {object} httpHelper.Response
// @Failure 401 {object} httpHelper.Response
// @Failure 403 {object} httpHelper.Response
// @Failure 404 {object} httpHelper.Response
// @Failure 500 {object} httpHelper.Response
// @Router /chats/{id} [delete]
func (h *Handler) DeleteSession(w http.ResponseWriter, r *http.Request) {
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

	vars := mux.Vars(r)
	sessionID := vars["id"]

	// Get existing session
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
		if p.UserID == userID.String() {
			isActive = true
			break
		}
	}
	if !isActive {
		log.Warn().Str("session_id", sessionID).Str("user_id", s.UserID).Msg("User not authorized for session")
		httpHelper.SendJSON(w, httpHelper.Response{
			Success: false,
			Error:   "Not authorized to delete this session",
		}, http.StatusForbidden)
		return
	}

	err = h.storage.SessionRepo().Delete(ctx, sessionID)
	if err != nil {
		log.Error().Err(err).Msg("Failed to delete session")
		httpHelper.SendJSON(w, httpHelper.Response{
			Success: false,
			Error:   "Failed to delete session",
		}, http.StatusInternalServerError)
		return
	}

	log.Info().Str("session_id", sessionID).Msg("Session deleted successfully")

	h.metrics.sessionsDeleted.Add(ctx, 1)

	httpHelper.SendJSON(w, httpHelper.Response{
		Success: true,
		Message: "Session deleted successfully",
	}, http.StatusOK)
}

// LeaveSession removes the current user from a chat session
// @Summary Leave a chat session
// @Description Remove yourself from a chat session. Session auto-deletes if <2 participants remain.
// @Tags chat
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Session ID"
// @Success 200 {object} httpHelper.Response
// @Failure 401 {object} httpHelper.Response
// @Failure 403 {object} httpHelper.Response
// @Failure 404 {object} httpHelper.Response
// @Failure 500 {object} httpHelper.Response
// @Router /chats/{id}/leave [post]
func (h *Handler) LeaveSession(w http.ResponseWriter, r *http.Request) {
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

	vars := mux.Vars(r)
	sessionID := vars["id"]

	// Get existing session
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
		if p.UserID == userID.String() {
			isActive = true
			break
		}
	}
	if !isActive {
		log.Warn().Str("session_id", sessionID).Str("user_id", s.UserID).Msg("User not active in session")
		httpHelper.SendJSON(w, httpHelper.Response{
			Success: false,
			Error:   "You are not an active participant in this session",
		}, http.StatusForbidden)
		return
	}

	// Remove participant (this will auto-delete session if <2 participants remain)
	err = h.storage.SessionRepo().RemoveParticipant(ctx, sessionID, userID)
	if err != nil {
		log.Error().Err(err).Msg("Failed to leave session")
		httpHelper.SendJSON(w, httpHelper.Response{
			Success: false,
			Error:   "Failed to leave session",
		}, http.StatusInternalServerError)
		return
	}

	// Invalidate participant cache for this session
	h.hub.InvalidateSessionCache(sessionID)

	log.Info().Str("session_id", sessionID).Str("user_id", s.UserID).Msg("User left session successfully")

	httpHelper.SendJSON(w, httpHelper.Response{
		Success: true,
		Message: "Successfully left the session",
	}, http.StatusOK)
}
