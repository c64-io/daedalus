package port

import (
	"context"

	"github.com/c64-io/daedalus/internal/core/domain"
)

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
