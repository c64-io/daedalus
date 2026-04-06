package domain

import (
	"errors"
	"fmt"
	"strings"
)

// Status represents the lifecycle state of any work item in the d7
// hierarchy (Idea, Epic, Feature, Story). The transition rules are
// shared across all entity types.
type Status string

const (
	StatusDraft      Status = "draft"
	StatusRefined    Status = "refined"
	StatusReady      Status = "ready"
	StatusInProgress Status = "in-progress"
	StatusReview     Status = "review"
	StatusDone       Status = "done"
	StatusArchived   Status = "archived"
	StatusBlocked    Status = "blocked"
)

// validTransitions defines which status transitions are allowed. The
// key is the current status; the value is the set of statuses it may
// transition to. This is the single source of truth for the lifecycle
// state machine.
var validTransitions = map[Status]map[Status]bool{
	StatusDraft:      {StatusRefined: true, StatusArchived: true, StatusBlocked: true},
	StatusRefined:    {StatusReady: true, StatusDraft: true, StatusArchived: true, StatusBlocked: true},
	StatusReady:      {StatusInProgress: true, StatusRefined: true, StatusArchived: true, StatusBlocked: true},
	StatusInProgress: {StatusReview: true, StatusReady: true, StatusArchived: true, StatusBlocked: true},
	StatusReview:     {StatusDone: true, StatusInProgress: true, StatusArchived: true, StatusBlocked: true},
	StatusDone:       {StatusArchived: true},
	StatusArchived:   {StatusDraft: true},
	StatusBlocked:    {StatusDraft: true, StatusRefined: true, StatusReady: true, StatusInProgress: true, StatusReview: true, StatusArchived: true},
}

// knownStatuses indexes the valid Status values for O(1) lookup.
var knownStatuses = map[Status]bool{
	StatusDraft: true, StatusRefined: true, StatusReady: true,
	StatusInProgress: true, StatusReview: true, StatusDone: true,
	StatusArchived: true, StatusBlocked: true,
}

// ErrInvalidTransition is returned when a status transition is not
// allowed by the lifecycle state machine.
var ErrInvalidTransition = errors.New("invalid status transition")

// ErrUnknownStatus is returned by ParseStatus for unrecognized values.
var ErrUnknownStatus = errors.New("unknown status")

// CanTransition reports whether moving from the current status to
// next is a valid lifecycle transition.
func (s Status) CanTransition(next Status) bool {
	targets, ok := validTransitions[s]
	if !ok {
		return false
	}
	return targets[next]
}

// Transition returns next if the transition from s is valid, or a
// wrapped ErrInvalidTransition otherwise.
func (s Status) Transition(next Status) (Status, error) {
	if !s.CanTransition(next) {
		return s, fmt.Errorf("%w: %s → %s", ErrInvalidTransition, s, next)
	}
	return next, nil
}

// ParseStatus converts a raw string into a valid Status. The input is
// case-insensitive and whitespace-trimmed.
func ParseStatus(raw string) (Status, error) {
	norm := Status(strings.ToLower(strings.TrimSpace(raw)))
	if !knownStatuses[norm] {
		return "", fmt.Errorf("%w: %q", ErrUnknownStatus, raw)
	}
	return norm, nil
}
