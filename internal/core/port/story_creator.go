package port

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
