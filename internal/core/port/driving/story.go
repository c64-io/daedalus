package driving

import (
	"context"

	"github.com/c64-io/daedalus/internal/core/domain"
)

// CreateStoryRequest is the input shape for the StoryCreator use case.
type CreateStoryRequest struct {
	RootDir     string          // workspace root; empty = cwd
	FeatureID   string          // parent feature ID (required)
	Title       string          // required
	Description string          // optional
	Priority    domain.Priority // optional; zero value = unset
	Size        domain.Size     // optional; zero value = unset
}

// StoryCreator is the driving port exposing the "create a new story"
// use case to inbound adapters.
type StoryCreator interface {
	CreateStory(ctx context.Context, req CreateStoryRequest) (*domain.Story, error)
}

// StoryReader is the driving port exposing the "read stories" use cases.
type StoryReader interface {
	GetStory(ctx context.Context, rootDir string, id string) (*domain.Story, error)
	ListStories(ctx context.Context, rootDir string, featureID string) ([]domain.Story, error)
}

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
