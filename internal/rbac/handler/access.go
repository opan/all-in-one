package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/all-in-one/internal/rbac"

	httpHelper "github.com/all-in-one/internal/http"
	"github.com/all-in-one/internal/logging"
	"github.com/all-in-one/internal/rbac/model"
	"github.com/google/uuid"
)

type CreateGroupRequest struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	FeatureKeys []string `json:"feature_keys"`
}

type UpdateGroupRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type SetGroupFeaturesRequest struct {
	FeatureKeys []string `json:"feature_keys"`
}

type AssignUserGroupRequest struct {
	GroupID *string `json:"group_id"`
}

type SetUserOverridesRequest struct {
	Overrides []model.FeatureOverrideView `json:"overrides"`
}

// handleServiceError maps a service-layer error to the appropriate HTTP
// status: 404 for anything not found (a path-referenced group/user, or a
// feature key named in the request body that doesn't exist), 409 for the
// admin-lockout and built-in-group guards, 500 otherwise.
func handleServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, httpHelper.ErrNotFound):
		httpHelper.SendError(w, "not found", http.StatusNotFound)
	case errors.Is(err, rbac.ErrLastAdmin), errors.Is(err, rbac.ErrBuiltinGroup):
		httpHelper.SendError(w, err.Error(), http.StatusConflict)
	default:
		httpHelper.SendError(w, "internal server error", http.StatusInternalServerError)
	}
}

// ListFeatures godoc
// @Summary      List RBAC features
// @Description  List every code-registered app-feature (admin-only). Features are code-defined; this endpoint is read-only.
// @Tags         access-management
// @Produce      json
// @Security     BearerAuth || DirectAuth
// @Success      200  {object}  httpHelper.Response{data=[]model.Feature}  "List of features"
// @Failure      401  {object}  httpHelper.Response                        "Unauthorized"
// @Failure      403  {object}  httpHelper.Response                        "Forbidden (not an admin)"
// @Router       /access/features [get]
func (h *Handler) ListFeatures(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logging.GetLoggerFromContext(ctx)

	features, err := h.service.ListFeatures(ctx)
	if err != nil {
		log.Error().Err(err).Msg("failed to list features")
		httpHelper.SendError(w, "failed to list features", http.StatusInternalServerError)
		return
	}

	httpHelper.SendJSON(w, httpHelper.Response{Success: true, Data: features}, http.StatusOK)
}

// ListGroups godoc
// @Summary      List RBAC groups
// @Description  List every group (built-in and custom), each with its granted feature keys (admin-only)
// @Tags         access-management
// @Produce      json
// @Security     BearerAuth || DirectAuth
// @Success      200  {object}  httpHelper.Response{data=[]model.Group}  "List of groups"
// @Failure      401  {object}  httpHelper.Response                      "Unauthorized"
// @Failure      403  {object}  httpHelper.Response                      "Forbidden (not an admin)"
// @Router       /access/groups [get]
func (h *Handler) ListGroups(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logging.GetLoggerFromContext(ctx)

	groups, err := h.service.ListGroups(ctx)
	if err != nil {
		log.Error().Err(err).Msg("failed to list groups")
		httpHelper.SendError(w, "failed to list groups", http.StatusInternalServerError)
		return
	}

	httpHelper.SendJSON(w, httpHelper.Response{Success: true, Data: groups}, http.StatusOK)
}

// GetGroup godoc
// @Summary      Get a group by ID
// @Description  Retrieve a single group, including its granted feature keys (admin-only)
// @Tags         access-management
// @Produce      json
// @Param        id   path      string                                  true  "Group ID"
// @Security     BearerAuth || DirectAuth
// @Success      200  {object}  httpHelper.Response{data=model.Group}  "Group details"
// @Failure      400  {object}  httpHelper.Response                    "Invalid ID"
// @Failure      404  {object}  httpHelper.Response                    "Group not found"
// @Router       /access/groups/{id} [get]
func (h *Handler) GetGroup(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logging.GetLoggerFromContext(ctx)

	id, err := getIDFromRequest(r)
	if err != nil {
		httpHelper.SendError(w, "invalid group id", http.StatusBadRequest)
		return
	}

	group, err := h.service.GetGroup(ctx, id)
	if err != nil {
		log.Error().Err(err).Str("group_id", id.String()).Msg("failed to get group")
		handleServiceError(w, err)
		return
	}

	httpHelper.SendJSON(w, httpHelper.Response{Success: true, Data: group}, http.StatusOK)
}

