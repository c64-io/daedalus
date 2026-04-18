package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/c64-io/daedalus/internal/core/port/driven"
)

// ---------------------------------------------------------------------
// Fakes
// ---------------------------------------------------------------------

// scriptedTurn is one entry in fakeLLM's script. Exactly one of err /
// toolUse / rawText should be set. Rate-limited errors honor retryAfter.
type scriptedTurn struct {
	err      error
	toolUse  *driven.ToolUseContent
	rawText  string
	stopReason string
	usage    driven.Usage
	retryAfter time.Duration // only meaningful when err is ErrAIRateLimited
}

// fakeLLM returns scripted responses in order and records the requests
// it saw, so tests can assert on cumulative message state.
type fakeLLM struct {
	script   []scriptedTurn
	idx      int
	requests []driven.ChatRequest
}

func (f *fakeLLM) Chat(_ context.Context, req driven.ChatRequest) (*driven.ChatResponse, error) {
	f.requests = append(f.requests, req)
	if f.idx >= len(f.script) {
		return nil, fmt.Errorf("fakeLLM: script exhausted at turn %d", f.idx)
	}
	t := f.script[f.idx]
	f.idx++
	if t.err != nil {
		if errors.Is(t.err, driven.ErrAIRateLimited) && t.retryAfter > 0 {
			return nil, &driven.RateLimitedError{RetryAfter: t.retryAfter}
		}
		return nil, t.err
	}
	return &driven.ChatResponse{
		ToolUse:    t.toolUse,
		RawText:    t.rawText,
		StopReason: t.stopReason,
		Usage:      t.usage,
	}, nil
}

// fakeInter plays scripted answers/decisions. It records the questions
// and proposals it saw so tests can assert on what the loop handed in.
type fakeInter struct {
	answers   []driven.Answer
	askErr    error
	decisions []driven.Decision
	reviewErr error

	askIdx     int
	reviewIdx  int
	seenQs     []driven.Question
	seenProps  []driven.Proposal
}

func (f *fakeInter) Ask(_ context.Context, q driven.Question) (driven.Answer, error) {
	f.seenQs = append(f.seenQs, q)
	if f.askErr != nil {
		return driven.Answer{}, f.askErr
	}
	if f.askIdx >= len(f.answers) {
		return driven.Answer{}, fmt.Errorf("fakeInter: answer script exhausted at %d", f.askIdx)
	}
	a := f.answers[f.askIdx]
	f.askIdx++
	return a, nil
}

func (f *fakeInter) Review(_ context.Context, p driven.Proposal) (driven.Decision, error) {
	f.seenProps = append(f.seenProps, p)
	if f.reviewErr != nil {
		return driven.Decision{}, f.reviewErr
	}
	if f.reviewIdx >= len(f.decisions) {
		return driven.Decision{}, fmt.Errorf("fakeInter: decision script exhausted at %d", f.reviewIdx)
	}
	d := f.decisions[f.reviewIdx]
	f.reviewIdx++
	return d, nil
}

// nullStatus is a no-op StatusRenderer used in tests that don't care
// about the header.
type nullStatus struct{}

func (nullStatus) Start(_ context.Context, _ driven.Status) {}
func (nullStatus) Update(_ driven.Status)                   {}
func (nullStatus) Stop()                                    {}

// fakeClock records sleep durations so tests can assert on backoff
// behavior without actually sleeping.
type fakeClock struct {
	now    time.Time
	sleeps []time.Duration
}

func (c *fakeClock) Now() time.Time { return c.now }

func (c *fakeClock) Sleep(d time.Duration) {
	c.sleeps = append(c.sleeps, d)
	c.now = c.now.Add(d)
}

// ---------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------

func newDialog() *aiDialog {
	return &aiDialog{
		command:          "test",
		target:           "X-1",
		model:            "test-model",
		system:           "you are a test",
		contextBlock:     "test context",
		tools:            nil,
		maxTokensPerTurn: 1024,
		maxInterviews:    10,
		malformedBudget:  2,
		backoffBudget:    2,
		backoffBase:      10 * time.Millisecond,
	}
}

func askQ(id, text string) *driven.ToolUseContent {
	return &driven.ToolUseContent{
		ID:    id,
		Name:  driven.ToolAskQuestion,
		Input: map[string]any{"question": text, "why": "because"},
	}
}

func submit(id string, payload map[string]any) *driven.ToolUseContent {
	return &driven.ToolUseContent{
		ID:    id,
		Name:  driven.ToolSubmitProposal,
		Input: payload,
	}
}

// ---------------------------------------------------------------------
// Happy paths
// ---------------------------------------------------------------------

