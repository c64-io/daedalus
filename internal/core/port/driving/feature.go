package driving

import (
	"context"

	"github.com/c64-io/daedalus/internal/core/domain"
)

// CreateFeatureRequest is the input shape for the FeatureCreator use case.
type CreateFeatureRequest struct {
	RootDir     string          // workspace root; empty = cwd
	EpicID      string          // parent epic ID (required)
	Title       string          // required
	Description string          // optional
	Priority    domain.Priority // optional; zero value = unset
	Size        domain.Size     // optional; zero value = unset
}

// FeatureCreator is the driving port exposing the "create a new feature"
// use case to inbound adapters.
type FeatureCreator interface {
	CreateFeature(ctx context.Context, req CreateFeatureRequest) (*domain.Feature, error)
}

// FeatureReader is the driving port exposing the "read features" use cases.
type FeatureReader interface {
	GetFeature(ctx context.Context, rootDir string, id string) (*domain.Feature, error)
	ListFeatures(ctx context.Context, rootDir string, epicID string) ([]domain.Feature, error)
}

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
