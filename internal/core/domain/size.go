package domain

import (
	"errors"
	"fmt"
	"strings"
)

// Size is a Fibonacci story-point estimate for a work item.
// Applicable to Epics, Features, Stories — not Ideas.
type Size int

const (
	SizeUnset Size = 0
	Size1     Size = 1
	Size2     Size = 2
	Size3     Size = 3
	Size5     Size = 5
	Size8     Size = 8
	Size13    Size = 13
	Size21    Size = 21
)

// validSizes indexes the valid non-zero Size values for O(1) lookup.
var validSizes = map[Size]bool{
	Size1: true, Size2: true, Size3: true, Size5: true,
	Size8: true, Size13: true, Size21: true,
}

// ErrInvalidSize is returned by ParseSize for unrecognized values.
var ErrInvalidSize = errors.New("invalid size")

// ParseSize converts a raw string into a valid Size.
func ParseSize(raw string) (Size, error) {
	raw = strings.TrimSpace(raw)
	var n int
	if _, err := fmt.Sscanf(raw, "%d", &n); err != nil {
		return 0, fmt.Errorf("%w: %q (supported: %s)", ErrInvalidSize, raw, SupportedSizes())
	}
	s := Size(n)
	if !validSizes[s] {
		return 0, fmt.Errorf("%w: %d (supported: %s)", ErrInvalidSize, n, SupportedSizes())
	}
	return s, nil
}

// SupportedSizes returns a human-readable list of valid sizes.
func SupportedSizes() string {
	return "1, 2, 3, 5, 8, 13, 21"
}
