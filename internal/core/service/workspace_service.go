package service

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/c64-io/daedalus/internal/core/domain"
	"github.com/c64-io/daedalus/internal/core/port"
)

// ErrWorkspaceExists is returned when `d7 init` is run in a directory
// that already contains a d7 workspace.
var ErrWorkspaceExists = errors.New("d7 workspace already initialized")

// WorkspaceService implements the WorkspaceInitializer use case.
type WorkspaceService struct {
	repo port.WorkspaceRepository
}

// NewWorkspaceService wires the service with its driven dependencies.
func NewWorkspaceService(repo port.WorkspaceRepository) *WorkspaceService {
	return &WorkspaceService{repo: repo}
}

// Init creates the on-disk workspace layout rooted at rootDir and
// provisions its database via the injected repository.
func (s *WorkspaceService) Init(ctx context.Context, rootDir string) (*domain.Workspace, error) {
	absRoot, err := filepath.Abs(rootDir)
	if err != nil {
		return nil, fmt.Errorf("resolve root dir: %w", err)
	}

	ws := domain.NewWorkspace(absRoot)

	if _, err := os.Stat(ws.Dir); err == nil {
		return nil, fmt.Errorf("%w at %s", ErrWorkspaceExists, ws.Dir)
	} else if !errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("stat workspace dir: %w", err)
	}

	if err := os.MkdirAll(ws.DBDir, 0o755); err != nil {
		return nil, fmt.Errorf("create workspace dirs: %w", err)
	}

	if err := s.repo.CreateDatabase(ctx, ws.DBDir); err != nil {
		return nil, fmt.Errorf("create database: %w", err)
	}

	return ws, nil
}
