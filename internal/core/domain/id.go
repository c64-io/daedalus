package domain

import "fmt"

// formatID builds a human-readable entity ID from a prefix and a
// monotonic sequence number: formatID("IDEA", 1) → "IDEA-001".
// Three-digit zero-padded format supports up to 999 items per type
// before widening; since d7 is a solo-founder tool, this is generous.
func formatID(prefix string, seq int) string {
	return fmt.Sprintf("%s-%03d", prefix, seq)
}
