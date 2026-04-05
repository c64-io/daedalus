package port

import "context"

// WorkspaceRepository is the driven port for persistent storage of a
// d7 workspace. Implementations provision and manage the underlying
// database backing the workspace.
type WorkspaceRepository interface {
	// CreateDatabase initializes a fresh database at the given directory.
	CreateDatabase(ctx context.Context, dbDir string) error
}
