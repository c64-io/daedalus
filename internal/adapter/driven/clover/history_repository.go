package clover

import (
	"context"
	"fmt"
	"time"

	c "github.com/ostafen/clover/v2"
	d "github.com/ostafen/clover/v2/document"

	"github.com/c64-io/daedalus/internal/core/domain"
	"github.com/c64-io/daedalus/internal/core/port"
)

const (
	historyCollection = "history"

	fieldEntityID  = "entity_id"
	fieldField     = "field"
	fieldOldValue  = "old_value"
	fieldNewValue  = "new_value"
	fieldTimestamp  = "timestamp"
)

// Compile-time assertion.
var _ port.HistoryRepository = (*HistoryRepository)(nil)

// HistoryRepository is the Clover v2 implementation of
// port.HistoryRepository. History entries are append-only.
type HistoryRepository struct{}

// NewHistoryRepository returns a Clover-backed history repository.
func NewHistoryRepository() *HistoryRepository {
	return &HistoryRepository{}
}

// AppendHistory writes one or more history entries into the shared
// history collection.
func (r *HistoryRepository) AppendHistory(_ context.Context, dbDir string, entries []domain.HistoryEntry) (retErr error) {
	if len(entries) == 0 {
		return nil
	}

	db, err := c.Open(dbDir)
	if err != nil {
		return fmt.Errorf("open clover db: %w", err)
	}
	defer closeDB(db, &retErr)

	if err := ensureCollection(db, historyCollection); err != nil {
		return err
	}

	for _, entry := range entries {
		doc := d.NewDocument()
		doc.Set(fieldEntityID, entry.EntityID)
		doc.Set(fieldField, entry.Field)
		doc.Set(fieldOldValue, entry.OldValue)
		doc.Set(fieldNewValue, entry.NewValue)
		doc.Set(fieldTimestamp, entry.Timestamp.Format(time.RFC3339Nano))

		if _, err := db.InsertOne(historyCollection, doc); err != nil {
			return fmt.Errorf("insert history entry: %w", err)
		}
	}

	return nil
}