func TestRunDialog_HappyInterviewThenAccept(t *testing.T) {
	payload := map[string]any{"title": "A login story"}
	llm := &fakeLLM{
		script: []scriptedTurn{
			{toolUse: askQ("q1", "who logs in?")},
			{toolUse: askQ("q2", "how?")},
			{toolUse: submit("p1", payload)},
		},
	}
	inter := &fakeInter{
		answers: []driven.Answer{
			{Kind: driven.AnswerReply, Text: "end users"},
			{Kind: driven.AnswerReply, Text: "via OAuth"},
		},
		decisions: []driven.Decision{{Kind: driven.DecisionAccept}},
	}
	var applied map[string]any
	d := newDialog()
	d.apply = func(_ context.Context, p map[string]any) (any, error) {
		applied = p
		return "applied-ok", nil
	}

	result, err := runDialog(context.Background(), d, llm, inter, nullStatus{}, &fakeClock{})
	if err != nil {
		t.Fatalf("runDialog: %v", err)
	}
	if result != "applied-ok" {
		t.Fatalf("result: got %v, want applied-ok", result)
	}
	if applied["title"] != "A login story" {
		t.Fatalf("apply saw wrong payload: %v", applied)
	}
	if len(inter.seenQs) != 2 {
		t.Fatalf("expected 2 questions asked, got %d", len(inter.seenQs))
	}
	if len(inter.seenProps) != 1 {
		t.Fatalf("expected 1 review, got %d", len(inter.seenProps))
	}
	if d.llmTurns != 3 {
		t.Fatalf("llmTurns: got %d, want 3", d.llmTurns)
	}
}

func TestRunDialog_ZeroInterviewThenAccept(t *testing.T) {
	llm := &fakeLLM{
		script: []scriptedTurn{
			{toolUse: submit("p1", map[string]any{"title": "skip questions"})},
		},
	}
	inter := &fakeInter{
		decisions: []driven.Decision{{Kind: driven.DecisionAccept}},
	}
	d := newDialog()
	d.apply = func(_ context.Context, p map[string]any) (any, error) { return p["title"], nil }

	result, err := runDialog(context.Background(), d, llm, inter, nullStatus{}, &fakeClock{})
	if err != nil {
		t.Fatalf("runDialog: %v", err)
	}
	if result != "skip questions" {
		t.Fatalf("result: %v", result)
	}
	if len(inter.seenQs) != 0 {
		t.Fatalf("expected 0 questions, got %d", len(inter.seenQs))
	}
}

func TestRunDialog_CritiqueLoopThenAccept(t *testing.T) {
	llm := &fakeLLM{
		script: []scriptedTurn{
			{toolUse: submit("p1", map[string]any{"title": "first try"})},
			{toolUse: submit("p2", map[string]any{"title": "better try"})},
		},
	}
	inter := &fakeInter{
		decisions: []driven.Decision{
			{Kind: driven.DecisionCritique, Critique: "needs more detail"},
			{Kind: driven.DecisionAccept},
		},
	}
	d := newDialog()
	d.apply = func(_ context.Context, p map[string]any) (any, error) { return p["title"], nil }

	result, err := runDialog(context.Background(), d, llm, inter, nullStatus{}, &fakeClock{})
	if err != nil {
		t.Fatalf("runDialog: %v", err)
	}
	if result != "better try" {
		t.Fatalf("result: %v", result)
	}
	if len(inter.seenProps) != 2 {
		t.Fatalf("expected 2 reviews, got %d", len(inter.seenProps))
	}
	// The second ChatRequest should contain the critique feedback in its
	// messages (the tool_use + tool_result pair we appended).
	req2 := llm.requests[1]
	foundCritique := false
	for _, m := range req2.Messages {
		if m.ToolResult != nil && strings.Contains(m.ToolResult.Content, "needs more detail") {
			foundCritique = true
		}
	}
	if !foundCritique {
		t.Fatalf("expected critique to be fed back to LLM on second turn")
	}
}

func TestRunDialog_ForceProposeAfterDone(t *testing.T) {
	llm := &fakeLLM{
		script: []scriptedTurn{
			{toolUse: askQ("q1", "a question")},
			{toolUse: submit("p1", map[string]any{"title": "forced proposal"})},
		},
	}
	inter := &fakeInter{
		answers:   []driven.Answer{{Kind: driven.AnswerForcePropose}},
		decisions: []driven.Decision{{Kind: driven.DecisionAccept}},
	}
	d := newDialog()
	d.apply = func(_ context.Context, p map[string]any) (any, error) { return p["title"], nil }

	result, err := runDialog(context.Background(), d, llm, inter, nullStatus{}, &fakeClock{})
	if err != nil {
		t.Fatalf("runDialog: %v", err)
	}
	if result != "forced proposal" {
		t.Fatalf("result: %v", result)
	}
	// The second LLM turn should include a tool_result telling the model
	// to propose now.
	req2 := llm.requests[1]
	foundForce := false
	for _, m := range req2.Messages {
		if m.ToolResult != nil && strings.Contains(m.ToolResult.Content, "propose now") {
			foundForce = true
		}
	}
	if !foundForce {
		t.Fatalf("expected force-propose message to be fed to LLM; got messages: %+v", req2.Messages)
	}
}

