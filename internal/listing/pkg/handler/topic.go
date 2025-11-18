package handler

import (
	"encoding/json"
	"fmt"
	"net/http"

	httpHelper "github.com/all-in-one/internal/http"
	"github.com/all-in-one/internal/listing/pkg/model"
)

// GetTopics godoc
// @Summary Get all topics
// @Description Retrieve a list of all topics
// @Tags topics
// @Produce json
// @Success 200 {object} httpHelper.Response{data=[]model.Topic} "List of topics"
// @Failure 500 {object} httpHelper.Response "Internal server error"
// @Router /topics [get]
func (h *Handler) GetTopics(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	topics, err := h.storage.TopicRepo().GetAll(ctx)
	if err != nil {
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
// @Summary Get topic by ID
// @Description Retrieve a single topic by its ID
// @Tags topics
// @Produce json
// @Param id path int true "Topic ID"
// @Success 200 {object} httpHelper.Response{data=model.Topic} "Topic details"
// @Failure 400 {object} httpHelper.Response "Invalid ID"
// @Failure 404 {object} httpHelper.Response "Topic not found"
// @Failure 500 {object} httpHelper.Response "Internal server error"
// @Router /topics/{id} [get]
func (h *Handler) GetTopic(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, err := getIDFromRequest(r)
	if err != nil {
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
// @Summary Create a new topic
// @Description Create a new topic
// @Tags topics
// @Accept json
// @Produce json
// @Param topic body model.Topic true "Topic to create"
// @Success 201 {object} httpHelper.Response{data=model.Topic} "Topic created successfully"
// @Failure 400 {object} httpHelper.Response "Invalid JSON data or missing required fields"
// @Failure 500 {object} httpHelper.Response "Internal server error"
// @Router /topics [post]
func (h *Handler) CreateTopic(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var newTopic model.Topic
	if err := json.NewDecoder(r.Body).Decode(&newTopic); err != nil {
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
// @Summary Update an existing topic
// @Description Update an existing topic by ID
// @Tags topics
// @Accept json
// @Produce json
// @Param id path int true "Topic ID"
// @Param topic body model.Topic true "Updated topic data"
// @Success 200 {object} httpHelper.Response{data=model.Topic} "Topic updated successfully"
// @Failure 400 {object} httpHelper.Response "Invalid ID or JSON data"
// @Failure 404 {object} httpHelper.Response "Topic not found"
// @Failure 500 {object} httpHelper.Response "Internal server error"
// @Router /topics/{id} [put]
func (h *Handler) UpdateTopic(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, err := getIDFromRequest(r)
	if err != nil {
		httpHelper.SendError(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	var updatedTopic model.Topic
	if err := json.NewDecoder(r.Body).Decode(&updatedTopic); err != nil {
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
// @Summary Delete a topic
// @Description Delete a topic by ID
// @Tags topics
// @Produce json
// @Param id path int true "Topic ID"
// @Success 200 {object} httpHelper.Response "Topic deleted successfully"
// @Failure 400 {object} httpHelper.Response "Invalid ID"
// @Failure 404 {object} httpHelper.Response "Topic not found"
// @Failure 500 {object} httpHelper.Response "Internal server error"
// @Router /topics/{id} [delete]
func (h *Handler) DeleteTopic(w http.ResponseWriter, r *http.Request) {
	id, err := getIDFromRequest(r)
	if err != nil {
		httpHelper.SendError(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	trx, err := h.storage.TopicRepo().CreateTrx(ctx)
	if err != nil {
		httpHelper.SendError(w, fmt.Sprintf("Failed to create transaction: %v", err), http.StatusInternalServerError)
		return
	}

	defer trx.Rollback()

	err = h.storage.ItemRepo().DeleteByTopicID(ctx, id, trx)
	if err != nil {
		httpHelper.SendError(w, fmt.Sprintf("Failed to delete topic items: %v", err), http.StatusInternalServerError)
		return
	}

	err = h.storage.TopicRepo().Delete(ctx, id, trx)
	if err != nil {
		if err == httpHelper.ErrNotFound {
			httpHelper.SendError(w, "Topic not found", http.StatusNotFound)
			return
		}
		httpHelper.SendError(w, fmt.Sprintf("Failed to delete topic: %v", err), http.StatusInternalServerError)
		return
	}

	err = trx.Commit()
	if err != nil {
		httpHelper.SendError(w, fmt.Sprintf("Failed to commit transaction: %v", err), http.StatusInternalServerError)
		return
	}

	response := httpHelper.Response{
		Success: true,
		Message: "Topic deleted successfully",
	}

	httpHelper.SendJSON(w, response, http.StatusOK)
}
