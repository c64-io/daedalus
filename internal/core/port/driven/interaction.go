package driven

import "context"

// Interaction is a driven port for the founder-facing side of an AI
// dialog. It has two responsibilities:
//
//   - Ask: relay a question from the model to the founder and collect
//     their answer. Used during the interview phase.
//   - Review: present a finished proposal to the founder and collect
//     their decision (accept, critique, abort).
//
// The CLI adapter has two implementations: one driven by $EDITOR for
// single-item flows (expand, refine) and one stdin picker for multi-
// item flows (suggest). Both satisfy this port.
type Interaction interface {
	Ask(ctx context.Context, q Question) (Answer, error)
	Review(ctx context.Context, p Proposal) (Decision, error)
}

// Question is one interview prompt from the model to the founder.
// TurnsUsed/TurnsMax are passed through so the CLI can render a
// "question 3 of 10" hint next to the prompt.
type Question struct {
	Text      string
	Why       string
	TurnsUsed int
	TurnsMax  int
}

// Answer is the founder's reply to a Question. `/done` triggers
// AnswerForcePropose — the model is told to propose now with what it
// has. `/quit` triggers AnswerAbort.
type Answer struct {
	Kind AnswerKind
	Text string
}

// AnswerKind enumerates how an interview turn was resolved.
type AnswerKind int

const (
	// AnswerReply is a normal textual answer.
	AnswerReply AnswerKind = iota
	// AnswerForcePropose tells the model to stop interviewing and
	// submit a proposal now with whatever it has.
	AnswerForcePropose
	// AnswerAbort ends the dialog with no DB writes.
	AnswerAbort
)

// Proposal is what a Review call displays to the founder.
//
// For a single-item command (expand, refine) the model produces one
// proposal; Single is populated and Multiple is nil. For a multi-
// item command (suggest) Multiple holds the whole set.
type Proposal struct {
	Format   ProposalFormat
	Single   *ProposalItem
	Multiple []ProposalItem
	// RawJSON is the canonical JSON form of the proposal, used by the
	// editor-based interaction to load the buffer.
	RawJSON string
}

// ProposalFormat distinguishes single-item (editor) from multi-item
// (picker) presentations.
type ProposalFormat int

const (
	ProposalSingle ProposalFormat = iota
	ProposalMultiple
)

// ProposalItem is one unit the founder dispositions.
type ProposalItem struct {
	Title string
	Body  string // rendered human-readable view
	JSON  string // per-item JSON for editor flow
}

// Decision is the founder's resolution of a Review call.
type Decision struct {
	Kind DecisionKind

	// Critique is the founder's free-text feedback for the whole
	// batch (Kind == DecisionCritique).
	Critique string

	// PerItem holds y/n/r dispositions for Kind == DecisionAccept or
	// DecisionRetry on a multi-item proposal. Index-aligned with
	// Proposal.Multiple.
	PerItem []ItemDecision

	// Edited carries the founder's in-editor edits when Kind ==
	// DecisionAccept for a single-item proposal and they edited the
	// JSON buffer rather than accepting verbatim; it also carries the
	// edited list payload for Kind == DecisionEditList.
	Edited string
}

// DecisionKind enumerates the founder's verdict on a proposal.
type DecisionKind int

const (
	// DecisionAccept commits the proposal (possibly after edits). For a
	// multi-item proposal, PerItem indicates which items were accepted.
	DecisionAccept DecisionKind = iota
	// DecisionCritique sends the whole batch back to the model with
	// free-text feedback for a new attempt.
	DecisionCritique
	// DecisionRetry sends a subset of items back to the model with
	// per-item feedback; locked-in items remain.
	DecisionRetry
	// DecisionAbort discards the dialog with no DB writes.
	DecisionAbort
	// DecisionEditList is the founder's edited JSON for a multi-item
	// proposal; the service re-validates before applying. Every item
	// in the edited list is implicitly accepted.
	DecisionEditList
)

// ItemDecision is the per-item disposition inside a Decision. y, n,
// or r with feedback on r.
type ItemDecision struct {
	Kind     ItemDecisionKind
	Feedback string
}

// ItemDecisionKind is y / n / r.
type ItemDecisionKind int

const (
	ItemYes ItemDecisionKind = iota
	ItemNo
	ItemRetry
)
