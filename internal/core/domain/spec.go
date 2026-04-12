package domain

import "time"

// Spec is prose requirements + context attached to a Story. It is the
// fifth level of the d7 hierarchy:
// Idea → Epic → Feature → Story → Spec → Scenario.
//
// A Spec captures the rules, constraints, edge cases, and narrative
// context that describe *what* a Story means and *why*. One Story can
// have many Specs, each covering a different aspect (happy-path
// rules, error handling, security, performance, etc.).
//
// Specs do not carry priority or size: they are documentation, not
// work items. Their executable counterparts are Scenarios, which
// hang off each Spec as Given/When/Then steps.
type Spec struct {
	ID          string    // e.g. "SPEC-001", stable and immutable
	StoryID     string    // parent Story ID (e.g. "STORY-001"), required
	Title       string    // short label, required, editable
	Description string    // prose body (markdown), optional, editable
	Status      Status    // lifecycle state; starts as StatusDraft
	CreatedAt   time.Time // set once at creation, never mutated
}

// SpecIDPrefix is the prefix used for Spec human-readable IDs.
const SpecIDPrefix = "SPEC"

// FormatSpecID builds a human-readable Spec ID from a sequence
// number: FormatSpecID(1) → "SPEC-001", FormatSpecID(42) → "SPEC-042".
func FormatSpecID(seq int) string {
	return formatID(SpecIDPrefix, seq)
}
