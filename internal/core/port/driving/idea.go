package driving

import (
	"context"

	"github.com/c64-io/daedalus/internal/core/domain"
)

// CreateIdeaRequest is the input shape for the IdeaCreator.Create use
// case.
type CreateIdeaRequest struct {
	RootDir     string // workspace root; empty = cwd
	Title       string // required
	Description string // optional
}

// IdeaCreator is the driving port exposing the "create a new idea"
// use case to inbound adapters.
type IdeaCreator interface {
	CreateIdea(ctx context.Context, req CreateIdeaRequest) (*domain.Idea, error)
}

// IdeaReader is the driving port exposing the "read ideas" use cases
// to inbound adapters. Get and List cohere under one port because
// both are read-only operations on the same entity at different
// granularity.
type IdeaReader interface {
	GetIdea(ctx context.Context, rootDir string, id string) (*domain.Idea, error)
	ListIdeas(ctx context.Context, rootDir string) ([]domain.Idea, error)
}

// SetIdeaRequest describes which fields to update on an Idea. Only
// non-nil pointer fields are applied; nil means "leave unchanged."
type SetIdeaRequest struct {
	RootDir     string         // workspace root; empty = cwd
	ID          string         // idea to update (required)
	Status      *domain.Status // new status (validated against state machine)
	Title       *string        // new title
	Description *string        // new description
}

// IdeaSetter is the driving port for updating Idea fields.
type IdeaSetter interface {
	SetIdea(ctx context.Context, req SetIdeaRequest) (*domain.Idea, error)
}

// Idea is the combined driving surface for the idea subcommand group.
// Individual CLI subcommands still take the narrow port they actually
// need (interface segregation); the bundle exists only to keep
// root-command wiring flat.
type Idea interface {
	IdeaCreator
	IdeaReader
	IdeaSetter
}