// ---------------------------------------------------------------------
// Aborts
// ---------------------------------------------------------------------

func TestRunDialog_AbortMidInterview(t *testing.T) {
	llm := &fakeLLM{
		script: []scriptedTurn{
			{toolUse: askQ("q1", "a question")},
		},
	}
	inter := &fakeInter{
		answers: []driven.Answer{{Kind: driven.AnswerAbort}},
	}
	d := newDialog()
	d.apply = func(_ context.Context, _ map[string]any) (any, error) {
		t.Fatalf("apply should not be called on abort")
		return nil, nil
	}

	_, err := runDialog(context.Background(), d, llm, inter, nullStatus{}, &fakeClock{})
	if !errors.Is(err, ErrAIDialogAborted) {
		t.Fatalf("expected ErrAIDialogAborted, got %v", err)
	}
}

func TestRunDialog_AbortAtReview(t *testing.T) {
	llm := &fakeLLM{
		script: []scriptedTurn{
			{toolUse: submit("p1", map[string]any{"title": "x"})},
		},
	}
	inter := &fakeInter{
		decisions: []driven.Decision{{Kind: driven.DecisionAbort}},
	}
	d := newDialog()
	d.apply = func(_ context.Context, _ map[string]any) (any, error) {
		t.Fatalf("apply should not be called on abort")
		return nil, nil
	}

	_, err := runDialog(context.Background(), d, llm, inter, nullStatus{}, &fakeClock{})
	if !errors.Is(err, ErrAIDialogAborted) {
		t.Fatalf("expected ErrAIDialogAborted, got %v", err)
	}
}

// ---------------------------------------------------------------------
// Malformed responses
// ---------------------------------------------------------------------

func TestRunDialog_MalformedThenSuccess(t *testing.T) {
	llm := &fakeLLM{
		script: []scriptedTurn{
			// First turn: model returned plain text instead of calling a tool.
			{rawText: "I don't want to call a tool"},
			{toolUse: submit("p1", map[string]any{"title": "recovered"})},
		},
	}
	inter := &fakeInter{
		decisions: []driven.Decision{{Kind: driven.DecisionAccept}},
	}
	d := newDialog()
	d.malformedBudget = 2
	d.apply = func(_ context.Context, p map[string]any) (any, error) { return p["title"], nil }

	result, err := runDialog(context.Background(), d, llm, inter, nullStatus{}, &fakeClock{})
	if err != nil {
		t.Fatalf("runDialog: %v", err)
	}
	if result != "recovered" {
		t.Fatalf("result: %v", result)
	}
	// The second ChatRequest should include the reminder text.
	req2 := llm.requests[1]
	found := false
	for _, m := range req2.Messages {
		if m.Text != "" && strings.Contains(m.Text, "available tools") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected malformed-retry reminder to be appended")
	}
}

func TestRunDialog_MalformedBudgetExhausted(t *testing.T) {
	// Three malformed responses with a budget of 2 retries.
	llm := &fakeLLM{
		script: []scriptedTurn{
			{rawText: "nope"},
			{rawText: "still nope"},
			{rawText: "nope again"},
		},
	}
	inter := &fakeInter{}
	d := newDialog()
	d.malformedBudget = 2
	d.apply = func(_ context.Context, _ map[string]any) (any, error) {
		t.Fatalf("apply should not be called")
		return nil, nil
	}

	_, err := runDialog(context.Background(), d, llm, inter, nullStatus{}, &fakeClock{})
	if err == nil {
		t.Fatalf("expected failure")
	}
	if errors.Is(err, ErrAIDialogAborted) {
		t.Fatalf("unexpected abort: %v", err)
	}
	if !strings.Contains(err.Error(), "valid tool call") {
		t.Fatalf("expected malformed failure message, got %v", err)
	}
	// The exhausted error should carry a preview of the model's last
	// response so the founder has something to go on.
	if !strings.Contains(err.Error(), "text=nope again") {
		t.Fatalf("expected final error to quote last raw text, got %v", err)
	}
}

// When the model hits max_tokens, the retry reminder should say so
// explicitly rather than the generic "you didn't call a tool" hint.
func TestRunDialog_MalformedMaxTokensHint(t *testing.T) {
	llm := &fakeLLM{
		script: []scriptedTurn{
			{rawText: "still thinking…", stopReason: "max_tokens"},
			{toolUse: submit("p1", map[string]any{"title": "recovered"})},
		},
	}
	inter := &fakeInter{
		decisions: []driven.Decision{{Kind: driven.DecisionAccept}},
	}
	d := newDialog()
	d.malformedBudget = 2
	d.apply = func(_ context.Context, p map[string]any) (any, error) { return p["title"], nil }

	_, err := runDialog(context.Background(), d, llm, inter, nullStatus{}, &fakeClock{})
	if err != nil {
		t.Fatalf("runDialog: %v", err)
	}
	// The retry turn (request index 1) must include a max_tokens-specific
	// reminder — not the generic "call a tool" one.
	req2 := llm.requests[1]
	found := false
	for _, m := range req2.Messages {
		if strings.Contains(m.Text, "stop_reason: max_tokens") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected max_tokens-specific retry hint")
	}
}

