package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/c64-io/daedalus/internal/core/domain"
	"github.com/c64-io/daedalus/internal/core/port/driven"
	"github.com/c64-io/daedalus/internal/core/port/driving"
)

// Sentinel errors for SpecService.
var (
	ErrSpecTitleRequired = errors.New("spec title is required")
	ErrSpecStoryRequired = errors.New("parent story is required")
	ErrStoryNotReady     = errors.New("parent story must be at least refined to create specs")
)

// minStoryStatusForSpec is the set of Story statuses that allow
// child Spec creation. The story must be at least refined.
var minStoryStatusForSpec = map[domain.Status]bool{
	domain.StatusRefined:    true,
	domain.StatusReady:      true,
	domain.StatusInProgress: true,
	domain.StatusReview:     true,
	domain.StatusDone:       true,
}

// Compile-time assertions.
var (
	_ driving.SpecCreator = (*SpecService)(nil)
	_ driving.SpecReader  = (*SpecService)(nil)
	_ driving.SpecSetter  = (*SpecService)(nil)
)

// SpecService implements the SpecCreator, SpecReader, and
// SpecSetter use cases.
type SpecService struct {
	fs        driven.FileSystem
	specRepo  driven.SpecRepository
	storyRepo driven.StoryRepository
	history   driven.HistoryRepository
}

// NewSpecService wires the service with its driven dependencies.
func NewSpecService(fs driven.FileSystem, specRepo driven.SpecRepository, storyRepo driven.StoryRepository, history driven.HistoryRepository) *SpecService {
	return &SpecService{fs: fs, specRepo: specRepo, storyRepo: storyRepo, history: history}
}

// CreateSpec validates the request, checks the parent Story exists
// and is at least refined, mints the next SPEC-XXX ID, and persists
// the new Spec with status "draft".
func (s *SpecService) CreateSpec(ctx context.Context, req driving.CreateSpecRequest) (*domain.Spec, error) {
	if req.StoryID == "" {
		return nil, ErrSpecStoryRequired
	}
	if req.Title == "" {
		return nil, ErrSpecTitleRequired
	}

	ws, err := requireWorkspace(s.fs, req.RootDir)
	if err != nil {
		return nil, err
	}

	// Validate parent story exists and is at least refined.
	story, err := s.storyRepo.GetStory(ctx, ws.DBDir, req.StoryID)
	if err != nil {
		return nil, fmt.Errorf("validate parent story: %w", err)
	}
	if !minStoryStatusForSpec[story.Status] {
		return nil, fmt.Errorf("%w: %s is %s", ErrStoryNotReady, story.ID, story.Status)
	}

	seq, err := s.specRepo.NextSpecSeq(ctx, ws.DBDir)
	if err != nil {
		return nil, fmt.Errorf("mint spec ID: %w", err)
	}

	spec := domain.Spec{
		ID:          domain.FormatSpecID(seq),
		StoryID:     req.StoryID,
		Title:       req.Title,
		Description: req.Description,
		Status:      domain.StatusDraft,
		CreatedAt:   time.Now(),
	}

	if err := s.specRepo.SaveSpec(ctx, ws.DBDir, spec); err != nil {
		return nil, fmt.Errorf("save spec: %w", err)
	}

	return &spec, nil
}

// GetSpec resolves the workspace and delegates to the repository.
func (s *SpecService) GetSpec(ctx context.Context, rootDir string, id string) (*domain.Spec, error) {
	ws, err := requireWorkspace(s.fs, rootDir)
	if err != nil {
		return nil, err
	}
	spec, err := s.specRepo.GetSpec(ctx, ws.DBDir, id)
	if err != nil {
		return nil, fmt.Errorf("get spec: %w", err)
	}
	return spec, nil
}

// ListSpecs resolves the workspace and delegates to the repository.
// When storyID is empty, all specs are returned.
func (s *SpecService) ListSpecs(ctx context.Context, rootDir string, storyID string) ([]domain.Spec, error) {
	ws, err := requireWorkspace(s.fs, rootDir)
	if err != nil {
		return nil, err
	}
	specs, err := s.specRepo.ListSpecs(ctx, ws.DBDir, storyID)
	if err != nil {
		return nil, fmt.Errorf("list specs: %w", err)
	}
	return specs, nil
}

// SetSpec applies the requested field changes to an existing Spec,
// validates status transitions, records history entries, and persists.
func (s *SpecService) SetSpec(ctx context.Context, req driving.SetSpecRequest) (*domain.Spec, error) {
	if req.ID == "" {
		return nil, ErrSpecTitleRequired
	}
	if req.Status == nil && req.Title == nil && req.Description == nil {
		return nil, ErrNoFieldsToSet
	}

	ws, err := requireWorkspace(s.fs, req.RootDir)
	if err != nil {
		return nil, err
	}

	spec, err := s.specRepo.GetSpec(ctx, ws.DBDir, req.ID)
	if err != nil {
		return nil, fmt.Errorf("get spec: %w", err)
	}

	now := time.Now()
	var entries []domain.HistoryEntry

	if req.Status != nil && *req.Status != spec.Status {
		newStatus, err := spec.Status.Transition(*req.Status)
		if err != nil {
			return nil, err
		}
		entries = append(entries, domain.HistoryEntry{
			EntityID: spec.ID, Field: "status",
			OldValue: string(spec.Status), NewValue: string(newStatus),
			Timestamp: now,
		})
		spec.Status = newStatus
	}

	if req.Title != nil && *req.Title != spec.Title {
		if *req.Title == "" {
			return nil, ErrSpecTitleRequired
		}
		entries = append(entries, domain.HistoryEntry{
			EntityID: spec.ID, Field: "title",
			OldValue: spec.Title, NewValue: *req.Title,
			Timestamp: now,
		})
		spec.Title = *req.Title
	}

	if req.Description != nil && *req.Description != spec.Description {
		entries = append(entries, domain.HistoryEntry{
			EntityID: spec.ID, Field: "description",
			OldValue: spec.Description, NewValue: *req.Description,
			Timestamp: now,
		})
		spec.Description = *req.Description
	}

	if len(entries) == 0 {
		return spec, nil
	}

	if err := s.specRepo.UpdateSpec(ctx, ws.DBDir, *spec); err != nil {
		return nil, fmt.Errorf("update spec: %w", err)
	}

	if err := s.history.AppendHistory(ctx, ws.DBDir, entries); err != nil {
		return nil, fmt.Errorf("record history: %w", err)
	}

	return spec, nil
}
