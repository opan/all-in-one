package handler

import (
	"encoding/json"
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

	sessions, err := h.storage.SessionRepo().GetAllByUserID(ctx, userID)
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

	// Verify user is a party in the session
	if !session.HasParty(userID) {
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
	uuidUserID, err := uuid.Parse(userID)
	if err != nil {
		log.Error().Err(err).Msg("Invalid user ID")
		httpHelper.SendJSON(w, httpHelper.Response{
			Success: false,
			Error:   "Invalid user ID",
		}, http.StatusBadRequest)
		return
	}

	var req model.CreateSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Error().Err(err).Msg("Failed to decode request body")
		httpHelper.SendJSON(w, httpHelper.Response{
			Success: false,
			Error:   "Invalid request body",
		}, http.StatusBadRequest)
		return
	}

	// Validate that at least one other party is specified
	if len(req.Parties) == 0 {
		httpHelper.SendJSON(w, httpHelper.Response{
			Success: false,
			Error:   "At least one party must be specified",
		}, http.StatusBadRequest)
		return
	}

	// Parse party UUIDs
	partyIDs := make([]uuid.UUID, 0, len(req.Parties)+1)
	partyIDs = append(partyIDs, uuidUserID) // Add current user

	for _, partyStr := range req.Parties {
		partyID, err := uuid.Parse(partyStr)
		if err != nil {
			log.Error().Err(err).Str("party", partyStr).Msg("Invalid party UUID")
			httpHelper.SendJSON(w, httpHelper.Response{
				Success: false,
				Error:   "Invalid party ID: " + partyStr,
			}, http.StatusBadRequest)
			return
		}
		// Avoid duplicates
		isDuplicate := false
		for _, existingID := range partyIDs {
			if existingID == partyID {
				isDuplicate = true
				break
			}
		}
		if !isDuplicate {
			partyIDs = append(partyIDs, partyID)
		}
	}

	// Create the session
	session := model.ChatSession{
		ID:        uuid.NewString(),
		CreatedBy: userID,
		Status:    model.SessionStatusActive,
	}
	session.SetPartyIDs(partyIDs)

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

	// Verify user is a party
	if !session.HasParty(userID) {
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

	// Update parties if provided
	if len(req.Parties) > 0 {
		for _, partyStr := range req.Parties {
			partyID, err := uuid.Parse(partyStr)
			if err != nil {
				log.Error().Err(err).Str("party", partyStr).Msg("Invalid party UUID")
				httpHelper.SendJSON(w, httpHelper.Response{
					Success: false,
					Error:   "Invalid party ID: " + partyStr,
				}, http.StatusBadRequest)
				return
			}
			session.AddParty(partyID)
		}
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

	// Verify user is a party
	if !session.HasParty(userID) {
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

	httpHelper.SendJSON(w, httpHelper.Response{
		Success: true,
		Message: "Session deleted successfully",
	}, http.StatusOK)
}
