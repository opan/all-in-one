package handler

import (
	"encoding/json"
	"fmt"
	"net/http"

	httpHelper "github.com/all-in-one/internal/http"
	"github.com/all-in-one/internal/listing/pkg/model"
	"github.com/all-in-one/internal/logging"
)

// GetItems godoc
// @Summary      Get all items for a topic
// @Description  Retrieve a list of all items for a specific topic
// @Tags         items
// @Produce      json
// @Param        topic_id  path      int                                     true  "Topic ID"
// @Success      200       {object}  httpHelper.Response{data=[]model.Item}  "List of items"
// @Failure      400       {object}  httpHelper.Response                     "Invalid topic ID"
// @Failure      500       {object}  httpHelper.Response                     "Internal server error"
// @Router       /topics/{topic_id}/items [get]
func (h *Handler) GetItems(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logging.GetLoggerFromContext(ctx)

	fmt.Println("--------------------------------")
	fmt.Println("masuk")
	fmt.Println("--------------------------------")

	topicID, err := getTopicIDFromRequest(r)
	if err != nil {
		log.Error().Err(err).Msg("invalid topic ID in request")
		httpHelper.SendError(w, "Invalid topic ID", http.StatusBadRequest)
		return
	}

	// Verify topic exists
	t, err := h.storage.TopicRepo().Get(ctx, topicID)
	if err != nil {
		if err == httpHelper.ErrNotFound {
			log.Warn().Msg("topic not found")
			httpHelper.SendError(w, "Topic not found", http.StatusNotFound)
			return
		}
		httpHelper.SendError(w, "Failed to verify topic", http.StatusInternalServerError)
		return
	}

	items, err := h.storage.ItemRepo().GetByTopicID(ctx, t.ID)
	if err != nil {
		fmt.Println("----------------------")
		fmt.Println(err)
		log.Error().Err(err).Msg("failed to get items from db")
		httpHelper.SendError(w, "Failed to retrieve items", http.StatusInternalServerError)
		return
	}

	response := httpHelper.Response{
		Success: true,
		Data:    items,
	}

	httpHelper.SendJSON(w, response, http.StatusOK)
}

