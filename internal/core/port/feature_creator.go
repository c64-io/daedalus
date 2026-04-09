package port

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
