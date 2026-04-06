package port

import (
	"context"

	"github.com/c64-io/daedalus/internal/core/domain"
)

// HistoryRepository is the driven port for persistent storage of
// HistoryEntry records. History is append-only — the service layer
// is never allowed to rewrite or delete entries.
type HistoryRepository interface {
	// AppendHistory writes one or more history entries atomically.
	AppendHistory(ctx context.Context, dbDir string, entries []domain.HistoryEntry) error
}