// CreateGroup godoc
// @Summary      Create a group
// @Description  Create a custom permission-preset group with an initial set of granted features (admin-only)
// @Tags         access-management
// @Accept       json
// @Produce      json
// @Param        group  body      CreateGroupRequest                     true  "Group to create"
// @Security     BearerAuth || DirectAuth
// @Success      201    {object}  httpHelper.Response{data=model.Group}  "Group created"
// @Failure      400    {object}  httpHelper.Response                    "Invalid request body or unknown feature key"
// @Router       /access/groups [post]
func (h *Handler) CreateGroup(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logging.GetLoggerFromContext(ctx)

	var req CreateGroupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpHelper.SendError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	if req.Name == "" {
		httpHelper.SendError(w, "name is required", http.StatusBadRequest)
		return
	}

	group, err := h.service.CreateGroup(ctx, req.Name, req.Description, req.FeatureKeys)
	if err != nil {
		log.Error().Err(err).Str("name", req.Name).Msg("failed to create group")
		if errors.Is(err, httpHelper.ErrNotFound) {
			httpHelper.SendError(w, "unknown feature key", http.StatusBadRequest)
			return
		}
		httpHelper.SendError(w, "failed to create group", http.StatusInternalServerError)
		return
	}

	h.metrics.groupsChanged.Add(ctx, 1, groupChangeAttr("created"))

	httpHelper.SendJSON(w, httpHelper.Response{Success: true, Message: "group created", Data: group}, http.StatusCreated)
}

// UpdateGroup godoc
// @Summary      Update a group
// @Description  Rename or re-describe a group. Renaming a built-in group (admin, regular-user) is rejected (admin-only)
// @Tags         access-management
// @Accept       json
// @Produce      json
// @Param        id     path      string                                  true  "Group ID"
// @Param        group  body      UpdateGroupRequest                      true  "Updated group data"
// @Security     BearerAuth || DirectAuth
// @Success      200    {object}  httpHelper.Response{data=model.Group}  "Group updated"
// @Failure      400    {object}  httpHelper.Response                    "Invalid ID or request body"
// @Failure      404    {object}  httpHelper.Response                    "Group not found"
// @Failure      409    {object}  httpHelper.Response                    "Cannot rename a built-in group"
// @Router       /access/groups/{id} [put]
func (h *Handler) UpdateGroup(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logging.GetLoggerFromContext(ctx)

	id, err := getIDFromRequest(r)
	if err != nil {
		httpHelper.SendError(w, "invalid group id", http.StatusBadRequest)
		return
	}

	var req UpdateGroupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpHelper.SendError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	if req.Name == "" {
		httpHelper.SendError(w, "name is required", http.StatusBadRequest)
		return
	}

	group, err := h.service.UpdateGroup(ctx, id, req.Name, req.Description)
	if err != nil {
		log.Error().Err(err).Str("group_id", id.String()).Msg("failed to update group")
		handleServiceError(w, err)
		return
	}

	h.metrics.groupsChanged.Add(ctx, 1, groupChangeAttr("updated"))

	httpHelper.SendJSON(w, httpHelper.Response{Success: true, Message: "group updated", Data: group}, http.StatusOK)
}

