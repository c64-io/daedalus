package port

import (
	"context"

	"github.com/c64-io/daedalus/internal/core/domain"
)

// ProjectDescriptionReader is a driving port for reading the
// workspace's project description file (d7/project.md).
type ProjectDescriptionReader interface {
	ReadProjectDescription(ctx context.Context, rootDir string) (*domain.ProjectDescription, error)
}
