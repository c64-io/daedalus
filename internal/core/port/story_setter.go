package port

import (
	"context"

	"github.com/c64-io/daedalus/internal/core/domain"
)

// SetStoryRequest describes which fields to update on a Story.
// Only non-nil pointer fields are applied; nil means "leave unchanged."
type SetStoryRequest struct {
	RootDir     string           // workspace root; empty = cwd
	ID          string           // story to update (required)
	Status      *domain.Status   // new status (validated against state machine)
	Title       *string          // new title
	Description *string          // new description
	Priority    *domain.Priority // new priority
	Size        *domain.Size     // new size
}

// StorySetter is the driving port for updating Story fields.
type StorySetter interface {
	SetStory(ctx context.Context, req SetStoryRequest) (*domain.Story, error)
}
