package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/c64-io/daedalus/internal/core/domain"
	"github.com/c64-io/daedalus/internal/core/port"
)

// Sentinel errors for FeatureService.
var (
	ErrFeatureTitleRequired = errors.New("feature title is required")
	ErrFeatureEpicRequired  = errors.New("parent epic is required")
	ErrEpicNotReady         = errors.New("parent epic must be at least refined to create features")
)

// minEpicStatusForFeature is the set of Epic statuses that allow child
// Feature creation. The epic must be at least refined.
var minEpicStatusForFeature = map[domain.Status]bool{
	domain.StatusRefined:    true,
	domain.StatusReady:      true,
	domain.StatusInProgress: true,
	domain.StatusReview:     true,
	domain.StatusDone:       true,
}

// Compile-time assertions.
var (
	_ port.FeatureCreator = (*FeatureService)(nil)
	_ port.FeatureReader  = (*FeatureService)(nil)
	_ port.FeatureSetter  = (*FeatureService)(nil)
)

// FeatureService implements the FeatureCreator, FeatureReader, and
// FeatureSetter use cases.
type FeatureService struct {
	fs          port.FileSystem
	featureRepo port.FeatureRepository
	epicRepo    port.EpicRepository
	history     port.HistoryRepository
}

// NewFeatureService wires the service with its driven dependencies.
func NewFeatureService(fs port.FileSystem, featureRepo port.FeatureRepository, epicRepo port.EpicRepository, history port.HistoryRepository) *FeatureService {
	return &FeatureService{fs: fs, featureRepo: featureRepo, epicRepo: epicRepo, history: history}
}

// CreateFeature validates the request, checks the parent Epic exists
// and is at least refined, mints the next FEAT-XXX ID, and persists
// the new Feature with status "draft".
func (s *FeatureService) CreateFeature(ctx context.Context, req port.CreateFeatureRequest) (*domain.Feature, error) {
	if req.EpicID == "" {
		return nil, ErrFeatureEpicRequired
	}
	if req.Title == "" {
		return nil, ErrFeatureTitleRequired
	}

	ws, err := requireWorkspace(s.fs, req.RootDir)
	if err != nil {
		return nil, err
	}

	// Validate parent epic exists and is at least refined.
	epic, err := s.epicRepo.GetEpic(ctx, ws.DBDir, req.EpicID)
	if err != nil {
		return nil, fmt.Errorf("validate parent epic: %w", err)
	}
	if !minEpicStatusForFeature[epic.Status] {
		return nil, fmt.Errorf("%w: %s is %s", ErrEpicNotReady, epic.ID, epic.Status)
	}

	seq, err := s.featureRepo.NextFeatureSeq(ctx, ws.DBDir)
	if err != nil {
		return nil, fmt.Errorf("mint feature ID: %w", err)
	}

	feature := domain.Feature{
		ID:          domain.FormatFeatureID(seq),
		EpicID:      req.EpicID,
		Title:       req.Title,
		Description: req.Description,
		Status:      domain.StatusDraft,
		Priority:    req.Priority,
		Size:        req.Size,
		CreatedAt:   time.Now(),
	}

	if err := s.featureRepo.SaveFeature(ctx, ws.DBDir, feature); err != nil {
		return nil, fmt.Errorf("save feature: %w", err)
	}

	return &feature, nil
}

// GetFeature resolves the workspace and delegates to the repository.
func (s *FeatureService) GetFeature(ctx context.Context, rootDir string, id string) (*domain.Feature, error) {
	ws, err := requireWorkspace(s.fs, rootDir)
	if err != nil {
		return nil, err
	}
	feature, err := s.featureRepo.GetFeature(ctx, ws.DBDir, id)
	if err != nil {
		return nil, fmt.Errorf("get feature: %w", err)
	}
	return feature, nil
}

// ListFeatures resolves the workspace and delegates to the repository.
// When epicID is empty, all features are returned.
func (s *FeatureService) ListFeatures(ctx context.Context, rootDir string, epicID string) ([]domain.Feature, error) {
	ws, err := requireWorkspace(s.fs, rootDir)
	if err != nil {
		return nil, err
	}
	features, err := s.featureRepo.ListFeatures(ctx, ws.DBDir, epicID)
	if err != nil {
		return nil, fmt.Errorf("list features: %w", err)
	}
	return features, nil
}

// SetFeature applies the requested field changes to an existing Feature,
// validates status transitions, records history entries, and persists.
func (s *FeatureService) SetFeature(ctx context.Context, req port.SetFeatureRequest) (*domain.Feature, error) {
	if req.ID == "" {
		return nil, ErrFeatureTitleRequired
	}
	if req.Status == nil && req.Title == nil && req.Description == nil && req.Priority == nil && req.Size == nil {
		return nil, ErrNoFieldsToSet
	}

	ws, err := requireWorkspace(s.fs, req.RootDir)
	if err != nil {
		return nil, err
	}

	feature, err := s.featureRepo.GetFeature(ctx, ws.DBDir, req.ID)
	if err != nil {
		return nil, fmt.Errorf("get feature: %w", err)
	}

	now := time.Now()
	var entries []domain.HistoryEntry

	if req.Status != nil && *req.Status != feature.Status {
		newStatus, err := feature.Status.Transition(*req.Status)
		if err != nil {
			return nil, err
		}
		entries = append(entries, domain.HistoryEntry{
			EntityID: feature.ID, Field: "status",
			OldValue: string(feature.Status), NewValue: string(newStatus),
			Timestamp: now,
		})
		feature.Status = newStatus
	}

	if req.Title != nil && *req.Title != feature.Title {
		if *req.Title == "" {
			return nil, ErrFeatureTitleRequired
		}
		entries = append(entries, domain.HistoryEntry{
			EntityID: feature.ID, Field: "title",
			OldValue: feature.Title, NewValue: *req.Title,
			Timestamp: now,
		})
		feature.Title = *req.Title
	}

	if req.Description != nil && *req.Description != feature.Description {
		entries = append(entries, domain.HistoryEntry{
			EntityID: feature.ID, Field: "description",
			OldValue: feature.Description, NewValue: *req.Description,
			Timestamp: now,
		})
		feature.Description = *req.Description
	}

	if req.Priority != nil && *req.Priority != feature.Priority {
		entries = append(entries, domain.HistoryEntry{
			EntityID: feature.ID, Field: "priority",
			OldValue: string(feature.Priority), NewValue: string(*req.Priority),
			Timestamp: now,
		})
		feature.Priority = *req.Priority
	}

	if req.Size != nil && *req.Size != feature.Size {
		entries = append(entries, domain.HistoryEntry{
			EntityID: feature.ID, Field: "size",
			OldValue: fmt.Sprintf("%d", feature.Size), NewValue: fmt.Sprintf("%d", *req.Size),
			Timestamp: now,
		})
		feature.Size = *req.Size
	}

	if len(entries) == 0 {
		return feature, nil
	}

	if err := s.featureRepo.UpdateFeature(ctx, ws.DBDir, *feature); err != nil {
		return nil, fmt.Errorf("update feature: %w", err)
	}

	if err := s.history.AppendHistory(ctx, ws.DBDir, entries); err != nil {
		return nil, fmt.Errorf("record history: %w", err)
	}

	return feature, nil
}
