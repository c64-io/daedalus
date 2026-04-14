package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/c64-io/daedalus/internal/core/port/driven"
)

// aiState enumerates the nodes of the AI dialog state machine. The
// runner is a non-recursive loop over transitions; every reachable
// code path is a single case in the switch below.
type aiState int

const (
	stateInit aiState = iota
	stateCallLLM
	stateInspectResponse
	stateValidateProposal
	stateAskUser
	stateReviewProposal
	stateApplyChanges
	stateBackoff
	stateRetryMalformed

	// Terminal states.
	stateFailed
	stateAborted
	stateDone
)

// ErrAIDialogAborted is returned by runDialog when the founder aborts
// the dialog (via AnswerAbort or DecisionAbort). The caller treats
// this as a clean exit, not a failure.
var ErrAIDialogAborted = errors.New("ai: dialog aborted by user")

// aiDialog carries the mutable state of one interactive dialog from
// state to state. It is constructed by the caller (a command service),
// populated with the command-specific prompt, context, tools, and
// callbacks, then handed to runDialog.
type aiDialog struct {
	// Command context for the status header.
	command string
	target  string

	// Immutable request inputs.
	model         string
	system        string
	contextBlock  string
	tools         []driven.Tool
	maxTokensPerTurn int

	// Budgets. All initialized by the caller.
	maxInterviews   int           // hard cap on ask_question turns
	malformedBudget int           // retries when LLM fails to call a tool
	backoffBudget   int           // retries on rate-limit / network errors
	backoffBase     time.Duration // initial backoff; doubles on each use

	// Callbacks. `validate` inspects the raw tool_input before we show
	// it to the founder. `renderProposal` turns the payload into a
	// user-facing Proposal. `apply` commits the accepted payload.
	validate       func(payload map[string]any) error
	renderProposal func(payload map[string]any) driven.Proposal
	apply          func(ctx context.Context, payload map[string]any) (any, error)

	// Mutable state.
	messages       []driven.Message
	interviewTurns int
	llmTurns       int

	lastResponse *driven.ChatResponse
	lastPayload  map[string]any
	lastError    error

	backoffDelay time.Duration
	totalUsage   driven.Usage
	startedAt    time.Time

	appliedResult any
}

// runDialog drives the dialog to a terminal state and returns the
// applied result (from d.apply) on success, ErrAIDialogAborted if
// the founder aborted, or the underlying error otherwise.
//
// There is zero recursion here — every transition is a single step
// in the for-loop below. Adding a new failure mode or new happy-path
// branch is adding one case, not refactoring a call graph.
func runDialog(
	ctx context.Context,
	d *aiDialog,
	llm driven.AIAssistant,
	inter driven.Interaction,
	status driven.StatusRenderer,
	clock driven.Clock,
) (any, error) {
	d.startedAt = clock.Now()
	d.backoffDelay = d.backoffBase
	if d.maxTokensPerTurn == 0 {
		d.maxTokensPerTurn = 4096
	}

	// Anthropic's Messages API rejects a request with an empty messages
	// array ("messages: at least one message is required"), even when
	// a system prompt is present. The assistant turn always needs at
	// least one user turn to respond to. If the caller hasn't pre-seeded
	// a conversation, kick off with a short nudge that matches what the
	// system prompt already tells the model to do: start the interview.
	if len(d.messages) == 0 {
		d.messages = append(d.messages, driven.Message{
			Role: driven.RoleUser,
			Text: "Let's get started. Please begin.",
		})
	}

	status.Start(ctx, d.renderStatus(driven.PhaseIdle, ""))
	defer status.Stop()

	state := stateInit
	for {
		switch state {
		case stateDone:
			return d.appliedResult, nil
		case stateAborted:
			return nil, ErrAIDialogAborted
		case stateFailed:
			return nil, d.lastError
		}
		state = d.step(ctx, state, llm, inter, status, clock)
	}
}

// step dispatches one transition. Kept as a single method so test
// authors can see the whole transition table in one place.
func (d *aiDialog) step(
	ctx context.Context,
	state aiState,
	llm driven.AIAssistant,
	inter driven.Interaction,
	status driven.StatusRenderer,
	clock driven.Clock,
) aiState {
	switch state {
	case stateInit:
		return stateCallLLM
	case stateCallLLM:
		return d.callLLM(ctx, llm, status)
	case stateInspectResponse:
		return d.inspectResponse()
	case stateAskUser:
		return d.askUser(ctx, inter, status)
	case stateValidateProposal:
		return d.validateProposal()
	case stateReviewProposal:
		return d.reviewProposal(ctx, inter, status)
	case stateApplyChanges:
		return d.applyChanges(ctx, status)
	case stateBackoff:
		return d.backoff(ctx, clock, status)
	case stateRetryMalformed:
		return d.retryMalformed()
	}
	d.lastError = fmt.Errorf("ai: state machine reached unknown state %d", state)
	return stateFailed
}