// GetItem godoc
// @Summary      Get item by ID
// @Description  Retrieve a single item by its ID within a topic
// @Tags         items
// @Produce      json
// @Param        topic_id  path      int                                   true  "Topic ID"
// @Param        id        path      int                                   true  "Item ID"
// @Success      200       {object}  httpHelper.Response{data=model.Item}  "Item details"
// @Failure      400       {object}  httpHelper.Response                   "Invalid ID"
// @Failure      404       {object}  httpHelper.Response                   "Item not found"
// @Failure      500       {object}  httpHelper.Response                   "Internal server error"
// @Router       /topics/{topic_id}/items/{id} [get]
func (h *Handler) GetItem(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	topicID, err := getTopicIDFromRequest(r)
	if err != nil {
		httpHelper.SendError(w, "Invalid topic ID", http.StatusBadRequest)
		return
	}

	id, err := getIDFromRequest(r)
	if err != nil {
		httpHelper.SendError(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	// Verify topic exists
	_, err = h.storage.TopicRepo().Get(ctx, topicID)
	if err != nil {
		if err == httpHelper.ErrNotFound {
			httpHelper.SendError(w, "Topic not found", http.StatusNotFound)
			return
		}
		httpHelper.SendError(w, "Failed to verify topic", http.StatusInternalServerError)
		return
	}

	item, err := h.storage.ItemRepo().Get(ctx, id)
	if err != nil {
		if err == httpHelper.ErrNotFound {
			httpHelper.SendError(w, "Item not found", http.StatusNotFound)
			return
		}
		httpHelper.SendError(w, "Failed to retrieve item", http.StatusInternalServerError)
		return
	}

	// Verify item belongs to the topic
	if item.TopicID != topicID {
		httpHelper.SendError(w, "Item not found in this topic", http.StatusNotFound)
		return
	}

	response := httpHelper.Response{
		Success: true,
		Data:    item,
	}

	httpHelper.SendJSON(w, response, http.StatusOK)
}

// CreateItem godoc
// @Summary      Create a new item
// @Description  Create a new item within a topic
// @Tags         items
// @Accept       json
// @Produce      json
// @Param        topic_id  path      int                                   true  "Topic ID"
// @Param        item      body      model.Item                            true  "Item to create"
// @Success      201       {object}  httpHelper.Response{data=model.Item}  "Item created successfully"
// @Failure      400       {object}  httpHelper.Response                   "Invalid JSON data or missing required fields"
// @Failure      404       {object}  httpHelper.Response                   "Topic not found"
// @Failure      500       {object}  httpHelper.Response                   "Internal server error"
// @Router       /topics/{topic_id}/items [post]
func (h *Handler) CreateItem(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	topicID, err := getTopicIDFromRequest(r)
	if err != nil {
		httpHelper.SendError(w, "Invalid topic ID", http.StatusBadRequest)
		return
	}

	// Verify topic exists
	_, err = h.storage.TopicRepo().Get(ctx, topicID)
	if err != nil {
		if err == httpHelper.ErrNotFound {
			httpHelper.SendError(w, "Topic not found", http.StatusNotFound)
			return
		}
		httpHelper.SendError(w, "Failed to verify topic", http.StatusInternalServerError)
		return
	}

	var newItem model.Item
	if err := json.NewDecoder(r.Body).Decode(&newItem); err != nil {
		httpHelper.SendError(w, "Invalid JSON data", http.StatusBadRequest)
		return
	}

	// Validate required fields
	if newItem.Title == "" {
		httpHelper.SendError(w, "Title is required", http.StatusBadRequest)
		return
	}

	// Set the topic ID from the URL
	newItem.TopicID = topicID

	createdItem, err := h.storage.ItemRepo().Create(ctx, newItem)
	if err != nil {
		httpHelper.SendError(w, "Failed to create item", http.StatusInternalServerError)
		return
	}

	response := httpHelper.Response{
		Success: true,
		Message: "Item created successfully",
		Data:    createdItem,
	}

	httpHelper.SendJSON(w, response, http.StatusCreated)
}

// UpdateItem godoc
// @Summary      Update an existing item
// @Description  Update an existing item by ID within a topic
// @Tags         items
// @Accept       json
// @Produce      json
// @Param        topic_id  path      int                                   true  "Topic ID"
// @Param        id        path      int                                   true  "Item ID"
// @Param        item      body      model.Item                            true  "Updated item data"
// @Success      200       {object}  httpHelper.Response{data=model.Item}  "Item updated successfully"
// @Failure      400       {object}  httpHelper.Response                   "Invalid ID or JSON data"
// @Failure      404       {object}  httpHelper.Response                   "Item not found"
// @Failure      500       {object}  httpHelper.Response                   "Internal server error"
// @Router       /topics/{topic_id}/items/{id} [put]
func (h *Handler) UpdateItem(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	topicID, err := getTopicIDFromRequest(r)
	if err != nil {
		httpHelper.SendError(w, "Invalid topic ID", http.StatusBadRequest)
		return
	}

	id, err := getIDFromRequest(r)
	if err != nil {
		httpHelper.SendError(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	// Verify topic exists
	_, err = h.storage.TopicRepo().Get(ctx, topicID)
	if err != nil {
		if err == httpHelper.ErrNotFound {
			httpHelper.SendError(w, "Topic not found", http.StatusNotFound)
			return
		}
		httpHelper.SendError(w, "Failed to verify topic", http.StatusInternalServerError)
		return
	}

	// Verify item exists and belongs to topic
	existingItem, err := h.storage.ItemRepo().Get(ctx, id)
	if err != nil {
		if err == httpHelper.ErrNotFound {
			httpHelper.SendError(w, "Item not found", http.StatusNotFound)
			return
		}
		httpHelper.SendError(w, "Failed to retrieve item", http.StatusInternalServerError)
		return
	}

	if existingItem.TopicID != topicID {
		httpHelper.SendError(w, "Item not found in this topic", http.StatusNotFound)
		return
	}

	var updatedItem model.Item
	if err := json.NewDecoder(r.Body).Decode(&updatedItem); err != nil {
		httpHelper.SendError(w, "Invalid JSON data", http.StatusBadRequest)
		return
	}

	// Validate required fields
	if updatedItem.Title == "" {
		httpHelper.SendError(w, "Title is required", http.StatusBadRequest)
		return
	}

	// Ensure topic ID remains the same
	updatedItem.TopicID = topicID

	result, err := h.storage.ItemRepo().Update(ctx, id, updatedItem)
	if err != nil {
		if err == httpHelper.ErrNotFound {
			httpHelper.SendError(w, "Item not found", http.StatusNotFound)
			return
		}
		httpHelper.SendError(w, "Failed to update item", http.StatusInternalServerError)
		return
	}

	response := httpHelper.Response{
		Success: true,
		Message: "Item updated successfully",
		Data:    result,
	}

	httpHelper.SendJSON(w, response, http.StatusOK)
}

// DeleteItem godoc
// @Summary      Delete an item
// @Description  Delete an item by ID within a topic
// @Tags         items
// @Produce      json
// @Param        topic_id  path      int                  true  "Topic ID"
// @Param        id        path      int                  true  "Item ID"
// @Success      200       {object}  httpHelper.Response  "Item deleted successfully"
// @Failure      400       {object}  httpHelper.Response  "Invalid ID"
// @Failure      404       {object}  httpHelper.Response  "Item not found"
// @Failure      500       {object}  httpHelper.Response  "Internal server error"
// @Router       /topics/{topic_id}/items/{id} [delete]
func (h *Handler) DeleteItem(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	topicID, err := getTopicIDFromRequest(r)
	if err != nil {
		httpHelper.SendError(w, "Invalid topic ID", http.StatusBadRequest)
		return
	}

	id, err := getIDFromRequest(r)
	if err != nil {
		httpHelper.SendError(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	// Verify topic exists
	_, err = h.storage.TopicRepo().Get(ctx, topicID)
	if err != nil {
		if err == httpHelper.ErrNotFound {
			httpHelper.SendError(w, "Topic not found", http.StatusNotFound)
			return
		}
		httpHelper.SendError(w, "Failed to verify topic", http.StatusInternalServerError)
		return
	}

	// Verify item exists and belongs to topic
	existingItem, err := h.storage.ItemRepo().Get(ctx, id)
	if err != nil {
		if err == httpHelper.ErrNotFound {
			httpHelper.SendError(w, "Item not found", http.StatusNotFound)
			return
		}
		httpHelper.SendError(w, "Failed to retrieve item", http.StatusInternalServerError)
		return
	}

	if existingItem.TopicID != topicID {
		httpHelper.SendError(w, "Item not found in this topic", http.StatusNotFound)
		return
	}

	err = h.storage.ItemRepo().Delete(ctx, id)
	if err != nil {
		if err == httpHelper.ErrNotFound {
			httpHelper.SendError(w, "Item not found", http.StatusNotFound)
			return
		}
		httpHelper.SendError(w, "Failed to delete item", http.StatusInternalServerError)
		return
	}

	response := httpHelper.Response{
		Success: true,
		Message: "Item deleted successfully",
	}

	httpHelper.SendJSON(w, response, http.StatusOK)
}
