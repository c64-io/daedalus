package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/c64-io/daedalus/internal/core/domain"
	"github.com/c64-io/daedalus/internal/core/port"
)

// Sentinel errors for StoryService.
var (
	ErrStoryTitleRequired   = errors.New("story title is required")
	ErrStoryFeatureRequired = errors.New("parent feature is required")
	ErrFeatureNotReady      = errors.New("parent feature must be at least refined to create stories")
)

// minFeatureStatusForStory is the set of Feature statuses that allow
// child Story creation. The feature must be at least refined.
var minFeatureStatusForStory = map[domain.Status]bool{
	domain.StatusRefined:    true,
	domain.StatusReady:      true,
	domain.StatusInProgress: true,
	domain.StatusReview:     true,
	domain.StatusDone:       true,
}

// Compile-time assertions.
var (
	_ port.StoryCreator = (*StoryService)(nil)
	_ port.StoryReader  = (*StoryService)(nil)
	_ port.StorySetter  = (*StoryService)(nil)
)

// StoryService implements the StoryCreator, StoryReader, and
// StorySetter use cases.
type StoryService struct {
	fs          port.FileSystem
	storyRepo   port.StoryRepository
	featureRepo port.FeatureRepository
	history     port.HistoryRepository
}

// NewStoryService wires the service with its driven dependencies.
func NewStoryService(fs port.FileSystem, storyRepo port.StoryRepository, featureRepo port.FeatureRepository, history port.HistoryRepository) *StoryService {
	return &StoryService{fs: fs, storyRepo: storyRepo, featureRepo: featureRepo, history: history}
}

// CreateStory validates the request, checks the parent Feature exists
// and is at least refined, mints the next STORY-XXX ID, and persists
// the new Story with status "draft".
func (s *StoryService) CreateStory(ctx context.Context, req port.CreateStoryRequest) (*domain.Story, error) {
	if req.FeatureID == "" {
		return nil, ErrStoryFeatureRequired
	}
	if req.Title == "" {
		return nil, ErrStoryTitleRequired
	}

	ws, err := requireWorkspace(s.fs, req.RootDir)
	if err != nil {
		return nil, err
	}

	// Validate parent feature exists and is at least refined.
	feature, err := s.featureRepo.GetFeature(ctx, ws.DBDir, req.FeatureID)
	if err != nil {
		return nil, fmt.Errorf("validate parent feature: %w", err)
	}
	if !minFeatureStatusForStory[feature.Status] {
		return nil, fmt.Errorf("%w: %s is %s", ErrFeatureNotReady, feature.ID, feature.Status)
	}

	seq, err := s.storyRepo.NextStorySeq(ctx, ws.DBDir)
	if err != nil {
		return nil, fmt.Errorf("mint story ID: %w", err)
	}

	story := domain.Story{
		ID:          domain.FormatStoryID(seq),
		FeatureID:   req.FeatureID,
		Title:       req.Title,
		Description: req.Description,
		Status:      domain.StatusDraft,
		Priority:    req.Priority,
		Size:        req.Size,
		CreatedAt:   time.Now(),
	}

	if err := s.storyRepo.SaveStory(ctx, ws.DBDir, story); err != nil {
		return nil, fmt.Errorf("save story: %w", err)
	}

	return &story, nil
}

// GetStory resolves the workspace and delegates to the repository.
func (s *StoryService) GetStory(ctx context.Context, rootDir string, id string) (*domain.Story, error) {
	ws, err := requireWorkspace(s.fs, rootDir)
	if err != nil {
		return nil, err
	}
	story, err := s.storyRepo.GetStory(ctx, ws.DBDir, id)
	if err != nil {
		return nil, fmt.Errorf("get story: %w", err)
	}
	return story, nil
}

// ListStories resolves the workspace and delegates to the repository.
// When featureID is empty, all stories are returned.
func (s *StoryService) ListStories(ctx context.Context, rootDir string, featureID string) ([]domain.Story, error) {
	ws, err := requireWorkspace(s.fs, rootDir)
	if err != nil {
		return nil, err
	}
	stories, err := s.storyRepo.ListStories(ctx, ws.DBDir, featureID)
	if err != nil {
		return nil, fmt.Errorf("list stories: %w", err)
	}
	return stories, nil
}

// SetStory applies the requested field changes to an existing Story,
// validates status transitions, records history entries, and persists.
func (s *StoryService) SetStory(ctx context.Context, req port.SetStoryRequest) (*domain.Story, error) {
	if req.ID == "" {
		return nil, ErrStoryTitleRequired
	}
	if req.Status == nil && req.Title == nil && req.Description == nil && req.Priority == nil && req.Size == nil {
		return nil, ErrNoFieldsToSet
	}

	ws, err := requireWorkspace(s.fs, req.RootDir)
	if err != nil {
		return nil, err
	}

	story, err := s.storyRepo.GetStory(ctx, ws.DBDir, req.ID)
	if err != nil {
		return nil, fmt.Errorf("get story: %w", err)
	}

	now := time.Now()
	var entries []domain.HistoryEntry

	if req.Status != nil && *req.Status != story.Status {
		newStatus, err := story.Status.Transition(*req.Status)
		if err != nil {
			return nil, err
		}
		entries = append(entries, domain.HistoryEntry{
			EntityID: story.ID, Field: "status",
			OldValue: string(story.Status), NewValue: string(newStatus),
			Timestamp: now,
		})
		story.Status = newStatus
	}

	if req.Title != nil && *req.Title != story.Title {
		if *req.Title == "" {
			return nil, ErrStoryTitleRequired
		}
		entries = append(entries, domain.HistoryEntry{
			EntityID: story.ID, Field: "title",
			OldValue: story.Title, NewValue: *req.Title,
			Timestamp: now,
		})
		story.Title = *req.Title
	}

	if req.Description != nil && *req.Description != story.Description {
		entries = append(entries, domain.HistoryEntry{
			EntityID: story.ID, Field: "description",
			OldValue: story.Description, NewValue: *req.Description,
			Timestamp: now,
		})
		story.Description = *req.Description
	}

	if req.Priority != nil && *req.Priority != story.Priority {
		entries = append(entries, domain.HistoryEntry{
			EntityID: story.ID, Field: "priority",
			OldValue: string(story.Priority), NewValue: string(*req.Priority),
			Timestamp: now,
		})
		story.Priority = *req.Priority
	}

	if req.Size != nil && *req.Size != story.Size {
		entries = append(entries, domain.HistoryEntry{
			EntityID: story.ID, Field: "size",
			OldValue: fmt.Sprintf("%d", story.Size), NewValue: fmt.Sprintf("%d", *req.Size),
			Timestamp: now,
		})
		story.Size = *req.Size
	}

	if len(entries) == 0 {
		return story, nil
	}

	if err := s.storyRepo.UpdateStory(ctx, ws.DBDir, *story); err != nil {
		return nil, fmt.Errorf("update story: %w", err)
	}

	if err := s.history.AppendHistory(ctx, ws.DBDir, entries); err != nil {
		return nil, fmt.Errorf("record history: %w", err)
	}

	return story, nil
}
