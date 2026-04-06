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

// Compile-time assertions that IdeaService satisfies both driving
// ports.
var (
	_ port.IdeaCreator = (*IdeaService)(nil)
	_ port.IdeaReader  = (*IdeaService)(nil)
)

// IdeaService implements the IdeaCreator and IdeaReader use cases.
type IdeaService struct {
	fs   port.FileSystem
	repo port.IdeaRepository
}

// NewIdeaService wires the service with its driven dependencies.
func NewIdeaService(fs port.FileSystem, repo port.IdeaRepository) *IdeaService {
	return &IdeaService{fs: fs, repo: repo}
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
