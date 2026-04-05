package port

import (
	"context"

	"github.com/c64-io/daedalus/internal/core/domain"
)

// CreateIdeaRequest is the input shape for the IdeaCreator.Create use
// case.
type CreateIdeaRequest struct {
	RootDir     string // workspace root; empty = cwd
	Title       string // required
	Description string // optional
}

// IdeaCreator is the driving port exposing the "create a new idea"
// use case to inbound adapters.
type IdeaCreator interface {
	CreateIdea(ctx context.Context, req CreateIdeaRequest) (*domain.Idea, error)
}
