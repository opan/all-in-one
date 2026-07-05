package service

import (
	"context"
	"fmt"

	authnzRepo "github.com/all-in-one/internal/authnz/repository"
	"github.com/all-in-one/internal/config"
	"github.com/all-in-one/internal/rbac"
	"github.com/all-in-one/internal/rbac/handler"
	"github.com/all-in-one/internal/rbac/model"
	"github.com/all-in-one/internal/rbac/repository"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog"
)

// Service wires the RBAC repository layer to the resolver and the admin-only
// management API handler.
type Service struct {
	Store    repository.Storage
	Resolver *Resolver
	Handler  *handler.Handler

	config config.Config
}

func NewService(ctx context.Context, db *sqlx.DB, config config.Config, log zerolog.Logger) (*Service, error) {
	store, err := repository.NewRepo(db, config)
	if err != nil {
		log.Error().Err(err).Msg("Failed to initiate rbac repository")
		return nil, err
	}

	s := &Service{
		Store:    store,
		Resolver: NewResolver(store),
		config:   config,
	}
	s.Handler = handler.NewHandler(s, config)

	return s, nil
}

func (s *Service) RegisterAdminRoutes(router *mux.Router) {
	s.Handler.RegisterAdminRoutes(router)
}

// Bootstrap runs the idempotent RBAC bootstrap (see bootstrap.go) using the
// configured admin username. Safe to call on every server start.
func (s *Service) Bootstrap(ctx context.Context, userRepo authnzRepo.UserRepository) error {
	return Bootstrap(ctx, s.Store, userRepo, s.config.RBAC.AdminUsername)
}

// --- Features ---

func (s *Service) ListFeatures(ctx context.Context) ([]model.Feature, error) {
	return s.Store.FeatureRepo().List(ctx)
}

// --- Groups ---

func (s *Service) ListGroups(ctx context.Context) ([]model.Group, error) {
	groups, err := s.Store.GroupRepo().List(ctx)
	if err != nil {
		return nil, err
	}
	for i := range groups {
		keys, err := s.Store.GroupFeatureRepo().ListKeysByGroup(ctx, groups[i].ID)
		if err != nil {
			return nil, err
		}
		groups[i].FeatureKeys = keys
	}
	return groups, nil
}

func (s *Service) GetGroup(ctx context.Context, id uuid.UUID) (model.Group, error) {
	group, err := s.Store.GroupRepo().Get(ctx, id)
	if err != nil {
		return model.Group{}, err
	}
	keys, err := s.Store.GroupFeatureRepo().ListKeysByGroup(ctx, id)
	if err != nil {
		return model.Group{}, err
	}
	group.FeatureKeys = keys
	return group, nil
}

func (s *Service) CreateGroup(ctx context.Context, name, description string, featureKeys []string) (model.Group, error) {
	featureIDs, err := s.resolveFeatureIDs(ctx, featureKeys)
	if err != nil {
		return model.Group{}, err
	}

	group := model.Group{ID: uuid.New(), Name: name, Description: description}
	if err := s.Store.GroupRepo().Create(ctx, group); err != nil {
		return model.Group{}, err
	}

	if len(featureIDs) > 0 {
		if err := s.Store.GroupFeatureRepo().ReplaceGrants(ctx, group.ID, featureIDs); err != nil {
			return model.Group{}, err
		}
	}

	group.FeatureKeys = featureKeys
	return group, nil
}

// UpdateGroup updates a group's name/description. Renaming a built-in group
// is rejected (rbac.ErrBuiltinGroup); its description may still be edited.
func (s *Service) UpdateGroup(ctx context.Context, id uuid.UUID, name, description string) (model.Group, error) {
	existing, err := s.Store.GroupRepo().Get(ctx, id)
	if err != nil {
		return model.Group{}, err
	}
	if existing.IsBuiltin && name != existing.Name {
		return model.Group{}, fmt.Errorf("%w: cannot rename a built-in group", rbac.ErrBuiltinGroup)
	}

	existing.Name = name
	existing.Description = description
	if err := s.Store.GroupRepo().Update(ctx, existing); err != nil {
		return model.Group{}, err
	}

	keys, err := s.Store.GroupFeatureRepo().ListKeysByGroup(ctx, id)
	if err != nil {
		return model.Group{}, err
	}
	existing.FeatureKeys = keys
	return existing, nil
}

func (s *Service) DeleteGroup(ctx context.Context, id uuid.UUID) error {
	group, err := s.Store.GroupRepo().Get(ctx, id)
	if err != nil {
		return err
	}
	if group.IsBuiltin {
		return fmt.Errorf("%w: cannot delete a built-in group", rbac.ErrBuiltinGroup)
	}
	return s.Store.GroupRepo().Delete(ctx, id)
}

