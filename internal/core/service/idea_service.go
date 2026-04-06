package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/c64-io/daedalus/internal/core/domain"
	"github.com/c64-io/daedalus/internal/core/port"
)

// ErrIdeaTitleRequired is returned when a CreateIdea request has an
// empty title.
var ErrIdeaTitleRequired = errors.New("idea title is required")

// ErrNoFieldsToSet is returned when a set request has no fields to update.
var ErrNoFieldsToSet = errors.New("no fields to set")

// Compile-time assertions.
var (
	_ port.IdeaCreator = (*IdeaService)(nil)
	_ port.IdeaReader  = (*IdeaService)(nil)
	_ port.IdeaSetter  = (*IdeaService)(nil)
)

// IdeaService implements the IdeaCreator, IdeaReader, and IdeaSetter
// use cases.
type IdeaService struct {
	fs      port.FileSystem
	repo    port.IdeaRepository
	history port.HistoryRepository
}

// NewIdeaService wires the service with its driven dependencies.
func NewIdeaService(fs port.FileSystem, repo port.IdeaRepository, history port.HistoryRepository) *IdeaService {
	return &IdeaService{fs: fs, repo: repo, history: history}
}

// CreateIdea validates the request, mints the next IDEA-XXX ID via
// the repository, and persists the new Idea with status "draft".
func (s *IdeaService) CreateIdea(ctx context.Context, req port.CreateIdeaRequest) (*domain.Idea, error) {
	if req.Title == "" {
		return nil, ErrIdeaTitleRequired
	}

	ws, err := requireWorkspace(s.fs, req.RootDir)
	if err != nil {
		return nil, err
	}

	seq, err := s.repo.NextIdeaSeq(ctx, ws.DBDir)
	if err != nil {
		return nil, fmt.Errorf("mint idea ID: %w", err)
	}

	idea := domain.Idea{
		ID:          domain.FormatIdeaID(seq),
		Title:       req.Title,
		Description: req.Description,
		Status:      domain.StatusDraft,
		CreatedAt:   time.Now(),
	}

	if err := s.repo.SaveIdea(ctx, ws.DBDir, idea); err != nil {
		return nil, fmt.Errorf("save idea: %w", err)
	}

	return &idea, nil
}

// GetIdea resolves the workspace and delegates to the repository.
func (s *IdeaService) GetIdea(ctx context.Context, rootDir string, id string) (*domain.Idea, error) {
	ws, err := requireWorkspace(s.fs, rootDir)
	if err != nil {
		return nil, err
	}
	idea, err := s.repo.GetIdea(ctx, ws.DBDir, id)
	if err != nil {
		return nil, fmt.Errorf("get idea: %w", err)
	}
	return idea, nil
}

// ListIdeas resolves the workspace and delegates to the repository.
func (s *IdeaService) ListIdeas(ctx context.Context, rootDir string) ([]domain.Idea, error) {
	ws, err := requireWorkspace(s.fs, rootDir)
	if err != nil {
		return nil, err
	}
	ideas, err := s.repo.ListIdeas(ctx, ws.DBDir)
	if err != nil {
		return nil, fmt.Errorf("list ideas: %w", err)
	}
	return ideas, nil
}

// SetIdea applies the requested field changes to an existing Idea,
// validates status transitions, records history entries, and persists.
func (s *IdeaService) SetIdea(ctx context.Context, req port.SetIdeaRequest) (*domain.Idea, error) {
	if req.ID == "" {
		return nil, ErrIdeaTitleRequired // reuse: ID is effectively required like title
	}
	if req.Status == nil && req.Title == nil && req.Description == nil {
		return nil, ErrNoFieldsToSet
	}

	ws, err := requireWorkspace(s.fs, req.RootDir)
	if err != nil {
		return nil, err
	}

	idea, err := s.repo.GetIdea(ctx, ws.DBDir, req.ID)
	if err != nil {
		return nil, fmt.Errorf("get idea: %w", err)
	}

	now := time.Now()
	var entries []domain.HistoryEntry

	if req.Status != nil && *req.Status != idea.Status {
		newStatus, err := idea.Status.Transition(*req.Status)
		if err != nil {
			return nil, err
		}
		entries = append(entries, domain.HistoryEntry{
			EntityID: idea.ID, Field: "status",
			OldValue: string(idea.Status), NewValue: string(newStatus),
			Timestamp: now,
		})
		idea.Status = newStatus
	}

	if req.Title != nil && *req.Title != idea.Title {
		if *req.Title == "" {
			return nil, ErrIdeaTitleRequired
		}
		entries = append(entries, domain.HistoryEntry{
			EntityID: idea.ID, Field: "title",
			OldValue: idea.Title, NewValue: *req.Title,
			Timestamp: now,
		})
		idea.Title = *req.Title
	}

	if req.Description != nil && *req.Description != idea.Description {
		entries = append(entries, domain.HistoryEntry{
			EntityID: idea.ID, Field: "description",
			OldValue: idea.Description, NewValue: *req.Description,
			Timestamp: now,
		})
		idea.Description = *req.Description
	}

	if len(entries) == 0 {
		return idea, nil // no actual changes
	}

	if err := s.repo.UpdateIdea(ctx, ws.DBDir, *idea); err != nil {
		return nil, fmt.Errorf("update idea: %w", err)
	}

	if err := s.history.AppendHistory(ctx, ws.DBDir, entries); err != nil {
		return nil, fmt.Errorf("record history: %w", err)
	}

	return idea, nil
}
