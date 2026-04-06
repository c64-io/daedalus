package domain

import (
	"errors"
	"fmt"
	"strings"
)

// Priority represents the urgency/importance of a work item.
// Applicable to Epics, Features, Stories — not Ideas.
type Priority string

const (
	PriorityLow      Priority = "low"
	PriorityMedium   Priority = "medium"
	PriorityHigh     Priority = "high"
	PriorityCritical Priority = "critical"
)

// knownPriorities indexes the valid Priority values for O(1) lookup.
var knownPriorities = map[Priority]bool{
	PriorityLow: true, PriorityMedium: true,
	PriorityHigh: true, PriorityCritical: true,
}

// ErrUnknownPriority is returned by ParsePriority for unrecognized values.
var ErrUnknownPriority = errors.New("unknown priority")

// ParsePriority converts a raw string into a valid Priority.
func ParsePriority(raw string) (Priority, error) {
	norm := Priority(strings.ToLower(strings.TrimSpace(raw)))
	if !knownPriorities[norm] {
		return "", fmt.Errorf("%w: %q (supported: low, medium, high, critical)", ErrUnknownPriority, raw)
	}
	return norm, nil
}

// SupportedPriorities returns a human-readable list of valid priorities.
func SupportedPriorities() string {
	return "low, medium, high, critical"
}
