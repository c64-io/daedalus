package driving

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

// WorkspaceStatusReader is the driving port exposing the "show
// workspace status" use case to inbound adapters (e.g. the CLI).
type WorkspaceStatusReader interface {
	Status(ctx context.Context, rootDir string) (*domain.WorkspaceStatus, error)
}

// WorkspaceDescriptionReader is a driving port for reading the
// workspace's project description from the database.
type WorkspaceDescriptionReader interface {
	ReadWorkspaceDescription(ctx context.Context, rootDir string) (*domain.ProjectDescription, error)
}

// WorkspaceDescriptionWriter is a driving port for writing the
// workspace's project description to the database.
type WorkspaceDescriptionWriter interface {
	WriteWorkspaceDescription(ctx context.Context, rootDir string, content string) error
}

// Workspace is the combined driving surface for the workspace
// subcommand group. Individual CLI subcommands still take the narrow
// port they actually need (interface segregation); the bundle exists
// only to keep root-command wiring flat.
type Workspace interface {
	WorkspaceInitializer
	WorkspaceStatusReader
	WorkspaceDescriptionReader
	WorkspaceDescriptionWriter
}
