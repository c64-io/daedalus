package port

import (
	"context"

	"github.com/c64-io/daedalus/internal/core/domain"
)

// EpicReader is the driving port exposing the "read epics" use cases.
type EpicReader interface {
	GetEpic(ctx context.Context, rootDir string, id string) (*domain.Epic, error)
	ListEpics(ctx context.Context, rootDir string, ideaID string) ([]domain.Epic, error)
}
