package port

import (
	"context"

	"github.com/c64-io/daedalus/internal/core/domain"
)

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