// When a malformed turn carries text, the assistant turn must be
// recorded in history so the model sees what it said when it retries.
func TestRunDialog_MalformedLogsAssistantText(t *testing.T) {
	llm := &fakeLLM{
		script: []scriptedTurn{
			{rawText: "I'm going to think out loud"},
			{toolUse: submit("p1", map[string]any{"title": "recovered"})},
		},
	}
	inter := &fakeInter{
		decisions: []driven.Decision{{Kind: driven.DecisionAccept}},
	}
	d := newDialog()
	d.malformedBudget = 2
	d.apply = func(_ context.Context, p map[string]any) (any, error) { return p["title"], nil }

	_, err := runDialog(context.Background(), d, llm, inter, nullStatus{}, &fakeClock{})
	if err != nil {
		t.Fatalf("runDialog: %v", err)
	}
	req2 := llm.requests[1]
	foundAssistant := false
	for _, m := range req2.Messages {
		if m.Role == driven.RoleAssistant && strings.Contains(m.Text, "think out loud") {
			foundAssistant = true
		}
	}
	if !foundAssistant {
		t.Fatalf("expected the malformed assistant turn to be recorded in history")
	}
}

// ---------------------------------------------------------------------
// Rate-limit / backoff
// ---------------------------------------------------------------------

func TestRunDialog_RateLimitedThenSuccess(t *testing.T) {
	llm := &fakeLLM{
		script: []scriptedTurn{
			{err: driven.ErrAIRateLimited, retryAfter: 50 * time.Millisecond},
			{toolUse: submit("p1", map[string]any{"title": "ok"})},
		},
	}
	inter := &fakeInter{
		decisions: []driven.Decision{{Kind: driven.DecisionAccept}},
	}
	clock := &fakeClock{}
	d := newDialog()
	d.backoffBudget = 2
	d.backoffBase = 10 * time.Millisecond
	d.apply = func(_ context.Context, p map[string]any) (any, error) { return p["title"], nil }

	result, err := runDialog(context.Background(), d, llm, inter, nullStatus{}, clock)
	if err != nil {
		t.Fatalf("runDialog: %v", err)
	}
	if result != "ok" {
		t.Fatalf("result: %v", result)
	}
	// Retry-After hint was honored.
	if len(clock.sleeps) != 1 || clock.sleeps[0] != 50*time.Millisecond {
		t.Fatalf("expected one 50ms sleep, got %v", clock.sleeps)
	}
}

func TestRunDialog_RateLimitedBudgetExhausted(t *testing.T) {
	llm := &fakeLLM{
		script: []scriptedTurn{
			{err: driven.ErrAIRateLimited},
			{err: driven.ErrAIRateLimited},
			{err: driven.ErrAIRateLimited},
		},
	}
	inter := &fakeInter{}
	clock := &fakeClock{}
	d := newDialog()
	d.backoffBudget = 2
	d.apply = func(_ context.Context, _ map[string]any) (any, error) {
		t.Fatalf("apply should not be called")
		return nil, nil
	}

	_, err := runDialog(context.Background(), d, llm, inter, nullStatus{}, clock)
	if err == nil {
		t.Fatalf("expected failure")
	}
	if !errors.Is(err, driven.ErrAIRateLimited) {
		t.Fatalf("expected rate-limited error to propagate, got %v", err)
	}
	if !strings.Contains(err.Error(), "backoff budget") {
		t.Fatalf("expected 'exhausted backoff budget' message, got %v", err)
	}
	// Two sleeps: budget is 2 retries.
	if len(clock.sleeps) != 2 {
		t.Fatalf("expected 2 sleeps, got %d (%v)", len(clock.sleeps), clock.sleeps)
	}
	// Exponential backoff: second sleep doubles the first.
	if clock.sleeps[1] != 2*clock.sleeps[0] {
		t.Fatalf("expected exponential doubling, got %v", clock.sleeps)
	}
}

// ---------------------------------------------------------------------
// Fail-fast errors
// ---------------------------------------------------------------------

