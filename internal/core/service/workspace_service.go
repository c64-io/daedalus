package service

import (
	"context"
	"errors"
	"fmt"
	"io/fs"

	"github.com/c64-io/daedalus/internal/core/domain"
	"github.com/c64-io/daedalus/internal/core/port"
)

// ErrWorkspaceExists is returned when `d7 init` is run in a directory
// that already contains a d7 workspace.
var ErrWorkspaceExists = errors.New("d7 workspace already initialized")

// Compile-time assertion that WorkspaceService satisfies the driving port.
var _ port.WorkspaceInitializer = (*WorkspaceService)(nil)

// WorkspaceService implements the WorkspaceInitializer use case.
// It depends only on driven ports — no direct I/O imports — so it is
// fully unit-testable with in-memory fakes.
type WorkspaceService struct {
	fs   port.FileSystem
	repo port.WorkspaceRepository
}

// NewWorkspaceService wires the service with its driven dependencies.
func NewWorkspaceService(fs port.FileSystem, repo port.WorkspaceRepository) *WorkspaceService {
	return &WorkspaceService{fs: fs, repo: repo}
}

// Init creates the on-disk workspace layout rooted at rootDir and
// provisions its database via the injected repository.
func (s *WorkspaceService) Init(ctx context.Context, rootDir string) (*domain.Workspace, error) {
	if rootDir == "" || rootDir == "." {
		cwd, err := s.fs.Getwd()
		if err != nil {
			return nil, fmt.Errorf("resolve cwd: %w", err)
		}
		rootDir = cwd
	}

	absRoot, err := s.fs.Abs(rootDir)
	if err != nil {
		return nil, fmt.Errorf("resolve root dir: %w", err)
	}

	ws := domain.NewWorkspace(absRoot)

	if _, err := s.fs.Stat(ws.Dir); err == nil {
		return nil, fmt.Errorf("%w at %s", ErrWorkspaceExists, ws.Dir)
	} else if !errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("stat workspace dir: %w", err)
	}

	if err := s.fs.MkdirAll(ws.DBDir, 0o755); err != nil {
		return nil, fmt.Errorf("create workspace dirs: %w", err)
	}

	if err := s.repo.CreateDatabase(ctx, ws.DBDir); err != nil {
		return nil, fmt.Errorf("create database: %w", err)
	}

	return ws, nil
}
