package driven

import (
	"context"
	"errors"

	"github.com/c64-io/daedalus/internal/core/domain"
)

// SpecRepository is the driven port for persistent storage of Spec
// entities.
type SpecRepository interface {
	// NextSpecSeq atomically increments and returns the next monotonic
	// sequence number for Spec IDs.
	NextSpecSeq(ctx context.Context, dbDir string) (int, error)

	// SaveSpec persists a fully constructed Spec.
	SaveSpec(ctx context.Context, dbDir string, spec domain.Spec) error

	// GetSpec returns the Spec with the given human-readable ID, or
	// a wrapped ErrSpecNotFound if it does not exist.
	GetSpec(ctx context.Context, dbDir string, id string) (*domain.Spec, error)

	// ListSpecs returns all Specs in creation order. If storyID is
	// non-empty, only Specs belonging to that Story are returned.
	ListSpecs(ctx context.Context, dbDir string, storyID string) ([]domain.Spec, error)

	// UpdateSpec replaces the stored Spec identified by spec.ID with
	// the provided values. Returns ErrSpecNotFound if it does not exist.
	UpdateSpec(ctx context.Context, dbDir string, spec domain.Spec) error
}

// ErrSpecNotFound is returned by SpecRepository.GetSpec when the
// requested Spec does not exist.
var ErrSpecNotFound = errors.New("spec not found")
