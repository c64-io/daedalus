package driven

import (
	"context"
	"errors"

	"github.com/c64-io/daedalus/internal/core/domain"
)

// StoryRepository is the driven port for persistent storage of Story
// entities.
type StoryRepository interface {
	// NextStorySeq atomically increments and returns the next monotonic
	// sequence number for Story IDs.
	NextStorySeq(ctx context.Context, dbDir string) (int, error)

	// SaveStory persists a fully constructed Story.
	SaveStory(ctx context.Context, dbDir string, story domain.Story) error

	// GetStory returns the Story with the given human-readable ID, or
	// a wrapped ErrStoryNotFound if it does not exist.
	GetStory(ctx context.Context, dbDir string, id string) (*domain.Story, error)

	// ListStories returns all Stories in creation order. If featureID is
	// non-empty, only Stories belonging to that Feature are returned.
	ListStories(ctx context.Context, dbDir string, featureID string) ([]domain.Story, error)

	// UpdateStory replaces the stored Story identified by story.ID with
	// the provided values. Returns ErrStoryNotFound if it does not exist.
	UpdateStory(ctx context.Context, dbDir string, story domain.Story) error
}

// ErrStoryNotFound is returned by StoryRepository.GetStory when the
// requested Story does not exist.
var ErrStoryNotFound = errors.New("story not found")
