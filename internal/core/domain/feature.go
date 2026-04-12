package domain

import "time"

// Feature is a coherent chunk of capability under an Epic. It is the
// third level of the d7 hierarchy: Idea → Epic → Feature → Story →
// Spec → Scenario. Features carry priority and size, like Epics.
type Feature struct {
	ID          string    // e.g. "FEAT-001", stable and immutable
	EpicID      string    // parent Epic ID (e.g. "EPIC-001"), required
	Title       string    // short, required, editable
	Description string    // longer prose, optional, editable
	Status      Status    // lifecycle state; starts as StatusDraft
	Priority    Priority  // low/medium/high/critical; zero value = unset
	Size        Size      // Fibonacci points; zero value = unset
	CreatedAt   time.Time // set once at creation, never mutated
}

// FeatureIDPrefix is the prefix used for Feature human-readable IDs.
const FeatureIDPrefix = "FEAT"

// FormatFeatureID builds a human-readable Feature ID from a sequence
// number: FormatFeatureID(1) → "FEAT-001", FormatFeatureID(42) → "FEAT-042".
func FormatFeatureID(seq int) string {
	return formatID(FeatureIDPrefix, seq)
}
