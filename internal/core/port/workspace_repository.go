package port

import (
	"context"
	"errors"

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

	// ReadMetadata opens the database at the given directory, reads
	// the single workspace metadata record, and returns it. It is the
	// read side of CreateDatabase. Implementations must return a
	// wrapped ErrMetadataNotFound when the database exists but
	// contains no workspace record (which indicates either corruption
	// or a partially-initialized workspace).
	ReadMetadata(ctx context.Context, dbDir string) (domain.WorkspaceMetadata, error)

	// SaveProjectDescription writes the project description content
	// into the workspace database, replacing any existing content.
	SaveProjectDescription(ctx context.Context, dbDir string, content string) error

	// ReadProjectDescription reads the project description from the
	// workspace database. Returns ErrProjectDescriptionNotFound when
	// no description has been stored yet.
	ReadProjectDescription(ctx context.Context, dbDir string) (string, error)
}

// ErrMetadataNotFound is returned by WorkspaceRepository.ReadMetadata
// when the database exists but contains no workspace metadata
// record. It is a driven-port sentinel so both the service layer and
// tests can match on it.
var ErrMetadataNotFound = errors.New("workspace metadata not found")

// ErrProjectDescriptionNotFound is returned when no project
// description has been stored in the workspace database.
var ErrProjectDescriptionNotFound = errors.New("project description not found")