// callLLM makes one chat call and classifies the result.
func (d *aiDialog) callLLM(ctx context.Context, llm driven.AIAssistant, status driven.StatusRenderer) aiState {
	d.llmTurns++
	status.Update(d.renderStatus(driven.PhaseThinking, ""))

	resp, err := llm.Chat(ctx, driven.ChatRequest{
		Model:     d.model,
		System:    d.system,
		Context:   d.contextBlock,
		Messages:  d.messages,
		Tools:     d.tools,
		MaxTokens: d.maxTokensPerTurn,
	})
	if err != nil {
		return d.classifyLLMError(err)
	}
	d.lastResponse = resp
	d.totalUsage = addUsage(d.totalUsage, resp.Usage)
	return stateInspectResponse
}

// classifyLLMError maps an AI sentinel error to the next state.
// Auth, context-too-large, and invalid-request are fail-fast — retrying
// them cannot possibly help and would only burn the backoff budget on
// an error the caller needs to see *now*. Rate-limit, overloaded, and
// network are retryable up to the backoff budget.
func (d *aiDialog) classifyLLMError(err error) aiState {
	switch {
	case errors.Is(err, driven.ErrAIAuthFailed),
		errors.Is(err, driven.ErrAIContextTooLarge),
		errors.Is(err, driven.ErrAIInvalidRequest):
		d.lastError = err
		return stateFailed
	case errors.Is(err, driven.ErrAIRateLimited),
		errors.Is(err, driven.ErrAIOverloaded),
		errors.Is(err, driven.ErrAINetworkError):
		if d.backoffBudget <= 0 {
			d.lastError = fmt.Errorf("ai: exhausted backoff budget: %w", err)
			return stateFailed
		}
		d.backoffBudget--

		// Honor Retry-After hint if present.
		var rl *driven.RateLimitedError
		if errors.As(err, &rl) && rl.RetryAfter > 0 {
			d.backoffDelay = rl.RetryAfter
		}
		return stateBackoff
	default:
		d.lastError = err
		return stateFailed
	}
}

// inspectResponse routes on which tool the LLM called.
func (d *aiDialog) inspectResponse() aiState {
	if d.lastResponse == nil || d.lastResponse.ToolUse == nil {
		return d.bumpMalformed()
	}
	switch d.lastResponse.ToolUse.Name {
	case driven.ToolAskQuestion:
		return stateAskUser
	case driven.ToolSubmitProposal:
		d.lastPayload = d.lastResponse.ToolUse.Input
		return stateValidateProposal
	default:
		return d.bumpMalformed()
	}
}

// askUser hands the LLM's question to the founder and records both
// sides of the exchange in the message history.
func (d *aiDialog) askUser(ctx context.Context, inter driven.Interaction, status driven.StatusRenderer) aiState {
	status.Update(d.renderStatus(driven.PhaseInterviewing, ""))

	tu := d.lastResponse.ToolUse
	qText, _ := tu.Input["question"].(string)
	qWhy, _ := tu.Input["why"].(string)

	ans, err := inter.Ask(ctx, driven.Question{
		Text:      qText,
		Why:       qWhy,
		TurnsUsed: d.interviewTurns + 1,
		TurnsMax:  d.maxInterviews,
	})
	if err != nil {
		d.lastError = err
		return stateFailed
	}

	// Record the assistant turn that asked.
	d.messages = append(d.messages, driven.Message{
		Role:    driven.RoleAssistant,
		ToolUse: tu,
	})

	switch ans.Kind {
	case driven.AnswerAbort:
		return stateAborted
	case driven.AnswerForcePropose:
		d.messages = append(d.messages, driven.Message{
			Role: driven.RoleUser,
			ToolResult: &driven.ToolResultContent{
				ToolUseID: tu.ID,
				Content:   "The founder wants you to propose now. Use what you know.",
			},
		})
	case driven.AnswerReply:
		d.messages = append(d.messages, driven.Message{
			Role: driven.RoleUser,
			ToolResult: &driven.ToolResultContent{
				ToolUseID: tu.ID,
				Content:   ans.Text,
			},
		})
	}

	d.interviewTurns++

	// Out of interview budget — force the next turn to propose.
	if d.interviewTurns >= d.maxInterviews {
		d.messages = append(d.messages, driven.Message{
			Role: driven.RoleUser,
			Text: "You have asked enough questions. Please submit_proposal now with what you know.",
		})
	}

	return stateCallLLM
}

// validateProposal runs the service-provided validator on the raw
// payload. Validator failure is treated as a malformed response —
// the LLM is asked to correct it.
func (d *aiDialog) validateProposal() aiState {
	if d.validate == nil {
		return stateReviewProposal
	}
	if err := d.validate(d.lastPayload); err != nil {
		d.lastError = err
		return d.bumpMalformed()
	}
	return stateReviewProposal
}

