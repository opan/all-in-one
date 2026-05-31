package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/all-in-one/internal/auth"
	"github.com/all-in-one/internal/chat/model"
	httpHelper "github.com/all-in-one/internal/http"
	"github.com/all-in-one/internal/logging"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// CreateInvite sends one or more invites to the specified participants.
// If session_id is provided, invitees are invited into an existing session (inviter must be active participant).
// If session_id is nil, a new session will be created upon first acceptance.
// @Summary Send chat invite(s)
// @Tags chat-invites
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body model.CreateInviteRequest true "Invite details"
// @Success 201 {object} httpHelper.Response
// @Failure 400 {object} httpHelper.Response
// @Failure 401 {object} httpHelper.Response
// @Failure 403 {object} httpHelper.Response
// @Failure 409 {object} httpHelper.Response
// @Failure 500 {object} httpHelper.Response
// @Router /chats/invites [post]
func (h *Handler) CreateInvite(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logging.GetLoggerFromContext(ctx)

	s, ok := auth.GetUserFromContext(ctx)
	if !ok {
		httpHelper.SendJSON(w, httpHelper.Response{Success: false, Error: "Unauthorized"}, http.StatusUnauthorized)
		return
	}

	var req model.CreateInviteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpHelper.SendJSON(w, httpHelper.Response{Success: false, Error: "Invalid request body"}, http.StatusBadRequest)
		return
	}

	if len(req.Participants) == 0 {
		httpHelper.SendJSON(w, httpHelper.Response{Success: false, Error: "At least one participant must be specified"}, http.StatusBadRequest)
		return
	}

	inviterID := s.UserID

	// If session_id is provided, validate the existing session and the inviter's membership
	if req.SessionID != nil {
		session, err := h.storage.SessionRepo().Get(ctx, *req.SessionID)
		if err != nil {
			httpHelper.SendJSON(w, httpHelper.Response{Success: false, Error: "Session not found"}, http.StatusBadRequest)
			return
		}

		if session.Status != model.SessionStatusActive {
			httpHelper.SendJSON(w, httpHelper.Response{Success: false, Error: "Session is not active"}, http.StatusBadRequest)
			return
		}

		isParticipant := false
		for _, p := range session.GetActiveParticipants() {
			if p.UserID == inviterID {
				isParticipant = true
				break
			}
		}
		if !isParticipant {
			httpHelper.SendJSON(w, httpHelper.Response{Success: false, Error: "Not authorized to invite users to this session"}, http.StatusForbidden)
			return
		}

		// Reject invitees who are already active participants
		activeIDs := make(map[string]bool)
		for _, p := range session.GetActiveParticipants() {
			activeIDs[p.UserID] = true
		}
		for _, pID := range req.Participants {
			if activeIDs[pID] {
				httpHelper.SendJSON(w, httpHelper.Response{Success: false, Error: "User " + pID + " is already a participant in this session"}, http.StatusConflict)
				return
			}
		}
	}

	// Check for an existing active session between inviter and the single invitee (new-session invite only)
	if req.SessionID == nil && len(req.Participants) == 1 {
		allIDs := []string{inviterID, req.Participants[0]}
		hash := model.GenerateParticipantHash(allIDs)
		if _, err := h.storage.SessionRepo().GetByParticipantHash(ctx, hash); err == nil {
			httpHelper.SendJSON(w, httpHelper.Response{
				Success: false,
				Error:   "A chat session already exists with this user",
			}, http.StatusConflict)
			return
		}
	}

	batchID := uuid.NewString()
	invites := make([]model.ChatInvite, 0, len(req.Participants))

	for _, inviteeStr := range req.Participants {
		inviteeID, err := uuid.Parse(inviteeStr)
		if err != nil {
			httpHelper.SendJSON(w, httpHelper.Response{Success: false, Error: "Invalid participant ID: " + inviteeStr}, http.StatusBadRequest)
			return
		}

		// Duplicate invite check (scoped correctly per context)
		if req.SessionID == nil {
			has, err := h.storage.InviteRepo().HasPendingInvite(ctx, inviterID, inviteeID.String())
			if err != nil {
				log.Error().Err(err).Msg("Failed to check pending invite")
				httpHelper.SendJSON(w, httpHelper.Response{Success: false, Error: "Failed to process invite"}, http.StatusInternalServerError)
				return
			}
			if has {
				httpHelper.SendJSON(w, httpHelper.Response{Success: false, Error: "Invite already pending for user " + inviteeStr}, http.StatusConflict)
				return
			}
		} else {
			has, err := h.storage.InviteRepo().HasPendingInviteForSession(ctx, *req.SessionID, inviteeID.String())
			if err != nil {
				log.Error().Err(err).Msg("Failed to check pending session invite")
				httpHelper.SendJSON(w, httpHelper.Response{Success: false, Error: "Failed to process invite"}, http.StatusInternalServerError)
				return
			}
			if has {
				httpHelper.SendJSON(w, httpHelper.Response{Success: false, Error: "Invite already pending for user " + inviteeStr}, http.StatusConflict)
				return
			}
		}

		invite := model.ChatInvite{
			ID:        uuid.NewString(),
			BatchID:   batchID,
			InviterID: inviterID,
			InviteeID: inviteeID.String(),
			SessionID: req.SessionID,
			Status:    model.InviteStatusPending,
		}

		created, err := h.storage.InviteRepo().Create(ctx, invite)
		if err != nil {
			log.Error().Err(err).Msg("Failed to create invite")
			httpHelper.SendJSON(w, httpHelper.Response{Success: false, Error: "Failed to create invite"}, http.StatusInternalServerError)
			return
		}

		// Fetch back with usernames for WS payload
		created, _ = h.storage.InviteRepo().GetByID(ctx, created.ID)
		invites = append(invites, created)

		// Notify invitee via WebSocket if online
		h.hub.BroadcastToUsers(ctx, "", model.WebSocketMessage{
			Type: "invite_received",
			Payload: model.InvitePayload{
				InviteID:        created.ID,
				BatchID:         created.BatchID,
				InviterID:       created.InviterID,
				InviterUsername: created.InviterUsername,
				InviteeID:       created.InviteeID,
				InviteeUsername: created.InviteeUsername,
				Status:          created.Status,
			},
			Timestamp: time.Now(),
		}, []string{inviteeID.String()})
	}

	log.Info().Str("batch_id", batchID).Int("count", len(invites)).Msg("Invites created")

	h.metrics.invitesSent.Add(ctx, int64(len(invites)))

	httpHelper.SendJSON(w, httpHelper.Response{
		Success: true,
		Message: "Invites sent",
		Data:    invites,
	}, http.StatusCreated)
}

