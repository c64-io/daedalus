package service

import (
	"errors"
	"fmt"
	"io/fs"

	"github.com/c64-io/daedalus/internal/core/domain"
	"github.com/c64-io/daedalus/internal/core/port/driven"
)

// resolveWorkspace resolves rootDir (or cwd when rootDir is empty)
// into a Workspace descriptor. It does not check whether the
// workspace actually exists on disk — callers must do that
// themselves based on their use case (Init checks it does NOT exist;
// Status/Idea commands check it DOES).
func resolveWorkspace(filesystem driven.FileSystem, rootDir string) (*domain.Workspace, error) {
	if rootDir == "" || rootDir == "." {
		cwd, err := filesystem.Getwd()
		if err != nil {
			return nil, fmt.Errorf("resolve cwd: %w", err)
		}
		rootDir = cwd
	}

	absRoot, err := filesystem.Abs(rootDir)
	if err != nil {
		return nil, fmt.Errorf("resolve root dir: %w", err)
	}

	return domain.NewWorkspace(absRoot), nil
}

// requireWorkspace resolves rootDir into a Workspace and confirms
// the workspace's database directory exists. Returns
// ErrWorkspaceNotFound when it does not.
func requireWorkspace(filesystem driven.FileSystem, rootDir string) (*domain.Workspace, error) {
	ws, err := resolveWorkspace(filesystem, rootDir)
	if err != nil {
		return nil, err
	}

	if _, err := filesystem.Stat(ws.DBDir); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("%w at %s", ErrWorkspaceNotFound, ws.Dir)
		}
		return nil, fmt.Errorf("stat workspace db dir: %w", err)
	}

	return ws, nil
}