// reviewProposal hands a rendered Proposal to the founder and routes
// on the decision.
func (d *aiDialog) reviewProposal(ctx context.Context, inter driven.Interaction, status driven.StatusRenderer) aiState {
	status.Update(d.renderStatus(driven.PhaseReviewing, ""))

	var p driven.Proposal
	if d.renderProposal != nil {
		p = d.renderProposal(d.lastPayload)
	} else {
		p = driven.Proposal{Format: driven.ProposalSingle}
	}

	dec, err := inter.Review(ctx, p)
	if err != nil {
		d.lastError = err
		return stateFailed
	}

	switch dec.Kind {
	case driven.DecisionAccept:
		// Founder may have edited the proposal in $EDITOR. Re-parse
		// and re-validate before applying.
		if dec.Edited != "" {
			edited, perr := parseEditedPayload(dec.Edited)
			if perr != nil {
				d.lastError = fmt.Errorf("ai: edited proposal is not valid JSON: %w", perr)
				return stateFailed
			}
			if d.validate != nil {
				if verr := d.validate(edited); verr != nil {
					d.lastError = fmt.Errorf("ai: edited proposal failed validation: %w", verr)
					return stateFailed
				}
			}
			d.lastPayload = edited
		}
		return stateApplyChanges

	case driven.DecisionCritique:
		// Record the assistant's last tool_use turn, then feed the
		// critique back as the paired tool_result.
		d.messages = append(d.messages, driven.Message{
			Role:    driven.RoleAssistant,
			ToolUse: d.lastResponse.ToolUse,
		})
		d.messages = append(d.messages, driven.Message{
			Role: driven.RoleUser,
			ToolResult: &driven.ToolResultContent{
				ToolUseID: d.lastResponse.ToolUse.ID,
				Content:   "The founder isn't happy with this proposal yet. Their feedback: " + dec.Critique,
			},
		})
		return stateCallLLM

	case driven.DecisionAbort:
		return stateAborted
	}

	d.lastError = fmt.Errorf("ai: unexpected decision kind %d", dec.Kind)
	return stateFailed
}

// applyChanges commits the accepted payload via the service-provided
// apply callback.
func (d *aiDialog) applyChanges(ctx context.Context, status driven.StatusRenderer) aiState {
	status.Update(d.renderStatus(driven.PhaseApplying, ""))
	if d.apply == nil {
		return stateDone
	}
	result, err := d.apply(ctx, d.lastPayload)
	if err != nil {
		d.lastError = err
		return stateFailed
	}
	d.appliedResult = result
	return stateDone
}

// backoff sleeps the current delay and doubles for the next use.
// Clock.Sleep is behind the port so tests run without wall time.
func (d *aiDialog) backoff(ctx context.Context, clock driven.Clock, status driven.StatusRenderer) aiState {
	status.Update(d.renderStatus(driven.PhaseBackoff, fmt.Sprintf("retry in %s", d.backoffDelay)))
	clock.Sleep(d.backoffDelay)
	d.backoffDelay *= 2
	return stateCallLLM
}

// retryMalformed appends a reminder to the conversation and loops
// back to stateCallLLM.
func (d *aiDialog) retryMalformed() aiState {
	d.messages = append(d.messages, driven.Message{
		Role: driven.RoleUser,
		Text: "Your previous response didn't call ask_question or submit_proposal. " +
			"Please call exactly one of those tools on your next turn.",
	})
	return stateCallLLM
}

// bumpMalformed decrements the malformed budget and either routes to
// stateRetryMalformed or fails the dialog.
func (d *aiDialog) bumpMalformed() aiState {
	if d.malformedBudget <= 0 {
		if d.lastError == nil {
			d.lastError = errors.New("ai: model failed to produce a valid tool call after multiple attempts")
		} else {
			d.lastError = fmt.Errorf("ai: model failed to produce a valid tool call after multiple attempts: %w", d.lastError)
		}
		return stateFailed
	}
	d.malformedBudget--
	return stateRetryMalformed
}

func (d *aiDialog) renderStatus(phase driven.Phase, extra string) driven.Status {
	return driven.Status{
		Command:   d.command,
		Target:    d.target,
		Phase:     phase,
		Turn:      d.llmTurns,
		Usage:     d.totalUsage,
		Model:     d.model,
		StartedAt: d.startedAt,
		Extra:     extra,
	}
}

func parseEditedPayload(s string) (map[string]any, error) {
	var m map[string]any
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		return nil, err
	}
	return m, nil
}

func addUsage(a, b driven.Usage) driven.Usage {
	return driven.Usage{
		InputTokens:         a.InputTokens + b.InputTokens,
		OutputTokens:        a.OutputTokens + b.OutputTokens,
		CacheCreationTokens: a.CacheCreationTokens + b.CacheCreationTokens,
		CacheReadTokens:     a.CacheReadTokens + b.CacheReadTokens,
	}
}
