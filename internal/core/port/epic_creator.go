package port

import (
	"context"

	"github.com/c64-io/daedalus/internal/core/domain"
)

// CreateEpicRequest is the input shape for the EpicCreator use case.
type CreateEpicRequest struct {
	RootDir     string          // workspace root; empty = cwd
	IdeaID      string          // parent idea ID (required)
	Title       string          // required
	Description string          // optional
	Priority    domain.Priority // optional; zero value = unset
	Size        domain.Size     // optional; zero value = unset
}

// EpicCreator is the driving port exposing the "create a new epic"
// use case to inbound adapters.
type EpicCreator interface {
	CreateEpic(ctx context.Context, req CreateEpicRequest) (*domain.Epic, error)
}
