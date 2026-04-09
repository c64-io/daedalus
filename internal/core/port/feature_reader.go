package port

import (
	"context"

	"github.com/c64-io/daedalus/internal/core/domain"
)

// FeatureReader is the driving port exposing the "read features" use cases.
type FeatureReader interface {
	GetFeature(ctx context.Context, rootDir string, id string) (*domain.Feature, error)
	ListFeatures(ctx context.Context, rootDir string, epicID string) ([]domain.Feature, error)
}