// SetGroupFeatures replaces a group's feature grants. Editing the admin
// group's grants is rejected — admin bypasses all feature gates, so its
// grants are never consulted and editing them would be a silent no-op.
func (s *Service) SetGroupFeatures(ctx context.Context, id uuid.UUID, featureKeys []string) (model.Group, error) {
	group, err := s.Store.GroupRepo().Get(ctx, id)
	if err != nil {
		return model.Group{}, err
	}
	if group.Name == rbac.GroupAdmin {
		return model.Group{}, fmt.Errorf("%w: admin bypasses all feature gates, so its grants cannot be edited", rbac.ErrBuiltinGroup)
	}

	featureIDs, err := s.resolveFeatureIDs(ctx, featureKeys)
	if err != nil {
		return model.Group{}, err
	}

	trx, err := s.Store.CreateTrx(ctx)
	if err != nil {
		return model.Group{}, err
	}
	defer trx.Rollback()

	if err := s.Store.GroupFeatureRepo().ReplaceGrants(ctx, id, featureIDs, trx); err != nil {
		return model.Group{}, err
	}
	if err := trx.Commit(); err != nil {
		return model.Group{}, err
	}

	group.FeatureKeys = featureKeys
	return group, nil
}

func (s *Service) resolveFeatureIDs(ctx context.Context, keys []string) ([]uuid.UUID, error) {
	ids := make([]uuid.UUID, 0, len(keys))
	for _, key := range keys {
		f, err := s.Store.FeatureRepo().GetByKey(ctx, key)
		if err != nil {
			return nil, fmt.Errorf("feature %q: %w", key, err)
		}
		ids = append(ids, f.ID)
	}
	return ids, nil
}

// --- Users ---

func (s *Service) ListUsers(ctx context.Context) ([]model.UserAccessRow, error) {
	rows, err := s.Store.UserGroupRepo().ListUsersWithGroup(ctx)
	if err != nil {
		return nil, err
	}
	for i := range rows {
		rows[i].IsAdmin = rows[i].GroupName != nil && *rows[i].GroupName == rbac.GroupAdmin
	}
	return rows, nil
}

// AssignUserGroup reassigns a user's group. If the user currently belongs to
// the admin group and is being moved away from it, the reassignment is
// rejected (rbac.ErrLastAdmin) unless another admin remains — see
// docs/adr/ACCESS_MANAGEMENT_ADR.md ADR-004.
func (s *Service) AssignUserGroup(ctx context.Context, userID uuid.UUID, groupID *uuid.UUID) error {
	if groupID != nil {
		if _, err := s.Store.GroupRepo().Get(ctx, *groupID); err != nil {
			return err
		}
	}

	currentGroupID, err := s.Store.UserGroupRepo().GetGroupID(ctx, userID)
	if err != nil {
		return err
	}

	if currentGroupID != nil {
		currentGroup, err := s.Store.GroupRepo().Get(ctx, *currentGroupID)
		if err != nil {
			return err
		}

		movingAway := groupID == nil || *groupID != *currentGroupID
		if currentGroup.Name == rbac.GroupAdmin && movingAway {
			count, err := s.Store.UserGroupRepo().CountByGroup(ctx, *currentGroupID)
			if err != nil {
				return err
			}
			if count <= 1 {
				return rbac.ErrLastAdmin
			}
		}
	}

	return s.Store.UserGroupRepo().AssignGroup(ctx, userID, groupID)
}

// --- Overrides ---

func (s *Service) ListUserOverrides(ctx context.Context, userID uuid.UUID) ([]model.FeatureOverrideView, error) {
	overrides, err := s.Store.OverrideRepo().ListByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	features, err := s.Store.FeatureRepo().List(ctx)
	if err != nil {
		return nil, err
	}
	keyByID := make(map[uuid.UUID]string, len(features))
	for _, f := range features {
		keyByID[f.ID] = f.Key
	}

	out := make([]model.FeatureOverrideView, 0, len(overrides))
	for _, ov := range overrides {
		out = append(out, model.FeatureOverrideView{FeatureKey: keyByID[ov.FeatureID], Allow: ov.Allow})
	}
	return out, nil
}

func (s *Service) SetUserOverrides(ctx context.Context, userID uuid.UUID, overrides []model.FeatureOverrideView) error {
	rows := make([]model.UserFeatureOverride, 0, len(overrides))
	for _, ov := range overrides {
		f, err := s.Store.FeatureRepo().GetByKey(ctx, ov.FeatureKey)
		if err != nil {
			return fmt.Errorf("feature %q: %w", ov.FeatureKey, err)
		}
		rows = append(rows, model.UserFeatureOverride{FeatureID: f.ID, Allow: ov.Allow})
	}

	trx, err := s.Store.CreateTrx(ctx)
	if err != nil {
		return err
	}
	defer trx.Rollback()

	if err := s.Store.OverrideRepo().ReplaceForUser(ctx, userID, rows, trx); err != nil {
		return err
	}
	return trx.Commit()
}
