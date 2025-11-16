package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	httpHelper "github.com/all-in-one/internal/http"
	"github.com/all-in-one/internal/listing/pkg/model"
	"github.com/google/uuid"
)

func (h *Handler) CreateSession(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	fmt.Println(ctx)

	var rl model.LoginRequest

	if err := json.NewDecoder(r.Body).Decode(&rl); err != nil {
		sendError(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	defer r.Body.Close()

	u, err := h.storage.UserRepo().FindByUsername(ctx, rl.Username)
	if err != nil {
		sendError(w, fmt.Sprintf("username or password not found: %v", err), http.StatusNotFound)
		return
	}

	sid, err := uuid.NewUUID()
	if err != nil {
		sendError(w, fmt.Sprintf("failed to generate session id: %v", err), http.StatusInternalServerError)
		return
	}

	s := model.Session{
		ID:        sid,
		UserID:    u.ID,
		CreatedAt: time.Now(),
		UserAgent: r.UserAgent(),
	}

	trx, err := h.storage.TopicRepo().CreateTrx(ctx)
	if err != nil {
		sendError(w, fmt.Sprintf("failed to create transaction: %v", err), http.StatusInternalServerError)
	}
	defer trx.Rollback()

	err = h.storage.SessionRepo().Create(ctx, s, trx)
	if err != nil {
		sendError(w, fmt.Sprintf("failed to create user session: %v", err), http.StatusInternalServerError)
		return
	}

	err = trx.Commit()
	if err != nil {
		sendError(w, fmt.Sprintf("failed to commit session to db: %v", err), http.StatusInternalServerError)
		return
	}

	response := httpHelper.Response{
		Success: true,
		Data:    u,
	}

	sendJSON(w, response, http.StatusCreated)
}
