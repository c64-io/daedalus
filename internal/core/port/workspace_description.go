package port

import (
	"context"

	"github.com/c64-io/daedalus/internal/core/domain"
)

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