// GetReceivedInvites returns all pending invites received by the current user.
// @Summary Get received invites
// @Tags chat-invites
// @Produce json
// @Security BearerAuth
// @Success 200 {object} httpHelper.Response
// @Failure 401 {object} httpHelper.Response
// @Failure 500 {object} httpHelper.Response
// @Router /chats/invites/received [get]
func (h *Handler) GetReceivedInvites(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logging.GetLoggerFromContext(ctx)

	s, ok := auth.GetUserFromContext(ctx)
	if !ok {
		httpHelper.SendJSON(w, httpHelper.Response{Success: false, Error: "Unauthorized"}, http.StatusUnauthorized)
		return
	}

	userID, err := uuid.Parse(s.UserID)
	if err != nil {
		httpHelper.SendJSON(w, httpHelper.Response{Success: false, Error: "Invalid user ID"}, http.StatusInternalServerError)
		return
	}

	invites, err := h.storage.InviteRepo().GetPendingByInviteeID(ctx, userID)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get received invites")
		httpHelper.SendJSON(w, httpHelper.Response{Success: false, Error: "Failed to retrieve invites"}, http.StatusInternalServerError)
		return
	}

	httpHelper.SendJSON(w, httpHelper.Response{Success: true, Data: invites}, http.StatusOK)
}

