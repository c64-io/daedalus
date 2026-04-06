package domain

import "time"

// HistoryEntry records a single mutation to a work item. History is
// append-only: the service layer is never allowed to rewrite or delete
// entries. This gives a durable audit trail answerable months later.
type HistoryEntry struct {
	EntityID  string    // e.g. "EPIC-003", "STORY-047"
	Field     string    // which field changed (e.g. "status", "title", "description")
	OldValue  string    // previous value (stringified)
	NewValue  string    // new value (stringified)
	Timestamp time.Time // when the change happened
}
