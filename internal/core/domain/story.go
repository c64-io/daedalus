package domain

import "time"

// Story is a user-visible slice of a Feature, following the INVEST
// criteria. It is the fourth level of the d7 hierarchy:
// Idea → Epic → Feature → Story → Spec → Scenario.
// Stories carry priority and size, like Epics and Features.
type Story struct {
	ID          string    // e.g. "STORY-001", stable and immutable
	FeatureID   string    // parent Feature ID (e.g. "FEAT-001"), required
	Title       string    // short, required, editable
	Description string    // longer prose, optional, editable
	Status      Status    // lifecycle state; starts as StatusDraft
	Priority    Priority  // low/medium/high/critical; zero value = unset
	Size        Size      // Fibonacci points; zero value = unset
	CreatedAt   time.Time // set once at creation, never mutated
}

// StoryIDPrefix is the prefix used for Story human-readable IDs.
const StoryIDPrefix = "STORY"

// FormatStoryID builds a human-readable Story ID from a sequence
// number: FormatStoryID(1) → "STORY-001", FormatStoryID(42) → "STORY-042".
func FormatStoryID(seq int) string {
	return formatID(StoryIDPrefix, seq)
}
