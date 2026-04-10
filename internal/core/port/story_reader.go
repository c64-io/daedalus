package port

import (
	"context"

	"github.com/c64-io/daedalus/internal/core/domain"
)

// StoryReader is the driving port exposing the "read stories" use cases.
type StoryReader interface {
	GetStory(ctx context.Context, rootDir string, id string) (*domain.Story, error)
	ListStories(ctx context.Context, rootDir string, featureID string) ([]domain.Story, error)
}
