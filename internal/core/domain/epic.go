package domain

import "time"

// Epic is a major capability under an Idea. It is the second level of
// the d7 hierarchy: Idea → Epic → Feature → Story → Spec → Scenario.
// Epics carry priority and size, which Ideas do not.
type Epic struct {
	ID          string    // e.g. "EPIC-001", stable and immutable
	IdeaID      string    // parent Idea ID (e.g. "IDEA-001"), required
	Title       string    // short, required, editable
	Description string    // longer prose, optional, editable
	Status      Status    // lifecycle state; starts as StatusDraft
	Priority    Priority  // low/medium/high/critical; zero value = unset
	Size        Size      // Fibonacci points; zero value = unset
	CreatedAt   time.Time // set once at creation, never mutated
}

// EpicIDPrefix is the prefix used for Epic human-readable IDs.
const EpicIDPrefix = "EPIC"

// FormatEpicID builds a human-readable Epic ID from a sequence
// number: FormatEpicID(1) → "EPIC-001", FormatEpicID(42) → "EPIC-042".
func FormatEpicID(seq int) string {
	return formatID(EpicIDPrefix, seq)
}
