package driving

import (
	"context"

	"github.com/c64-io/daedalus/internal/core/domain"
)

// CreateEpicRequest is the input shape for the EpicCreator use case.
type CreateEpicRequest struct {
	RootDir     string          // workspace root; empty = cwd
	IdeaID      string          // parent idea ID (required)
	Title       string          // required
	Description string          // optional
	Priority    domain.Priority // optional; zero value = unset
	Size        domain.Size     // optional; zero value = unset
}

// EpicCreator is the driving port exposing the "create a new epic"
// use case to inbound adapters.
type EpicCreator interface {
	CreateEpic(ctx context.Context, req CreateEpicRequest) (*domain.Epic, error)
}

// EpicReader is the driving port exposing the "read epics" use cases.
type EpicReader interface {
	GetEpic(ctx context.Context, rootDir string, id string) (*domain.Epic, error)
	ListEpics(ctx context.Context, rootDir string, ideaID string) ([]domain.Epic, error)
}

// SetEpicRequest describes which fields to update on an Epic. Only
// non-nil pointer fields are applied; nil means "leave unchanged."
type SetEpicRequest struct {
	RootDir     string           // workspace root; empty = cwd
	ID          string           // epic to update (required)
	Status      *domain.Status   // new status (validated against state machine)
	Title       *string          // new title
	Description *string          // new description
	Priority    *domain.Priority // new priority
	Size        *domain.Size     // new size
}

// EpicSetter is the driving port for updating Epic fields.
type EpicSetter interface {
	SetEpic(ctx context.Context, req SetEpicRequest) (*domain.Epic, error)
}

// Epic is the combined driving surface for the epic subcommand group.
// Individual CLI subcommands still take the narrow port they actually
// need; the bundle exists only to keep root-command wiring flat.
type Epic interface {
	EpicCreator
	EpicReader
	EpicSetter
}
