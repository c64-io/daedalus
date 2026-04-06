package port

import (
	"context"

	"github.com/c64-io/daedalus/internal/core/domain"
)

// WorkspaceStatusReader is the driving port exposing the "show
// workspace status" use case to inbound adapters (e.g. the CLI).
// It is deliberately a separate interface from WorkspaceInitializer
// per the one-verb-per-port rule: the two use cases have nothing in
// common beyond operating on the same entity.
type WorkspaceStatusReader interface {
	// Status resolves the workspace rooted at rootDir (or the current
	// working directory when rootDir is empty) and returns a snapshot
	// of its on-disk location plus persisted metadata. Returns a
	// wrapped ErrWorkspaceNotFound when no workspace exists at the
	// resolved location.
	Status(ctx context.Context, rootDir string) (*domain.WorkspaceStatus, error)
}
