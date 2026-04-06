package port

import (
	"context"

	"github.com/c64-io/daedalus/internal/core/domain"
)

// IdeaReader is the driving port exposing the "read ideas" use cases
// to inbound adapters. Get and List cohere under one port because
// both are read-only operations on the same entity at different
// granularity.
type IdeaReader interface {
	GetIdea(ctx context.Context, rootDir string, id string) (*domain.Idea, error)
	ListIdeas(ctx context.Context, rootDir string) ([]domain.Idea, error)
}
