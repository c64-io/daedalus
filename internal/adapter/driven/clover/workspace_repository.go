package clover

import (
	"context"
	"fmt"

	c "github.com/ostafen/clover/v2"
)

// WorkspaceRepository is the Clover v2 implementation of
// port.WorkspaceRepository. It provisions the directory-backed store
// used by a d7 workspace.
type WorkspaceRepository struct{}

// NewWorkspaceRepository returns a Clover-backed workspace repository.
func NewWorkspaceRepository() *WorkspaceRepository {
	return &WorkspaceRepository{}
}

// CreateDatabase opens (and thereby creates) a Clover store at dbDir
// and closes it cleanly, leaving the on-disk files ready for use.
func (r *WorkspaceRepository) CreateDatabase(_ context.Context, dbDir string) error {
	db, err := c.Open(dbDir)
	if err != nil {
		return fmt.Errorf("open clover db: %w", err)
	}
	if err := db.Close(); err != nil {
		return fmt.Errorf("close clover db: %w", err)
	}
	return nil
}
