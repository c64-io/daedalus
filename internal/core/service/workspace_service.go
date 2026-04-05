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

// ErrNoTargets is returned when a workspace init request carries no
// declared targets. At least one target is required.
var ErrNoTargets = errors.New("at least one target must be declared")

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

// Init creates the on-disk workspace layout rooted at req.RootDir and
// provisions its database via the injected repository, persisting the
// supplied targets as the workspace's immutable metadata.
func (s *WorkspaceService) Init(ctx context.Context, req port.InitRequest) (*domain.Workspace, error) {
	if len(req.Targets) == 0 {
		return nil, ErrNoTargets
	}

	rootDir := req.RootDir
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

	meta := domain.WorkspaceMetadata{Targets: req.Targets}
	if err := s.repo.CreateDatabase(ctx, ws.DBDir, meta); err != nil {
		return nil, fmt.Errorf("create database: %w", err)
	}

	return ws, nil
}
