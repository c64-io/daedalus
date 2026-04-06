package port

import (
	"context"
	"errors"

	"github.com/c64-io/daedalus/internal/core/domain"
)

// EpicRepository is the driven port for persistent storage of Epic
// entities.
type EpicRepository interface {
	// NextEpicSeq atomically increments and returns the next monotonic
	// sequence number for Epic IDs.
	NextEpicSeq(ctx context.Context, dbDir string) (int, error)

	// SaveEpic persists a fully constructed Epic.
	SaveEpic(ctx context.Context, dbDir string, epic domain.Epic) error

	// GetEpic returns the Epic with the given human-readable ID, or
	// a wrapped ErrEpicNotFound if it does not exist.
	GetEpic(ctx context.Context, dbDir string, id string) (*domain.Epic, error)

	// ListEpics returns all Epics in creation order. If ideaID is
	// non-empty, only Epics belonging to that Idea are returned.
	ListEpics(ctx context.Context, dbDir string, ideaID string) ([]domain.Epic, error)

	// UpdateEpic replaces the stored Epic identified by epic.ID with
	// the provided values. Returns ErrEpicNotFound if it does not exist.
	UpdateEpic(ctx context.Context, dbDir string, epic domain.Epic) error
}

// ErrEpicNotFound is returned by EpicRepository.GetEpic when the
// requested Epic does not exist.
var ErrEpicNotFound = errors.New("epic not found")
