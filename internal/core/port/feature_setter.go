package port

import (
	"context"

	"github.com/c64-io/daedalus/internal/core/domain"
)

// SetFeatureRequest describes which fields to update on a Feature.
// Only non-nil pointer fields are applied; nil means "leave unchanged."
type SetFeatureRequest struct {
	RootDir     string           // workspace root; empty = cwd
	ID          string           // feature to update (required)
	Status      *domain.Status   // new status (validated against state machine)
	Title       *string          // new title
	Description *string          // new description
	Priority    *domain.Priority // new priority
	Size        *domain.Size     // new size
}

// FeatureSetter is the driving port for updating Feature fields.
type FeatureSetter interface {
	SetFeature(ctx context.Context, req SetFeatureRequest) (*domain.Feature, error)
}
