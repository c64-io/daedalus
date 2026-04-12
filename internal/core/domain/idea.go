package domain

import "time"

// Idea is the top of the d7 hierarchy — the vague spark from which
// Epics, Features, Stories, Specs, and Scenarios descend. Every Idea
// starts in StatusDraft and is assigned a stable, human-readable ID
// (IDEA-001, IDEA-002, ...) at creation time.
type Idea struct {
	ID          string    // e.g. "IDEA-001", stable and immutable
	Title       string    // short, required, editable
	Description string    // longer prose, optional, editable
	Status      Status    // lifecycle state; starts as StatusDraft
	CreatedAt   time.Time // set once at creation, never mutated
}

// IdeaIDPrefix is the prefix used for Idea human-readable IDs.
const IdeaIDPrefix = "IDEA"

// FormatIdeaID builds a human-readable Idea ID from a sequence
// number: FormatIdeaID(1) → "IDEA-001", FormatIdeaID(42) → "IDEA-042".
func FormatIdeaID(seq int) string {
	return formatID(IdeaIDPrefix, seq)
}
