package driven

import (
	"context"
	"errors"

	"github.com/c64-io/daedalus/internal/core/domain"
)

// FeatureRepository is the driven port for persistent storage of Feature
// entities.
type FeatureRepository interface {
	// NextFeatureSeq atomically increments and returns the next monotonic
	// sequence number for Feature IDs.
	NextFeatureSeq(ctx context.Context, dbDir string) (int, error)

	// SaveFeature persists a fully constructed Feature.
	SaveFeature(ctx context.Context, dbDir string, feature domain.Feature) error

	// GetFeature returns the Feature with the given human-readable ID, or
	// a wrapped ErrFeatureNotFound if it does not exist.
	GetFeature(ctx context.Context, dbDir string, id string) (*domain.Feature, error)

	// ListFeatures returns all Features in creation order. If epicID is
	// non-empty, only Features belonging to that Epic are returned.
	ListFeatures(ctx context.Context, dbDir string, epicID string) ([]domain.Feature, error)

	// UpdateFeature replaces the stored Feature identified by feature.ID
	// with the provided values. Returns ErrFeatureNotFound if it does not exist.
	UpdateFeature(ctx context.Context, dbDir string, feature domain.Feature) error
}

// ErrFeatureNotFound is returned by FeatureRepository.GetFeature when
// the requested Feature does not exist.
var ErrFeatureNotFound = errors.New("feature not found")
