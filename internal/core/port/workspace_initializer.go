package port

import (
	"context"

	"github.com/c64-io/daedalus/internal/core/domain"
)

// InitRequest is the input shape for the WorkspaceInitializer.Init use
// case. A request struct is used (rather than positional parameters)
// so the use case can grow fields without churning every caller.
type InitRequest struct {
	// RootDir is the directory in which to create the d7 workspace.
	// An empty string or "." resolves to the current working directory
	// via the FileSystem port.
	RootDir string

	// Target is the code-generation and verification ecosystem
	// declared for the workspace. It is immutable for the life of
	// the workspace.
	Target domain.Target
}

// WorkspaceInitializer is the driving port exposing the "init workspace"
// use case to inbound adapters (e.g. the CLI).
type WorkspaceInitializer interface {
	Init(ctx context.Context, req InitRequest) (*domain.Workspace, error)
}
