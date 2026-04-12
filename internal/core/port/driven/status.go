package driven

import (
	"context"
	"time"
)

// StatusRenderer is a driven port for a live status header that sits
// above d7's normal stdout during AI-assisted commands. Two adapter
// implementations are provided: a TTY-aware one that pins a header
// via ANSI scroll regions, and a noop one used when stdout is not a
// terminal (CI, pipes).
//
// Lifecycle: the service calls Start once at dialog begin, Update on
// every state transition, and Stop in a deferred call. Implementations
// must be safe to Update while Stopped (no-op) and safe to Stop more
// than once.
type StatusRenderer interface {
	Start(ctx context.Context, s Status)
	Update(s Status)
	Stop()
}

// Status is the snapshot the renderer draws. All fields are optional;
// the renderer fills in sensible blanks when any are empty.
type Status struct {
	Command   string    // "expand story"
	Target    string    // "STORY-001"
	Phase     Phase     // what the dialog is doing right now
	Turn      int       // nth LLM turn in this dialog
	Usage     Usage     // running total across the dialog
	Model     string    // model id in use
	StartedAt time.Time // for the elapsed-time display
	Extra     string    // free-form detail; e.g. "retry 2/3 in 4s"
}

// Phase is the coarse-grained state of the dialog, picked to render
// cleanly in a one-line header. It is a deliberate projection of the
// state machine's internal states — not every internal state maps to
// a distinct Phase.
type Phase int

const (
	PhaseIdle         Phase = iota // before Start or after Stop
	PhaseThinking                  // LLM is computing a turn
	PhaseInterviewing              // awaiting founder's answer
	PhaseReviewing                 // awaiting founder's review of a proposal
	PhaseApplying                  // writing changes to the DB
	PhaseBackoff                   // sleeping before a retry
	PhaseRetry                     // re-asking after malformed output
)

// String returns a short human-readable label suitable for the header.
func (p Phase) String() string {
	switch p {
	case PhaseThinking:
		return "thinking"
	case PhaseInterviewing:
		return "asking"
	case PhaseReviewing:
		return "reviewing"
	case PhaseApplying:
		return "saving"
	case PhaseBackoff:
		return "backing off"
	case PhaseRetry:
		return "retrying"
	default:
		return "idle"
	}
}
