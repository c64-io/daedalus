package driving

import (
	"context"

	"github.com/c64-io/daedalus/internal/core/domain"
)

// CreateSpecRequest is the input shape for the SpecCreator use case.
type CreateSpecRequest struct {
	RootDir     string // workspace root; empty = cwd
	StoryID     string // parent story ID (required)
	Title       string // required
	Description string // optional
}

// SpecCreator is the driving port exposing the "create a new spec"
// use case to inbound adapters.
type SpecCreator interface {
	CreateSpec(ctx context.Context, req CreateSpecRequest) (*domain.Spec, error)
}

// SpecReader is the driving port exposing the "read specs" use cases.
type SpecReader interface {
	GetSpec(ctx context.Context, rootDir string, id string) (*domain.Spec, error)
	ListSpecs(ctx context.Context, rootDir string, storyID string) ([]domain.Spec, error)
}

// SetSpecRequest describes which fields to update on a Spec.
// Only non-nil pointer fields are applied; nil means "leave unchanged."
type SetSpecRequest struct {
	RootDir     string         // workspace root; empty = cwd
	ID          string         // spec to update (required)
	Status      *domain.Status // new status (validated against state machine)
	Title       *string        // new title
	Description *string        // new description
}

// SpecSetter is the driving port for updating Spec fields.
type SpecSetter interface {
	SetSpec(ctx context.Context, req SetSpecRequest) (*domain.Spec, error)
}

// Spec is the combined driving surface for the spec subcommand group.
// Individual CLI subcommands still take the narrow port they actually
// need; the bundle exists only to keep root-command wiring flat.
type Spec interface {
	SpecCreator
	SpecReader
	SpecSetter
}