// GetSentInvites returns all invites sent by the current user.
// @Summary Get sent invites
// @Tags chat-invites
// @Produce json
// @Security BearerAuth
// @Success 200 {object} httpHelper.Response
// @Failure 401 {object} httpHelper.Response
// @Failure 500 {object} httpHelper.Response
// @Router /chats/invites/sent [get]
func (h *Handler) GetSentInvites(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logging.GetLoggerFromContext(ctx)

	s, ok := auth.GetUserFromContext(ctx)
	if !ok {
		httpHelper.SendJSON(w, httpHelper.Response{Success: false, Error: "Unauthorized"}, http.StatusUnauthorized)
		return
	}

	userID, err := uuid.Parse(s.UserID)
	if err != nil {
		httpHelper.SendJSON(w, httpHelper.Response{Success: false, Error: "Invalid user ID"}, http.StatusInternalServerError)
		return
	}

	invites, err := h.storage.InviteRepo().GetSentByInviterID(ctx, userID)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get sent invites")
		httpHelper.SendJSON(w, httpHelper.Response{Success: false, Error: "Failed to retrieve invites"}, http.StatusInternalServerError)
		return
	}

	httpHelper.SendJSON(w, httpHelper.Response{Success: true, Data: invites}, http.StatusOK)
}

// RespondToInvite accepts or declines an invite.
// On accept: creates or joins a session and returns it alongside the updated invite.
// On decline: marks the invite declined and notifies the inviter.
// @Summary Respond to an invite
// @Tags chat-invites
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Invite ID"
// @Param body body model.RespondInviteRequest true "Response action"
// @Success 200 {object} httpHelper.Response
// @Failure 400 {object} httpHelper.Response
// @Failure 401 {object} httpHelper.Response
// @Failure 403 {object} httpHelper.Response
// @Failure 404 {object} httpHelper.Response
// @Failure 500 {object} httpHelper.Response
// @Router /chats/invites/{id}/respond [post]
func (h *Handler) RespondToInvite(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logging.GetLoggerFromContext(ctx)

	s, ok := auth.GetUserFromContext(ctx)
	if !ok {
		httpHelper.SendJSON(w, httpHelper.Response{Success: false, Error: "Unauthorized"}, http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(r)
	inviteID := vars["inviteID"]

	var req model.RespondInviteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpHelper.SendJSON(w, httpHelper.Response{Success: false, Error: "Invalid request body"}, http.StatusBadRequest)
		return
	}

	if req.Action != "accept" && req.Action != "decline" {
		httpHelper.SendJSON(w, httpHelper.Response{Success: false, Error: "Action must be 'accept' or 'decline'"}, http.StatusBadRequest)
		return
	}

	invite, err := h.storage.InviteRepo().GetByID(ctx, inviteID)
	if err != nil {
		httpHelper.SendJSON(w, httpHelper.Response{Success: false, Error: "Invite not found"}, http.StatusNotFound)
		return
	}

	if invite.InviteeID != s.UserID {
		httpHelper.SendJSON(w, httpHelper.Response{Success: false, Error: "Not authorized to respond to this invite"}, http.StatusForbidden)
		return
	}

	if invite.Status != model.InviteStatusPending {
		httpHelper.SendJSON(w, httpHelper.Response{Success: false, Error: "Invite is no longer pending"}, http.StatusBadRequest)
		return
	}

	invitePayload := model.InvitePayload{
		InviteID:        invite.ID,
		BatchID:         invite.BatchID,
		InviterID:       invite.InviterID,
		InviterUsername: invite.InviterUsername,
		InviteeID:       invite.InviteeID,
		InviteeUsername: invite.InviteeUsername,
	}

	if req.Action == "decline" {
		if err := h.storage.InviteRepo().UpdateStatus(ctx, invite.ID, model.InviteStatusDeclined); err != nil {
			log.Error().Err(err).Msg("Failed to decline invite")
			httpHelper.SendJSON(w, httpHelper.Response{Success: false, Error: "Failed to decline invite"}, http.StatusInternalServerError)
			return
		}

		invite.Status = model.InviteStatusDeclined
		invitePayload.Status = invite.Status

		h.hub.BroadcastToUsers(ctx, "", model.WebSocketMessage{
			Type:      "invite_declined",
			Payload:   invitePayload,
			Timestamp: time.Now(),
		}, []string{invite.InviterID})

		h.metrics.invitesResponded.Add(ctx, 1, metric.WithAttributes(attribute.String("result", "declined")))

		httpHelper.SendJSON(w, httpHelper.Response{
			Success: true,
			Data:    model.RespondInviteResponse{Invite: invite},
		}, http.StatusOK)
		return
	}

	// --- Accept flow ---
	if err := h.storage.InviteRepo().UpdateStatus(ctx, invite.ID, model.InviteStatusAccepted); err != nil {
		log.Error().Err(err).Msg("Failed to accept invite")
		httpHelper.SendJSON(w, httpHelper.Response{Success: false, Error: "Failed to accept invite"}, http.StatusInternalServerError)
		return
	}
	invite.Status = model.InviteStatusAccepted

	var session model.ChatSession

	if invite.SessionID != nil {
		// Existing-session invite: just add the invitee as participant
		inviteeUUID, _ := uuid.Parse(invite.InviteeID)
		if err := h.storage.SessionRepo().AddParticipant(ctx, *invite.SessionID, inviteeUUID); err != nil {
			log.Error().Err(err).Msg("Failed to add participant to existing session")
			httpHelper.SendJSON(w, httpHelper.Response{Success: false, Error: "Failed to join session"}, http.StatusInternalServerError)
			return
		}

		session, err = h.storage.SessionRepo().Get(ctx, *invite.SessionID)
		if err != nil {
			log.Error().Err(err).Msg("Failed to get session after adding participant")
			httpHelper.SendJSON(w, httpHelper.Response{Success: false, Error: "Failed to retrieve session"}, http.StatusInternalServerError)
			return
		}
	} else {
		// New-session invite: check if another invitee from the same batch already created the session
		batchInvites, err := h.storage.InviteRepo().GetByBatchID(ctx, invite.BatchID)
		if err != nil {
			log.Error().Err(err).Msg("Failed to get batch invites")
			httpHelper.SendJSON(w, httpHelper.Response{Success: false, Error: "Failed to process invite"}, http.StatusInternalServerError)
			return
		}

		var existingSessionID string
		for _, bi := range batchInvites {
			if bi.SessionID != nil {
				existingSessionID = *bi.SessionID
				break
			}
		}

		if existingSessionID != "" {
			// Session already created by a previous acceptor — just join it
			inviteeUUID, _ := uuid.Parse(invite.InviteeID)
			if err := h.storage.SessionRepo().AddParticipant(ctx, existingSessionID, inviteeUUID); err != nil {
				log.Error().Err(err).Msg("Failed to add participant to batch session")
				httpHelper.SendJSON(w, httpHelper.Response{Success: false, Error: "Failed to join session"}, http.StatusInternalServerError)
				return
			}

			session, err = h.storage.SessionRepo().Get(ctx, existingSessionID)
			if err != nil {
				log.Error().Err(err).Msg("Failed to get batch session")
				httpHelper.SendJSON(w, httpHelper.Response{Success: false, Error: "Failed to retrieve session"}, http.StatusInternalServerError)
				return
			}
		} else {
			// First acceptance: create a new session.
			//
			// participant_hash is computed from inviter + this first acceptor only (the founding pair).
			// It does NOT include other potential batch acceptors. The hash identifies who originated
			// the session, providing DB-level dedup for the founding pair. Subsequent acceptors join
			// the existing session without changing the hash.
			participantIDs := []string{invite.InviterID, invite.InviteeID}
			participantHash := model.GenerateParticipantHash(participantIDs)

			participants := []model.SessionParticipant{
				{UserID: invite.InviterID},
				{UserID: invite.InviteeID},
			}

			newSession := model.ChatSession{
				ID:              uuid.NewString(),
				ParticipantHash: participantHash,
				CreatedBy:       invite.InviterID,
				Status:          model.SessionStatusActive,
				Participants:    participants,
			}

			session, err = h.storage.SessionRepo().Create(ctx, newSession)
			if err != nil {
				log.Error().Err(err).Msg("Failed to create session from invite")
				httpHelper.SendJSON(w, httpHelper.Response{Success: false, Error: "Failed to create session"}, http.StatusInternalServerError)
				return
			}

			// Propagate session_id to all invites in this batch so subsequent acceptors know to join
			if err := h.storage.InviteRepo().UpdateBatchSessionID(ctx, invite.BatchID, session.ID); err != nil {
				log.Error().Err(err).Msg("Failed to update batch session ID")
			}
		}
	}

	invitePayload.Status = invite.Status
	if session.ID != "" {
		invitePayload.SessionID = session.ID
	}

	h.hub.BroadcastToUsers(ctx, "", model.WebSocketMessage{
		Type:      "invite_accepted",
		Payload:   invitePayload,
		Timestamp: time.Now(),
	}, []string{invite.InviterID})

	log.Info().Str("invite_id", invite.ID).Str("session_id", session.ID).Msg("Invite accepted, session ready")

	h.metrics.invitesResponded.Add(ctx, 1, metric.WithAttributes(attribute.String("result", "accepted")))

	httpHelper.SendJSON(w, httpHelper.Response{
		Success: true,
		Data:    model.RespondInviteResponse{Invite: invite, Session: &session},
	}, http.StatusOK)
}

// CancelInvite cancels a pending invite (inviter only).
// @Summary Cancel a pending invite
// @Tags chat-invites
// @Produce json
// @Security BearerAuth
// @Param id path string true "Invite ID"
// @Success 200 {object} httpHelper.Response
// @Failure 400 {object} httpHelper.Response
// @Failure 401 {object} httpHelper.Response
// @Failure 403 {object} httpHelper.Response
// @Failure 404 {object} httpHelper.Response
// @Failure 500 {object} httpHelper.Response
// @Router /chats/invites/{id} [delete]
func (h *Handler) CancelInvite(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logging.GetLoggerFromContext(ctx)

	s, ok := auth.GetUserFromContext(ctx)
	if !ok {
		httpHelper.SendJSON(w, httpHelper.Response{Success: false, Error: "Unauthorized"}, http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(r)
	inviteID := vars["inviteID"]

	invite, err := h.storage.InviteRepo().GetByID(ctx, inviteID)
	if err != nil {
		httpHelper.SendJSON(w, httpHelper.Response{Success: false, Error: "Invite not found"}, http.StatusNotFound)
		return
	}

	if invite.InviterID != s.UserID {
		httpHelper.SendJSON(w, httpHelper.Response{Success: false, Error: "Not authorized to cancel this invite"}, http.StatusForbidden)
		return
	}

	if invite.Status != model.InviteStatusPending {
		httpHelper.SendJSON(w, httpHelper.Response{Success: false, Error: "Only pending invites can be cancelled"}, http.StatusBadRequest)
		return
	}

	if err := h.storage.InviteRepo().UpdateStatus(ctx, invite.ID, model.InviteStatusCancelled); err != nil {
		log.Error().Err(err).Msg("Failed to cancel invite")
		httpHelper.SendJSON(w, httpHelper.Response{Success: false, Error: "Failed to cancel invite"}, http.StatusInternalServerError)
		return
	}

	invite.Status = model.InviteStatusCancelled

	h.hub.BroadcastToUsers(ctx, "", model.WebSocketMessage{
		Type: "invite_cancelled",
		Payload: model.InvitePayload{
			InviteID:        invite.ID,
			BatchID:         invite.BatchID,
			InviterID:       invite.InviterID,
			InviterUsername: invite.InviterUsername,
			InviteeID:       invite.InviteeID,
			InviteeUsername: invite.InviteeUsername,
			Status:          invite.Status,
		},
		Timestamp: time.Now(),
	}, []string{invite.InviteeID})

	log.Info().Str("invite_id", invite.ID).Msg("Invite cancelled")

	h.metrics.invitesCancelled.Add(ctx, 1)

	httpHelper.SendJSON(w, httpHelper.Response{Success: true, Message: "Invite cancelled"}, http.StatusOK)
}
