package port

import (
	"context"

	"github.com/c64-io/daedalus/internal/core/domain"
)

// WorkspaceRepository is the driven port for persistent storage of a
// d7 workspace. Implementations provision and manage the underlying
// database backing the workspace.
type WorkspaceRepository interface {
	// CreateDatabase initializes a fresh database at the given directory
	// and atomically writes the supplied workspace metadata into it.
	// Implementations must not leave a database behind if metadata
	// persistence fails.
	CreateDatabase(ctx context.Context, dbDir string, meta domain.WorkspaceMetadata) error
}