// DeleteGroup godoc
// @Summary      Delete a group
// @Description  Delete a custom group. Built-in groups (admin, regular-user) cannot be deleted (admin-only)
// @Tags         access-management
// @Produce      json
// @Param        id   path      string                true  "Group ID"
// @Security     BearerAuth || DirectAuth
// @Success      200  {object}  httpHelper.Response   "Group deleted"
// @Failure      400  {object}  httpHelper.Response   "Invalid ID"
// @Failure      404  {object}  httpHelper.Response   "Group not found"
// @Failure      409  {object}  httpHelper.Response   "Cannot delete a built-in group"
// @Router       /access/groups/{id} [delete]
func (h *Handler) DeleteGroup(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logging.GetLoggerFromContext(ctx)

	id, err := getIDFromRequest(r)
	if err != nil {
		httpHelper.SendError(w, "invalid group id", http.StatusBadRequest)
		return
	}

	if err := h.service.DeleteGroup(ctx, id); err != nil {
		log.Error().Err(err).Str("group_id", id.String()).Msg("failed to delete group")
		handleServiceError(w, err)
		return
	}

	h.metrics.groupsChanged.Add(ctx, 1, groupChangeAttr("deleted"))

	httpHelper.SendJSON(w, httpHelper.Response{Success: true, Message: "group deleted"}, http.StatusOK)
}

// SetGroupFeatures godoc
// @Summary      Replace a group's feature grants
// @Description  Atomically replace which features a group grants. Rejected for the admin group, whose grants are never consulted (admin bypasses all gates) (admin-only)
// @Tags         access-management
// @Accept       json
// @Produce      json
// @Param        id      path      string                                  true  "Group ID"
// @Param        grants  body      SetGroupFeaturesRequest                true  "Feature keys to grant"
// @Security     BearerAuth || DirectAuth
// @Success      200     {object}  httpHelper.Response{data=model.Group}  "Grants replaced"
// @Failure      400     {object}  httpHelper.Response                    "Invalid ID, request body, or unknown feature key"
// @Failure      404     {object}  httpHelper.Response                    "Group not found"
// @Failure      409     {object}  httpHelper.Response                    "Cannot edit the admin group's grants"
// @Router       /access/groups/{id}/features [put]
func (h *Handler) SetGroupFeatures(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logging.GetLoggerFromContext(ctx)

	id, err := getIDFromRequest(r)
	if err != nil {
		httpHelper.SendError(w, "invalid group id", http.StatusBadRequest)
		return
	}

	var req SetGroupFeaturesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpHelper.SendError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	group, err := h.service.SetGroupFeatures(ctx, id, req.FeatureKeys)
	if err != nil {
		log.Error().Err(err).Str("group_id", id.String()).Msg("failed to set group features")
		if errors.Is(err, rbac.ErrBuiltinGroup) {
			httpHelper.SendError(w, err.Error(), http.StatusConflict)
			return
		}
		handleServiceError(w, err)
		return
	}

	h.metrics.groupsChanged.Add(ctx, 1, groupChangeAttr("features_set"))

	httpHelper.SendJSON(w, httpHelper.Response{Success: true, Message: "group features replaced", Data: group}, http.StatusOK)
}

// ListUsers godoc
// @Summary      List users and their access
// @Description  List every user with their assigned group and admin status (admin-only)
// @Tags         access-management
// @Produce      json
// @Security     BearerAuth || DirectAuth
// @Success      200  {object}  httpHelper.Response{data=[]model.UserAccessRow}  "List of users"
// @Failure      401  {object}  httpHelper.Response                              "Unauthorized"
// @Failure      403  {object}  httpHelper.Response                              "Forbidden (not an admin)"
// @Router       /access/users [get]
func (h *Handler) ListUsers(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logging.GetLoggerFromContext(ctx)

	users, err := h.service.ListUsers(ctx)
	if err != nil {
		log.Error().Err(err).Msg("failed to list users")
		httpHelper.SendError(w, "failed to list users", http.StatusInternalServerError)
		return
	}

	httpHelper.SendJSON(w, httpHelper.Response{Success: true, Data: users}, http.StatusOK)
}