func TestRunDialog_AuthFailureFailsFast(t *testing.T) {
	llm := &fakeLLM{
		script: []scriptedTurn{{err: driven.ErrAIAuthFailed}},
	}
	inter := &fakeInter{}
	clock := &fakeClock{}
	d := newDialog()

	_, err := runDialog(context.Background(), d, llm, inter, nullStatus{}, clock)
	if !errors.Is(err, driven.ErrAIAuthFailed) {
		t.Fatalf("expected ErrAIAuthFailed, got %v", err)
	}
	if len(clock.sleeps) != 0 {
		t.Fatalf("auth failure should not backoff; got sleeps: %v", clock.sleeps)
	}
}

func TestRunDialog_ContextTooLargeFailsFast(t *testing.T) {
	llm := &fakeLLM{
		script: []scriptedTurn{{err: driven.ErrAIContextTooLarge}},
	}
	inter := &fakeInter{}
	clock := &fakeClock{}
	d := newDialog()

	_, err := runDialog(context.Background(), d, llm, inter, nullStatus{}, clock)
	if !errors.Is(err, driven.ErrAIContextTooLarge) {
		t.Fatalf("expected ErrAIContextTooLarge, got %v", err)
	}
	if len(clock.sleeps) != 0 {
		t.Fatalf("context-too-large should not backoff")
	}
}

// Invalid-request (e.g. a 400 from a malformed payload) is the error
// class that previously burned the backoff budget. It must fail fast
// with a single call — no sleeps, no retries.
func TestRunDialog_InvalidRequestFailsFast(t *testing.T) {
	llm := &fakeLLM{
		script: []scriptedTurn{
			{err: driven.ErrAIInvalidRequest},
			// Intentionally a second entry to prove we don't retry — if
			// the loop wrongly retries, it would consume this and mask
			// the bug.
			{err: driven.ErrAIInvalidRequest},
		},
	}
	inter := &fakeInter{}
	clock := &fakeClock{}
	d := newDialog()

	_, err := runDialog(context.Background(), d, llm, inter, nullStatus{}, clock)
	if !errors.Is(err, driven.ErrAIInvalidRequest) {
		t.Fatalf("expected ErrAIInvalidRequest, got %v", err)
	}
	if len(clock.sleeps) != 0 {
		t.Fatalf("invalid-request should not backoff; got sleeps: %v", clock.sleeps)
	}
	if llm.idx != 1 {
		t.Fatalf("expected exactly 1 LLM call, got %d", llm.idx)
	}
}

// ---------------------------------------------------------------------
// Interview budget
// ---------------------------------------------------------------------

func TestRunDialog_MaxInterviewsForcesPropose(t *testing.T) {
	// Budget = 2 questions. Model tries to ask 2, then must propose.
	llm := &fakeLLM{
		script: []scriptedTurn{
			{toolUse: askQ("q1", "q1")},
			{toolUse: askQ("q2", "q2")},
			{toolUse: submit("p1", map[string]any{"title": "done"})},
		},
	}
	inter := &fakeInter{
		answers: []driven.Answer{
			{Kind: driven.AnswerReply, Text: "a1"},
			{Kind: driven.AnswerReply, Text: "a2"},
		},
		decisions: []driven.Decision{{Kind: driven.DecisionAccept}},
	}
	d := newDialog()
	d.maxInterviews = 2
	d.apply = func(_ context.Context, p map[string]any) (any, error) { return p["title"], nil }

	result, err := runDialog(context.Background(), d, llm, inter, nullStatus{}, &fakeClock{})
	if err != nil {
		t.Fatalf("runDialog: %v", err)
	}
	if result != "done" {
		t.Fatalf("result: %v", result)
	}
	// After the 2nd question, a user-role text message should be
	// appended telling the model it's out of interview budget.
	req3 := llm.requests[2]
	found := false
	for _, m := range req3.Messages {
		if m.Role == driven.RoleUser && strings.Contains(m.Text, "submit_proposal now") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected 'submit_proposal now' nudge after budget exhausted")
	}
}

// ---------------------------------------------------------------------
// Apply / validate
// ---------------------------------------------------------------------

func TestRunDialog_ApplyErrorFails(t *testing.T) {
	llm := &fakeLLM{
		script: []scriptedTurn{
			{toolUse: submit("p1", map[string]any{"title": "x"})},
		},
	}
	inter := &fakeInter{
		decisions: []driven.Decision{{Kind: driven.DecisionAccept}},
	}
	applyErr := errors.New("disk full")
	d := newDialog()
	d.apply = func(_ context.Context, _ map[string]any) (any, error) { return nil, applyErr }

	_, err := runDialog(context.Background(), d, llm, inter, nullStatus{}, &fakeClock{})
	if !errors.Is(err, applyErr) {
		t.Fatalf("expected apply error to propagate, got %v", err)
	}
}

