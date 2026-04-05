package clover

import (
	"context"
	"fmt"

	c "github.com/ostafen/clover/v2"
	d "github.com/ostafen/clover/v2/document"

	"github.com/c64-io/daedalus/internal/core/domain"
	"github.com/c64-io/daedalus/internal/core/port"
)

// Collection and field names for the workspace metadata document.
// These are the Clover adapter's concern, not the core's.
const (
	workspaceCollection = "workspace"
	fieldTarget         = "target"
)

// Compile-time assertion that WorkspaceRepository satisfies the driven port.
var _ port.WorkspaceRepository = (*WorkspaceRepository)(nil)

// WorkspaceRepository is the Clover v2 implementation of
// port.WorkspaceRepository. It provisions the directory-backed store
// used by a d7 workspace and writes the initial workspace metadata
// document.
type WorkspaceRepository struct{}

// NewWorkspaceRepository returns a Clover-backed workspace repository.
func NewWorkspaceRepository() *WorkspaceRepository {
	return &WorkspaceRepository{}
}

// CreateDatabase opens (and thereby creates) a Clover store at dbDir,
// creates the workspace collection, writes a single metadata document
// containing the supplied target, and closes the store cleanly. If
// any step fails after the database is opened, the store is still
// closed before the error is returned.
func (r *WorkspaceRepository) CreateDatabase(_ context.Context, dbDir string, meta domain.WorkspaceMetadata) (retErr error) {
	db, err := c.Open(dbDir)
	if err != nil {
		return fmt.Errorf("open clover db: %w", err)
	}
	defer func() {
		if closeErr := db.Close(); closeErr != nil && retErr == nil {
			retErr = fmt.Errorf("close clover db: %w", closeErr)
		}
	}()

	if err := db.CreateCollection(workspaceCollection); err != nil {
		return fmt.Errorf("create %q collection: %w", workspaceCollection, err)
	}

	doc := d.NewDocument()
	doc.Set(fieldTarget, string(meta.Target))

	if _, err := db.InsertOne(workspaceCollection, doc); err != nil {
		return fmt.Errorf("insert workspace metadata: %w", err)
	}

	return nil
}
