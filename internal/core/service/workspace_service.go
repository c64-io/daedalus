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

// ErrNoTarget is returned when a workspace init request carries no
// declared target. Exactly one target is required.
var ErrNoTarget = errors.New("a target must be declared")

// ErrWorkspaceNotFound is returned by Status (and any command that
// requires an existing workspace) when no d7 workspace exists at
// the resolved location.
var ErrWorkspaceNotFound = errors.New("d7 workspace not found")

// Compile-time assertions that WorkspaceService satisfies the
// driving ports it implements.
var (
	_ port.WorkspaceInitializer           = (*WorkspaceService)(nil)
	_ port.WorkspaceStatusReader          = (*WorkspaceService)(nil)
	_ port.WorkspaceDescriptionReader     = (*WorkspaceService)(nil)
	_ port.WorkspaceDescriptionWriter     = (*WorkspaceService)(nil)
)

// WorkspaceService implements the WorkspaceInitializer and
// WorkspaceStatusReader use cases. It depends only on driven ports —
// no direct I/O imports — so it is fully unit-testable with in-memory
// fakes.
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
// supplied target as the workspace's immutable metadata.
func (s *WorkspaceService) Init(ctx context.Context, req port.InitRequest) (*domain.Workspace, error) {
	if req.Target == "" {
		return nil, ErrNoTarget
	}

	ws, err := resolveWorkspace(s.fs, req.RootDir)
	if err != nil {
		return nil, err
	}

	if _, err := s.fs.Stat(ws.Dir); err == nil {
		return nil, fmt.Errorf("%w at %s", ErrWorkspaceExists, ws.Dir)
	} else if !errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("stat workspace dir: %w", err)
	}

	if err := s.fs.MkdirAll(ws.DBDir, 0o755); err != nil {
		return nil, fmt.Errorf("create workspace dirs: %w", err)
	}

	meta := domain.WorkspaceMetadata{Target: req.Target}
	if err := s.repo.CreateDatabase(ctx, ws.DBDir, meta); err != nil {
		return nil, fmt.Errorf("create database: %w", err)
	}

	if err := s.repo.SaveProjectDescription(ctx, ws.DBDir, domain.ProjectDescriptionTemplate); err != nil {
		return nil, fmt.Errorf("save project description: %w", err)
	}

	return ws, nil
}

// Status resolves the workspace rooted at rootDir (or the current
// working directory when rootDir is empty) and returns a snapshot of
// its on-disk location plus persisted metadata.
func (s *WorkspaceService) Status(ctx context.Context, rootDir string) (*domain.WorkspaceStatus, error) {
	ws, err := requireWorkspace(s.fs, rootDir)
	if err != nil {
		return nil, err
	}

	meta, err := s.repo.ReadMetadata(ctx, ws.DBDir)
	if err != nil {
		return nil, fmt.Errorf("read workspace metadata: %w", err)
	}

	pd := s.readProjectDescription(ctx, ws)

	return &domain.WorkspaceStatus{
		Workspace:          ws,
		Metadata:           meta,
		ProjectDescription: pd,
	}, nil
}

// ReadWorkspaceDescription reads the workspace's project description
// from the database.
func (s *WorkspaceService) ReadWorkspaceDescription(ctx context.Context, rootDir string) (*domain.ProjectDescription, error) {
	ws, err := requireWorkspace(s.fs, rootDir)
	if err != nil {
		return nil, err
	}

	content, err := s.repo.ReadProjectDescription(ctx, ws.DBDir)
	if err != nil {
		return nil, fmt.Errorf("read project description: %w", err)
	}

	return &domain.ProjectDescription{
		Content:   content,
		IsDefault: content == domain.ProjectDescriptionTemplate,
	}, nil
}

// WriteWorkspaceDescription writes the project description to the
// workspace database.
func (s *WorkspaceService) WriteWorkspaceDescription(ctx context.Context, rootDir string, content string) error {
	ws, err := requireWorkspace(s.fs, rootDir)
	if err != nil {
		return err
	}

	return s.repo.SaveProjectDescription(ctx, ws.DBDir, content)
}

// readProjectDescription is a best-effort helper used by Status. If
// the description cannot be read, it returns nil rather than failing
// the entire status call.
func (s *WorkspaceService) readProjectDescription(ctx context.Context, ws *domain.Workspace) *domain.ProjectDescription {
	content, err := s.repo.ReadProjectDescription(ctx, ws.DBDir)
	if err != nil {
		return nil
	}
	return &domain.ProjectDescription{
		Content:   content,
		IsDefault: content == domain.ProjectDescriptionTemplate,
	}
}