func TestRunDialog_ValidateFailureTriggersRetry(t *testing.T) {
	llm := &fakeLLM{
		script: []scriptedTurn{
			{toolUse: submit("p1", map[string]any{"title": "bad"})},
			{toolUse: submit("p2", map[string]any{"title": "good"})},
		},
	}
	inter := &fakeInter{
		decisions: []driven.Decision{{Kind: driven.DecisionAccept}},
	}
	d := newDialog()
	d.malformedBudget = 2
	d.validate = func(p map[string]any) error {
		if p["title"] == "bad" {
			return errors.New("title is bad")
		}
		return nil
	}
	d.apply = func(_ context.Context, p map[string]any) (any, error) { return p["title"], nil }

	result, err := runDialog(context.Background(), d, llm, inter, nullStatus{}, &fakeClock{})
	if err != nil {
		t.Fatalf("runDialog: %v", err)
	}
	if result != "good" {
		t.Fatalf("result: %v", result)
	}
}

func TestRunDialog_EditedProposalValidatesAndApplies(t *testing.T) {
	llm := &fakeLLM{
		script: []scriptedTurn{
			{toolUse: submit("p1", map[string]any{"title": "original"})},
		},
	}
	inter := &fakeInter{
		decisions: []driven.Decision{
			{Kind: driven.DecisionAccept, Edited: `{"title":"edited"}`},
		},
	}
	var got map[string]any
	d := newDialog()
	d.validate = func(p map[string]any) error {
		if p["title"] == "" {
			return errors.New("empty title")
		}
		return nil
	}
	d.apply = func(_ context.Context, p map[string]any) (any, error) {
		got = p
		return p["title"], nil
	}

	result, err := runDialog(context.Background(), d, llm, inter, nullStatus{}, &fakeClock{})
	if err != nil {
		t.Fatalf("runDialog: %v", err)
	}
	if result != "edited" {
		t.Fatalf("result: %v", result)
	}
	if got["title"] != "edited" {
		t.Fatalf("apply saw %v, want edited", got)
	}
}

func TestRunDialog_EditedProposalInvalidJSONFails(t *testing.T) {
	llm := &fakeLLM{
		script: []scriptedTurn{
			{toolUse: submit("p1", map[string]any{"title": "original"})},
		},
	}
	inter := &fakeInter{
		decisions: []driven.Decision{
			{Kind: driven.DecisionAccept, Edited: `not-json`},
		},
	}
	d := newDialog()
	d.apply = func(_ context.Context, _ map[string]any) (any, error) {
		t.Fatalf("apply should not be called")
		return nil, nil
	}

	_, err := runDialog(context.Background(), d, llm, inter, nullStatus{}, &fakeClock{})
	if err == nil || !strings.Contains(err.Error(), "not valid JSON") {
		t.Fatalf("expected JSON parse error, got %v", err)
	}
}

func TestRunDialog_EditedProposalFailsValidation(t *testing.T) {
	llm := &fakeLLM{
		script: []scriptedTurn{
			{toolUse: submit("p1", map[string]any{"title": "ok"})},
		},
	}
	inter := &fakeInter{
		decisions: []driven.Decision{
			{Kind: driven.DecisionAccept, Edited: `{"title":""}`},
		},
	}
	d := newDialog()
	d.validate = func(p map[string]any) error {
		if p["title"] == "" {
			return errors.New("title required")
		}
		return nil
	}
	d.apply = func(_ context.Context, _ map[string]any) (any, error) {
		t.Fatalf("apply should not be called")
		return nil, nil
	}

	_, err := runDialog(context.Background(), d, llm, inter, nullStatus{}, &fakeClock{})
	if err == nil || !strings.Contains(err.Error(), "failed validation") {
		t.Fatalf("expected edited-validation error, got %v", err)
	}
}

// ---------------------------------------------------------------------
// Defaults
// ---------------------------------------------------------------------

// The first Chat() call must never be made with an empty messages
// array — Anthropic's Messages API rejects that with 400
// "messages: at least one message is required". runDialog seeds a
// kickoff user message when the caller didn't pre-populate one.
func TestRunDialog_SeedsKickoffWhenMessagesEmpty(t *testing.T) {
	llm := &fakeLLM{
		script: []scriptedTurn{
			{toolUse: submit("p1", map[string]any{"title": "ok"})},
		},
	}
	inter := &fakeInter{
		decisions: []driven.Decision{{Kind: driven.DecisionAccept}},
	}
	d := newDialog()
	d.apply = func(_ context.Context, _ map[string]any) (any, error) { return nil, nil }

	_, err := runDialog(context.Background(), d, llm, inter, nullStatus{}, &fakeClock{})
	if err != nil {
		t.Fatalf("runDialog: %v", err)
	}
	if len(llm.requests) == 0 {
		t.Fatalf("expected at least one LLM request")
	}
	first := llm.requests[0]
	if len(first.Messages) == 0 {
		t.Fatalf("first Chat call had empty messages array; " +
			"Anthropic would 400 this request")
	}
	if first.Messages[0].Role != driven.RoleUser {
		t.Fatalf("kickoff message must be role=user, got %v", first.Messages[0].Role)
	}
	if strings.TrimSpace(first.Messages[0].Text) == "" {
		t.Fatalf("kickoff message must have non-empty text")
	}
}

