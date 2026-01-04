package handler

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/all-in-one/internal/auth"
	httpHelper "github.com/all-in-one/internal/http"
	"github.com/all-in-one/internal/listing/model"
	"github.com/all-in-one/internal/logging"
	"github.com/google/uuid"
)

// GetTopics godoc
// @Summary      Get all topics
// @Description  Retrieve a list of all topics
// @Tags         topics
// @Produce      json
// @Success      200  {object}  httpHelper.Response{data=[]model.Topic}  "List of topics"
// @Failure      500  {object}  httpHelper.Response                      "Internal server error"
// @Router       /topics [get]
func (h *Handler) GetTopics(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logging.GetLoggerFromContext(ctx)

	cu, ok := auth.GetUserFromContext(ctx)
	if !ok {
		log.Error().Msg("failed to get user from context")
		httpHelper.SendError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	uid, err := uuid.Parse(cu.UserID)
	if err != nil {
		log.Error().Err(err).Str("user_id", cu.UserID).Msg("invalid user ID in context")
		httpHelper.SendError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	topics, err := h.storage.TopicRepo().GetAll(ctx, uid)
	if err != nil {
		log.Error().Err(err).Msg("failed to get topics from db")
		httpHelper.SendError(w, "Failed to retrieve topics", http.StatusInternalServerError)
		return
	}

	response := httpHelper.Response{
		Success: true,
		Data:    topics,
	}

	httpHelper.SendJSON(w, response, http.StatusOK)
}

// GetTopic godoc
// @Summary      Get topic by ID
// @Description  Retrieve a single topic by its ID
// @Tags         topics
// @Produce      json
// @Param        id   path      int                                    true  "Topic ID"
// @Success      200  {object}  httpHelper.Response{data=model.Topic}  "Topic details"
// @Failure      400  {object}  httpHelper.Response                    "Invalid ID"
// @Failure      404  {object}  httpHelper.Response                    "Topic not found"
// @Failure      500  {object}  httpHelper.Response                    "Internal server error"
// @Router       /topics/{id} [get]
func (h *Handler) GetTopic(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logging.GetLoggerFromContext(ctx)

	id, err := getIDFromRequest(r)
	if err != nil {
		log.Error().Err(err).Msg("invalid topic ID in request")
		httpHelper.SendError(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	topic, err := h.storage.TopicRepo().Get(ctx, id)
	if err != nil {
		if err == httpHelper.ErrNotFound {
			httpHelper.SendError(w, "Topic not found", http.StatusNotFound)
			return
		}
		httpHelper.SendError(w, "Failed to retrieve topic", http.StatusInternalServerError)
		return
	}

	response := httpHelper.Response{
		Success: true,
		Data:    topic,
	}

	httpHelper.SendJSON(w, response, http.StatusOK)
}

// CreateTopic godoc
// @Summary      Create a new topic
// @Description  Create a new topic
// @Tags         topics
// @Accept       json
// @Produce      json
// @Param        topic  body      model.Topic                            true  "Topic to create"
// @Success      201    {object}  httpHelper.Response{data=model.Topic}  "Topic created successfully"
// @Failure      400    {object}  httpHelper.Response                    "Invalid JSON data or missing required fields"
// @Failure      500    {object}  httpHelper.Response                    "Internal server error"
// @Router       /topics [post]
func (h *Handler) CreateTopic(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logging.GetLoggerFromContext(ctx)

	var newTopic model.Topic
	if err := json.NewDecoder(r.Body).Decode(&newTopic); err != nil {
		log.Error().Err(err).Msg("invalid JSON data")
		httpHelper.SendError(w, "Invalid JSON data", http.StatusBadRequest)
		return
	}

	// Validate required fields
	if newTopic.Name == "" {
		httpHelper.SendError(w, "Name is required", http.StatusBadRequest)
		return
	}

	createdTopic, err := h.storage.TopicRepo().Create(ctx, newTopic)
	if err != nil {
		log.Error().Err(err).Msg("failed to create topic in db")
		httpHelper.SendError(w, "Failed to create topic", http.StatusInternalServerError)
		return
	}

	response := httpHelper.Response{
		Success: true,
		Message: "Topic created successfully",
		Data:    createdTopic,
	}

	httpHelper.SendJSON(w, response, http.StatusCreated)
}

// UpdateTopic godoc
// @Summary      Update an existing topic
// @Description  Update an existing topic by ID
// @Tags         topics
// @Accept       json
// @Produce      json
// @Param        id     path      int                                    true  "Topic ID"
// @Param        topic  body      model.Topic                            true  "Updated topic data"
// @Success      200    {object}  httpHelper.Response{data=model.Topic}  "Topic updated successfully"
// @Failure      400    {object}  httpHelper.Response                    "Invalid ID or JSON data"
// @Failure      404    {object}  httpHelper.Response                    "Topic not found"
// @Failure      500    {object}  httpHelper.Response                    "Internal server error"
// @Router       /topics/{id} [put]
func (h *Handler) UpdateTopic(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logging.GetLoggerFromContext(ctx)

	id, err := getIDFromRequest(r)
	if err != nil {
		log.Error().Err(err).Msg("invalid topic ID in request")
		httpHelper.SendError(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	var updatedTopic model.Topic
	if err := json.NewDecoder(r.Body).Decode(&updatedTopic); err != nil {
		log.Error().Err(err).Msg("invalid JSON data")
		httpHelper.SendError(w, "Invalid JSON data", http.StatusBadRequest)
		return
	}

	// Validate required fields
	if updatedTopic.Name == "" {
		httpHelper.SendError(w, "Name is required", http.StatusBadRequest)
		return
	}

	result, err := h.storage.TopicRepo().Update(ctx, id, updatedTopic)
	if err != nil {
		if err == httpHelper.ErrNotFound {
			httpHelper.SendError(w, "Topic not found", http.StatusNotFound)
			return
		}
		log.Error().Err(err).Msg("failed to update topic in db")
		httpHelper.SendError(w, "Failed to update topic", http.StatusInternalServerError)
		return
	}

	response := httpHelper.Response{
		Success: true,
		Message: "Topic updated successfully",
		Data:    result,
	}

	httpHelper.SendJSON(w, response, http.StatusOK)
}

// DeleteTopic godoc
// @Summary      Delete a topic
// @Description  Delete a topic by ID
// @Tags         topics
// @Produce      json
// @Param        id   path      int                  true  "Topic ID"
// @Success      200  {object}  httpHelper.Response  "Topic deleted successfully"
// @Failure      400  {object}  httpHelper.Response  "Invalid ID"
// @Failure      404  {object}  httpHelper.Response  "Topic not found"
// @Failure      500  {object}  httpHelper.Response  "Internal server error"
// @Router       /topics/{id} [delete]
func (h *Handler) DeleteTopic(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logging.GetLoggerFromContext(ctx)

	id, err := getIDFromRequest(r)
	if err != nil {
		httpHelper.SendError(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	trx, err := h.storage.TopicRepo().CreateTrx(ctx)
	if err != nil {
		log.Error().Err(err).Msg("failed to begin transaction")
		httpHelper.SendError(w, fmt.Sprintf("Failed to create transaction: %v", err), http.StatusInternalServerError)
		return
	}

	defer trx.Rollback()

	err = h.storage.ItemRepo().DeleteByTopicID(ctx, id, trx)
	if err != nil {
		log.Error().Err(err).Msg("failed to delete topic items in db")
		httpHelper.SendError(w, fmt.Sprintf("Failed to delete topic items: %v", err), http.StatusInternalServerError)
		return
	}

	err = h.storage.TopicRepo().Delete(ctx, id, trx)
	if err != nil {
		if err == httpHelper.ErrNotFound {
			httpHelper.SendError(w, "Topic not found", http.StatusNotFound)
			return
		}
		log.Error().Err(err).Msg("failed to delete topic in db")
		httpHelper.SendError(w, fmt.Sprintf("Failed to delete topic: %v", err), http.StatusInternalServerError)
		return
	}

	err = trx.Commit()
	if err != nil {
		log.Error().Err(err).Msg("failed to commit transaction")
		httpHelper.SendError(w, fmt.Sprintf("Failed to commit transaction: %v", err), http.StatusInternalServerError)
		return
	}

	response := httpHelper.Response{
		Success: true,
		Message: "Topic deleted successfully",
	}

	httpHelper.SendJSON(w, response, http.StatusOK)
}
