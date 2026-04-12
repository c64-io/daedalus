package driven

import (
	"context"
	"errors"

	"github.com/c64-io/daedalus/internal/core/domain"
)

// IdeaRepository is the driven port for persistent storage of Idea
// entities. Each method takes dbDir so the repository stays stateless
// — callers resolve the workspace's database directory before
// invoking.
type IdeaRepository interface {
	// NextIdeaSeq atomically increments and returns the next monotonic
	// sequence number for Idea IDs (1, 2, 3, ...).
	NextIdeaSeq(ctx context.Context, dbDir string) (int, error)

	// SaveIdea persists a fully constructed Idea.
	SaveIdea(ctx context.Context, dbDir string, idea domain.Idea) error

	// GetIdea returns the Idea with the given human-readable ID
	// (e.g. "IDEA-001"), or a wrapped ErrIdeaNotFound if it does
	// not exist.
	GetIdea(ctx context.Context, dbDir string, id string) (*domain.Idea, error)

	// ListIdeas returns all Ideas in creation order.
	ListIdeas(ctx context.Context, dbDir string) ([]domain.Idea, error)

	// UpdateIdea replaces the stored Idea identified by idea.ID with
	// the provided values. Returns ErrIdeaNotFound if it does not exist.
	UpdateIdea(ctx context.Context, dbDir string, idea domain.Idea) error
}

// ErrIdeaNotFound is returned by IdeaRepository.GetIdea when the
// requested Idea does not exist.
var ErrIdeaNotFound = errors.New("idea not found")