// When the caller pre-seeds messages, the kickoff logic must not
// clobber or duplicate them.
func TestRunDialog_DoesNotOverrideSeededMessages(t *testing.T) {
	llm := &fakeLLM{
		script: []scriptedTurn{
			{toolUse: submit("p1", map[string]any{"title": "ok"})},
		},
	}
	inter := &fakeInter{
		decisions: []driven.Decision{{Kind: driven.DecisionAccept}},
	}
	d := newDialog()
	d.messages = []driven.Message{
		{Role: driven.RoleUser, Text: "caller seeded this"},
	}
	d.apply = func(_ context.Context, _ map[string]any) (any, error) { return nil, nil }

	_, err := runDialog(context.Background(), d, llm, inter, nullStatus{}, &fakeClock{})
	if err != nil {
		t.Fatalf("runDialog: %v", err)
	}
	first := llm.requests[0]
	if len(first.Messages) != 1 {
		t.Fatalf("expected exactly the seeded message, got %d", len(first.Messages))
	}
	if first.Messages[0].Text != "caller seeded this" {
		t.Fatalf("seeded message was clobbered: %q", first.Messages[0].Text)
	}
}

func TestRunDialog_DefaultsMaxTokens(t *testing.T) {
	llm := &fakeLLM{
		script: []scriptedTurn{
			{toolUse: submit("p1", map[string]any{"title": "x"})},
		},
	}
	inter := &fakeInter{
		decisions: []driven.Decision{{Kind: driven.DecisionAccept}},
	}
	d := newDialog()
	d.maxTokensPerTurn = 0 // trigger default
	d.apply = func(_ context.Context, _ map[string]any) (any, error) { return nil, nil }

	_, err := runDialog(context.Background(), d, llm, inter, nullStatus{}, &fakeClock{})
	if err != nil {
		t.Fatalf("runDialog: %v", err)
	}
	if llm.requests[0].MaxTokens != 4096 {
		t.Fatalf("expected MaxTokens default 4096, got %d", llm.requests[0].MaxTokens)
	}
}

// ---------------------------------------------------------------------
// Multi-item review paths (suggest)
// ---------------------------------------------------------------------

func TestRunDialog_MultiItemAcceptAll(t *testing.T) {
	payload := map[string]any{
		"items": []any{
			map[string]any{"title": "A", "description": "x"},
			map[string]any{"title": "B", "description": "y"},
			map[string]any{"title": "C", "description": "z"},
		},
	}
	llm := &fakeLLM{
		script: []scriptedTurn{{toolUse: submit("p1", payload)}},
	}
	inter := &fakeInter{
		decisions: []driven.Decision{{
			Kind: driven.DecisionAccept,
			PerItem: []driven.ItemDecision{
				{Kind: driven.ItemYes},
				{Kind: driven.ItemYes},
				{Kind: driven.ItemYes},
			},
		}},
	}
	var applied map[string]any
	d := newDialog()
	d.apply = func(_ context.Context, p map[string]any) (any, error) {
		applied = p
		return "ok", nil
	}

	_, err := runDialog(context.Background(), d, llm, inter, nullStatus{}, &fakeClock{})
	if err != nil {
		t.Fatalf("runDialog: %v", err)
	}
	acc, ok := applied["_accepted"].([]int)
	if !ok {
		t.Fatalf("_accepted missing or wrong type: %T", applied["_accepted"])
	}
	if len(acc) != 3 || acc[0] != 0 || acc[1] != 1 || acc[2] != 2 {
		t.Fatalf("_accepted: got %v, want [0 1 2]", acc)
	}
}

func TestRunDialog_MultiItemAcceptSubset(t *testing.T) {
	payload := map[string]any{
		"items": []any{
			map[string]any{"title": "A"},
			map[string]any{"title": "B"},
			map[string]any{"title": "C"},
		},
	}
	llm := &fakeLLM{
		script: []scriptedTurn{{toolUse: submit("p1", payload)}},
	}
	inter := &fakeInter{
		decisions: []driven.Decision{{
			Kind: driven.DecisionAccept,
			PerItem: []driven.ItemDecision{
				{Kind: driven.ItemYes},
				{Kind: driven.ItemNo},
				{Kind: driven.ItemYes},
			},
		}},
	}
	var applied map[string]any
	d := newDialog()
	d.apply = func(_ context.Context, p map[string]any) (any, error) {
		applied = p
		return "ok", nil
	}

	_, err := runDialog(context.Background(), d, llm, inter, nullStatus{}, &fakeClock{})
	if err != nil {
		t.Fatalf("runDialog: %v", err)
	}
	acc, _ := applied["_accepted"].([]int)
	if len(acc) != 2 || acc[0] != 0 || acc[1] != 2 {
		t.Fatalf("_accepted: got %v, want [0 2]", acc)
	}
}

