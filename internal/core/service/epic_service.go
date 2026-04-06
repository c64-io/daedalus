package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/c64-io/daedalus/internal/core/domain"
	"github.com/c64-io/daedalus/internal/core/port"
)

// Sentinel errors for EpicService.
var (
	ErrEpicTitleRequired = errors.New("epic title is required")
	ErrEpicIdeaRequired  = errors.New("parent idea is required")
	ErrIdeaNotReady      = errors.New("parent idea must be at least refined to create epics")
)

// minIdeaStatusForEpic is the set of Idea statuses that allow child
// Epic creation. The idea must be at least refined.
var minIdeaStatusForEpic = map[domain.Status]bool{
	domain.StatusRefined:    true,
	domain.StatusReady:      true,
	domain.StatusInProgress: true,
	domain.StatusReview:     true,
	domain.StatusDone:       true,
}

// Compile-time assertions.
var (
	_ port.EpicCreator = (*EpicService)(nil)
	_ port.EpicReader  = (*EpicService)(nil)
	_ port.EpicSetter  = (*EpicService)(nil)
)

// EpicService implements the EpicCreator, EpicReader, and EpicSetter
// use cases.
type EpicService struct {
	fs       port.FileSystem
	epicRepo port.EpicRepository
	ideaRepo port.IdeaRepository
	history  port.HistoryRepository
}

// NewEpicService wires the service with its driven dependencies.
func NewEpicService(fs port.FileSystem, epicRepo port.EpicRepository, ideaRepo port.IdeaRepository, history port.HistoryRepository) *EpicService {
	return &EpicService{fs: fs, epicRepo: epicRepo, ideaRepo: ideaRepo, history: history}
}

// CreateEpic validates the request, checks the parent Idea exists and
// is at least refined, mints the next EPIC-XXX ID, and persists the
// new Epic with status "draft".
func (s *EpicService) CreateEpic(ctx context.Context, req port.CreateEpicRequest) (*domain.Epic, error) {
	if req.IdeaID == "" {
		return nil, ErrEpicIdeaRequired
	}
	if req.Title == "" {
		return nil, ErrEpicTitleRequired
	}

	ws, err := requireWorkspace(s.fs, req.RootDir)
	if err != nil {
		return nil, err
	}

	// Validate parent idea exists and is at least refined.
	idea, err := s.ideaRepo.GetIdea(ctx, ws.DBDir, req.IdeaID)
	if err != nil {
		return nil, fmt.Errorf("validate parent idea: %w", err)
	}
	if !minIdeaStatusForEpic[idea.Status] {
		return nil, fmt.Errorf("%w: %s is %s", ErrIdeaNotReady, idea.ID, idea.Status)
	}

	seq, err := s.epicRepo.NextEpicSeq(ctx, ws.DBDir)
	if err != nil {
		return nil, fmt.Errorf("mint epic ID: %w", err)
	}

	epic := domain.Epic{
		ID:          domain.FormatEpicID(seq),
		IdeaID:      req.IdeaID,
		Title:       req.Title,
		Description: req.Description,
		Status:      domain.StatusDraft,
		Priority:    req.Priority,
		Size:        req.Size,
		CreatedAt:   time.Now(),
	}

	if err := s.epicRepo.SaveEpic(ctx, ws.DBDir, epic); err != nil {
		return nil, fmt.Errorf("save epic: %w", err)
	}

	return &epic, nil
}

// GetEpic resolves the workspace and delegates to the repository.
func (s *EpicService) GetEpic(ctx context.Context, rootDir string, id string) (*domain.Epic, error) {
	ws, err := requireWorkspace(s.fs, rootDir)
	if err != nil {
		return nil, err
	}
	epic, err := s.epicRepo.GetEpic(ctx, ws.DBDir, id)
	if err != nil {
		return nil, fmt.Errorf("get epic: %w", err)
	}
	return epic, nil
}

// ListEpics resolves the workspace and delegates to the repository.
// When ideaID is empty, all epics are returned.
func (s *EpicService) ListEpics(ctx context.Context, rootDir string, ideaID string) ([]domain.Epic, error) {
	ws, err := requireWorkspace(s.fs, rootDir)
	if err != nil {
		return nil, err
	}
	epics, err := s.epicRepo.ListEpics(ctx, ws.DBDir, ideaID)
	if err != nil {
		return nil, fmt.Errorf("list epics: %w", err)
	}
	return epics, nil
}

// SetEpic applies the requested field changes to an existing Epic,
// validates status transitions, records history entries, and persists.
func (s *EpicService) SetEpic(ctx context.Context, req port.SetEpicRequest) (*domain.Epic, error) {
	if req.ID == "" {
		return nil, ErrEpicTitleRequired
	}
	if req.Status == nil && req.Title == nil && req.Description == nil && req.Priority == nil && req.Size == nil {
		return nil, ErrNoFieldsToSet
	}

	ws, err := requireWorkspace(s.fs, req.RootDir)
	if err != nil {
		return nil, err
	}

	epic, err := s.epicRepo.GetEpic(ctx, ws.DBDir, req.ID)
	if err != nil {
		return nil, fmt.Errorf("get epic: %w", err)
	}

	now := time.Now()
	var entries []domain.HistoryEntry

	if req.Status != nil && *req.Status != epic.Status {
		newStatus, err := epic.Status.Transition(*req.Status)
		if err != nil {
			return nil, err
		}
		entries = append(entries, domain.HistoryEntry{
			EntityID: epic.ID, Field: "status",
			OldValue: string(epic.Status), NewValue: string(newStatus),
			Timestamp: now,
		})
		epic.Status = newStatus
	}

	if req.Title != nil && *req.Title != epic.Title {
		if *req.Title == "" {
			return nil, ErrEpicTitleRequired
		}
		entries = append(entries, domain.HistoryEntry{
			EntityID: epic.ID, Field: "title",
			OldValue: epic.Title, NewValue: *req.Title,
			Timestamp: now,
		})
		epic.Title = *req.Title
	}

	if req.Description != nil && *req.Description != epic.Description {
		entries = append(entries, domain.HistoryEntry{
			EntityID: epic.ID, Field: "description",
			OldValue: epic.Description, NewValue: *req.Description,
			Timestamp: now,
		})
		epic.Description = *req.Description
	}

	if req.Priority != nil && *req.Priority != epic.Priority {
		entries = append(entries, domain.HistoryEntry{
			EntityID: epic.ID, Field: "priority",
			OldValue: string(epic.Priority), NewValue: string(*req.Priority),
			Timestamp: now,
		})
		epic.Priority = *req.Priority
	}

	if req.Size != nil && *req.Size != epic.Size {
		entries = append(entries, domain.HistoryEntry{
			EntityID: epic.ID, Field: "size",
			OldValue: fmt.Sprintf("%d", epic.Size), NewValue: fmt.Sprintf("%d", *req.Size),
			Timestamp: now,
		})
		epic.Size = *req.Size
	}

	if len(entries) == 0 {
		return epic, nil
	}

	if err := s.epicRepo.UpdateEpic(ctx, ws.DBDir, *epic); err != nil {
		return nil, fmt.Errorf("update epic: %w", err)
	}

	if err := s.history.AppendHistory(ctx, ws.DBDir, entries); err != nil {
		return nil, fmt.Errorf("record history: %w", err)
	}

	return epic, nil
}