// AssignUserGroup godoc
// @Summary      Reassign a user's group
// @Description  Move a user to a different group (or unassign them back to the regular-user default with group_id=null). Rejected with 409 if this would remove the last remaining admin (admin-only)
// @Tags         access-management
// @Accept       json
// @Produce      json
// @Param        id      path      string                   true  "User ID"
// @Param        group   body      AssignUserGroupRequest  true  "Target group ID (null to unassign)"
// @Security     BearerAuth || DirectAuth
// @Success      200     {object}  httpHelper.Response      "Group reassigned"
// @Failure      400     {object}  httpHelper.Response      "Invalid ID or request body"
// @Failure      404     {object}  httpHelper.Response      "User or group not found"
// @Failure      409     {object}  httpHelper.Response      "Cannot remove the last admin"
// @Router       /access/users/{id}/group [put]
func (h *Handler) AssignUserGroup(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logging.GetLoggerFromContext(ctx)

	userID, err := getIDFromRequest(r)
	if err != nil {
		httpHelper.SendError(w, "invalid user id", http.StatusBadRequest)
		return
	}

	var req AssignUserGroupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpHelper.SendError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var groupID *uuid.UUID
	if req.GroupID != nil {
		parsed, err := uuid.Parse(*req.GroupID)
		if err != nil {
			httpHelper.SendError(w, "invalid group id", http.StatusBadRequest)
			return
		}
		groupID = &parsed
	}

	if err := h.service.AssignUserGroup(ctx, userID, groupID); err != nil {
		log.Error().Err(err).Str("user_id", userID.String()).Msg("failed to assign user group")
		handleServiceError(w, err)
		return
	}

	h.metrics.userGroupAssigned.Add(ctx, 1)

	httpHelper.SendJSON(w, httpHelper.Response{Success: true, Message: "user group reassigned"}, http.StatusOK)
}

// ListUserOverrides godoc
// @Summary      List a user's feature overrides
// @Description  List the per-feature grant/revoke overrides set for a user (admin-only)
// @Tags         access-management
// @Produce      json
// @Param        id   path      string                                                true  "User ID"
// @Security     BearerAuth || DirectAuth
// @Success      200  {object}  httpHelper.Response{data=[]model.FeatureOverrideView}  "List of overrides"
// @Failure      400  {object}  httpHelper.Response                                    "Invalid ID"
// @Router       /access/users/{id}/overrides [get]
func (h *Handler) ListUserOverrides(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logging.GetLoggerFromContext(ctx)

	userID, err := getIDFromRequest(r)
	if err != nil {
		httpHelper.SendError(w, "invalid user id", http.StatusBadRequest)
		return
	}

	overrides, err := h.service.ListUserOverrides(ctx, userID)
	if err != nil {
		log.Error().Err(err).Str("user_id", userID.String()).Msg("failed to list user overrides")
		httpHelper.SendError(w, "failed to list user overrides", http.StatusInternalServerError)
		return
	}

	httpHelper.SendJSON(w, httpHelper.Response{Success: true, Data: overrides}, http.StatusOK)
}

// SetUserOverrides godoc
// @Summary      Replace a user's feature overrides
// @Description  Atomically replace all per-feature grant/revoke overrides for a user. Overrides take precedence over the user's group but can never grant admin-equivalent access (admin-only)
// @Tags         access-management
// @Accept       json
// @Produce      json
// @Param        id         path      string                    true  "User ID"
// @Param        overrides  body      SetUserOverridesRequest  true  "Overrides to set"
// @Security     BearerAuth || DirectAuth
// @Success      200        {object}  httpHelper.Response       "Overrides replaced"
// @Failure      400        {object}  httpHelper.Response       "Invalid ID, request body, or unknown feature key"
// @Router       /access/users/{id}/overrides [put]
func (h *Handler) SetUserOverrides(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logging.GetLoggerFromContext(ctx)

	userID, err := getIDFromRequest(r)
	if err != nil {
		httpHelper.SendError(w, "invalid user id", http.StatusBadRequest)
		return
	}

	var req SetUserOverridesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpHelper.SendError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	if err := h.service.SetUserOverrides(ctx, userID, req.Overrides); err != nil {
		log.Error().Err(err).Str("user_id", userID.String()).Msg("failed to set user overrides")
		if errors.Is(err, httpHelper.ErrNotFound) {
			httpHelper.SendError(w, "unknown feature key", http.StatusBadRequest)
			return
		}
		httpHelper.SendError(w, "failed to set user overrides", http.StatusInternalServerError)
		return
	}

	h.metrics.userOverridesSet.Add(ctx, 1)

	httpHelper.SendJSON(w, httpHelper.Response{Success: true, Message: "user overrides replaced"}, http.StatusOK)
}