func TestRunDialog_MultiItemAcceptNoneAborts(t *testing.T) {
	payload := map[string]any{
		"items": []any{
			map[string]any{"title": "A"},
			map[string]any{"title": "B"},
		},
	}
	llm := &fakeLLM{
		script: []scriptedTurn{{toolUse: submit("p1", payload)}},
	}
	inter := &fakeInter{
		decisions: []driven.Decision{{
			Kind: driven.DecisionAccept,
			PerItem: []driven.ItemDecision{
				{Kind: driven.ItemNo},
				{Kind: driven.ItemNo},
			},
		}},
	}
	d := newDialog()
	d.apply = func(_ context.Context, _ map[string]any) (any, error) {
		t.Fatalf("apply should not be called when no items are accepted")
		return nil, nil
	}

	_, err := runDialog(context.Background(), d, llm, inter, nullStatus{}, &fakeClock{})
	if !errors.Is(err, ErrAIDialogAborted) {
		t.Fatalf("expected ErrAIDialogAborted, got %v", err)
	}
}

func TestRunDialog_MultiItemEditList(t *testing.T) {
	original := map[string]any{
		"items": []any{
			map[string]any{"title": "original"},
		},
	}
	edited := `{"items":[{"title":"edited-one"},{"title":"edited-two"}]}`
	llm := &fakeLLM{
		script: []scriptedTurn{{toolUse: submit("p1", original)}},
	}
	inter := &fakeInter{
		decisions: []driven.Decision{{
			Kind:   driven.DecisionEditList,
			Edited: edited,
		}},
	}
	var applied map[string]any
	d := newDialog()
	d.apply = func(_ context.Context, p map[string]any) (any, error) {
		applied = p
		return "ok", nil
	}

	_, err := runDialog(context.Background(), d, llm, inter, nullStatus{}, &fakeClock{})
	if err != nil {
		t.Fatalf("runDialog: %v", err)
	}
	items, _ := applied["items"].([]any)
	if len(items) != 2 {
		t.Fatalf("expected 2 items after edit, got %d", len(items))
	}
	acc, _ := applied["_accepted"].([]int)
	if len(acc) != 2 || acc[0] != 0 || acc[1] != 1 {
		t.Fatalf("_accepted after edit: got %v, want [0 1]", acc)
	}
}

func TestRunDialog_MultiItemEditListBadJSON(t *testing.T) {
	llm := &fakeLLM{
		script: []scriptedTurn{{toolUse: submit("p1", map[string]any{"items": []any{}})}},
	}
	inter := &fakeInter{
		decisions: []driven.Decision{{
			Kind:   driven.DecisionEditList,
			Edited: "not json",
		}},
	}
	d := newDialog()
	d.apply = func(_ context.Context, _ map[string]any) (any, error) {
		t.Fatalf("apply should not be called on bad-JSON edit")
		return nil, nil
	}

	_, err := runDialog(context.Background(), d, llm, inter, nullStatus{}, &fakeClock{})
	if err == nil {
		t.Fatalf("expected error on bad-JSON edit, got nil")
	}
	if !strings.Contains(err.Error(), "edited list is not valid JSON") {
		t.Fatalf("expected 'edited list is not valid JSON' error, got %v", err)
	}
}

func TestRunDialog_MultiItemEditListFailsValidation(t *testing.T) {
	// Initial proposal is valid; the founder edits it into something
	// that fails validation and the failure routes to stateFailed.
	llm := &fakeLLM{
		script: []scriptedTurn{{toolUse: submit("p1", map[string]any{
			"items": []any{map[string]any{"title": "fine"}},
		})}},
	}
	inter := &fakeInter{
		decisions: []driven.Decision{{
			Kind:   driven.DecisionEditList,
			Edited: `{"items":[{"title":""}]}`,
		}},
	}
	d := newDialog()
	d.validate = func(m map[string]any) error {
		items, _ := m["items"].([]any)
		if len(items) == 0 {
			return fmt.Errorf("items must be non-empty")
		}
		first, _ := items[0].(map[string]any)
		if first["title"] == "" {
			return fmt.Errorf("title is required")
		}
		return nil
	}
	d.apply = func(_ context.Context, _ map[string]any) (any, error) {
		t.Fatalf("apply should not be called on validation failure")
		return nil, nil
	}

	_, err := runDialog(context.Background(), d, llm, inter, nullStatus{}, &fakeClock{})
	if err == nil {
		t.Fatalf("expected validation error")
	}
	if !strings.Contains(err.Error(), "edited list failed validation") {
		t.Fatalf("expected 'edited list failed validation' error, got %v", err)
	}
}
