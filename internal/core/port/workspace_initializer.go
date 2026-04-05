package port

import (
	"context"

	"github.com/c64-io/daedalus/internal/core/domain"
)

// WorkspaceInitializer is the driving port exposing the "init workspace"
// use case to inbound adapters (e.g. the CLI).
type WorkspaceInitializer interface {
	Init(ctx context.Context, rootDir string) (*domain.Workspace, error)
}
