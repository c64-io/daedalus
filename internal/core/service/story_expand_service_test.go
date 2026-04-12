package service_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/c64-io/daedalus/internal/core/domain"
	"github.com/c64-io/daedalus/internal/core/port/driven"
	"github.com/c64-io/daedalus/internal/core/port/driving"
	"github.com/c64-io/daedalus/internal/core/service"
)

// ---------------------------------------------------------------------
// AI-side fakes (mirrors the ones in ai_loop_test.go, redeclared here
// so service_test does not depend on internal symbols).
// ---------------------------------------------------------------------

type expScriptedTurn struct {
	err     error
	toolUse *driven.ToolUseContent
	rawText string
}

type expFakeLLM struct {
	script   []expScriptedTurn
	idx      int
	requests []driven.ChatRequest
}

func (f *expFakeLLM) Chat(_ context.Context, req driven.ChatRequest) (*driven.ChatResponse, error) {
	f.requests = append(f.requests, req)
	if f.idx >= len(f.script) {
		return nil, fmt.Errorf("expFakeLLM: script exhausted at %d", f.idx)
	}
	t := f.script[f.idx]
	f.idx++
	if t.err != nil {
		return nil, t.err
	}
	return &driven.ChatResponse{ToolUse: t.toolUse, RawText: t.rawText}, nil
}

type expFakeInter struct {
	answers    []driven.Answer
	decisions  []driven.Decision
	askIdx     int
	reviewIdx  int
	seenQs     []driven.Question
	seenProps  []driven.Proposal
}

func (f *expFakeInter) Ask(_ context.Context, q driven.Question) (driven.Answer, error) {
	f.seenQs = append(f.seenQs, q)
	if f.askIdx >= len(f.answers) {
		return driven.Answer{}, fmt.Errorf("expFakeInter: answer script exhausted")
	}
	a := f.answers[f.askIdx]
	f.askIdx++
	return a, nil
}

func (f *expFakeInter) Review(_ context.Context, p driven.Proposal) (driven.Decision, error) {
	f.seenProps = append(f.seenProps, p)
	if f.reviewIdx >= len(f.decisions) {
		return driven.Decision{}, fmt.Errorf("expFakeInter: decision script exhausted")
	}
	d := f.decisions[f.reviewIdx]
	f.reviewIdx++
	return d, nil
}

type expNullStatus struct{}

func (expNullStatus) Start(_ context.Context, _ driven.Status) {}
func (expNullStatus) Update(_ driven.Status)                   {}
func (expNullStatus) Stop()                                    {}

type expFixedClock struct{ now time.Time }

func (c *expFixedClock) Now() time.Time        { return c.now }
func (c *expFixedClock) Sleep(_ time.Duration) {}

// ---------------------------------------------------------------------
// Shared setup: build a workspace with a full parent chain and a target
// story that the expand-service can interrogate.
// ---------------------------------------------------------------------

type expandFixture struct {
	svc       *service.StoryExpandService
	fs        *fakeFS
	storyRepo *fakeStoryRepo
	history   *fakeHistoryRepo
	llm       *expFakeLLM
	inter     *expFakeInter
	clock     *expFixedClock
}

func newExpandFixture(t *testing.T, llmScript []expScriptedTurn, answers []driven.Answer, decisions []driven.Decision) *expandFixture {
	t.Helper()

	fs := newFakeFS("/proj")
	fs.existing["/proj/d7/.db"] = true

	wsRepo := &fakeRepo{
		description:    "A coupon SaaS.",
		descriptionSet: true,
	}

	ideaRepo := &fakeIdeaRepo{}
	ideaRepo.ideas = append(ideaRepo.ideas, domain.Idea{
		ID:          "IDEA-001",
		Title:       "Coupons",
		Description: "Shoppers redeem coupons at checkout.",
		Status:      domain.StatusRefined,
		CreatedAt:   time.Now(),
	})

	epicRepo := &fakeEpicRepo{}
	epicRepo.epics = append(epicRepo.epics, domain.Epic{
		ID:          "EPIC-001",
		IdeaID:      "IDEA-001",
		Title:       "Redemption flow",
		Description: "End-to-end coupon redemption.",
		Status:      domain.StatusRefined,
		CreatedAt:   time.Now(),
	})

	featureRepo := &fakeFeatureRepo{}
	featureRepo.features = append(featureRepo.features, domain.Feature{
		ID:          "FEAT-001",
		EpicID:      "EPIC-001",
		Title:       "Checkout redemption",
		Description: "Apply coupons at checkout.",
		Status:      domain.StatusRefined,
		CreatedAt:   time.Now(),
	})

	storyRepo := &fakeStoryRepo{}
	storyRepo.stories = append(storyRepo.stories, domain.Story{
		ID:          "STORY-001",
		FeatureID:   "FEAT-001",
		Title:       "User redeems a coupon",
		Description: "",
		Status:      domain.StatusDraft,
		CreatedAt:   time.Now(),
	})

	linkRepo := &fakeLinkRepo{}
	refRepo := &fakeRefRepo{}
	resolver := &fakeEntityResolver{entities: map[string]string{
		"IDEA-001":  "Coupons",
		"EPIC-001":  "Redemption flow",
		"FEAT-001":  "Checkout redemption",
		"STORY-001": "User redeems a coupon",
	}}
	history := &fakeHistoryRepo{}

	llm := &expFakeLLM{script: llmScript}
	inter := &expFakeInter{answers: answers, decisions: decisions}
	clock := &expFixedClock{now: time.Date(2026, 4, 12, 12, 0, 0, 0, time.UTC)}

	svc := service.NewStoryExpandService(
		fs, wsRepo, ideaRepo, epicRepo, featureRepo, storyRepo,
		linkRepo, refRepo, resolver, history,
		llm, inter, expNullStatus{}, clock,
	)

	return &expandFixture{
		svc:       svc,
		fs:        fs,
		storyRepo: storyRepo,
		history:   history,
		llm:       llm,
		inter:     inter,
		clock:     clock,
	}
}

// ---------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------

func TestExpandStory_HappyPath(t *testing.T) {
	t.Parallel()

	f := newExpandFixture(t,
		[]expScriptedTurn{
			{toolUse: &driven.ToolUseContent{
				ID: "q1", Name: driven.ToolAskQuestion,
				Input: map[string]any{"question": "who is this for?", "why": "audience matters"},
			}},
			{toolUse: &driven.ToolUseContent{
				ID: "p1", Name: driven.ToolSubmitProposal,
				Input: map[string]any{"description": "A shopper at checkout applies a coupon code and sees a discount."},
			}},
		},
		[]driven.Answer{{Kind: driven.AnswerReply, Text: "end shoppers"}},
		[]driven.Decision{{Kind: driven.DecisionAccept}},
	)

	got, err := f.svc.ExpandStory(context.Background(), driving.ExpandStoryRequest{StoryID: "STORY-001"})
	if err != nil {
		t.Fatalf("ExpandStory: %v", err)
	}
	wantDesc := "A shopper at checkout applies a coupon code and sees a discount."
	if got.Description != wantDesc {
		t.Fatalf("description: got %q, want %q", got.Description, wantDesc)
	}

	// The context block handed to the LLM must include the whole ancestor
	// chain and the workspace description.
	if len(f.llm.requests) == 0 {
		t.Fatalf("expected at least one LLM call")
	}
	ctxBlock := f.llm.requests[0].Context
	for _, needle := range []string{
		"A coupon SaaS.",
		"IDEA-001", "EPIC-001", "FEAT-001",
		"User redeems a coupon",
	} {
		if !strings.Contains(ctxBlock, needle) {
			t.Errorf("context block missing %q; got:\n%s", needle, ctxBlock)
		}
	}

	// History entry was appended for the description change.
	if len(f.history.entries) != 1 {
		t.Fatalf("expected 1 history entry, got %d", len(f.history.entries))
	}
	if f.history.entries[0].Field != "description" {
		t.Errorf("history field: got %q, want %q", f.history.entries[0].Field, "description")
	}

	// The story in the repo was updated.
	saved := f.storyRepo.stories[0]
	if saved.Description != wantDesc {
		t.Errorf("repo not updated; description = %q", saved.Description)
	}
}

func TestExpandStory_ZeroInterview(t *testing.T) {
	t.Parallel()

	f := newExpandFixture(t,
		[]expScriptedTurn{
			{toolUse: &driven.ToolUseContent{
				ID: "p1", Name: driven.ToolSubmitProposal,
				Input: map[string]any{"description": "Direct proposal."},
			}},
		},
		nil,
		[]driven.Decision{{Kind: driven.DecisionAccept}},
	)

	got, err := f.svc.ExpandStory(context.Background(), driving.ExpandStoryRequest{StoryID: "STORY-001"})
	if err != nil {
		t.Fatalf("ExpandStory: %v", err)
	}
	if got.Description != "Direct proposal." {
		t.Fatalf("description: %q", got.Description)
	}
	if len(f.inter.seenQs) != 0 {
		t.Fatalf("expected 0 questions, got %d", len(f.inter.seenQs))
	}
}

func TestExpandStory_AbortNoWrite(t *testing.T) {
	t.Parallel()

	f := newExpandFixture(t,
		[]expScriptedTurn{
			{toolUse: &driven.ToolUseContent{
				ID: "q1", Name: driven.ToolAskQuestion,
				Input: map[string]any{"question": "who is this for?"},
			}},
		},
		[]driven.Answer{{Kind: driven.AnswerAbort}},
		nil,
	)

	_, err := f.svc.ExpandStory(context.Background(), driving.ExpandStoryRequest{StoryID: "STORY-001"})
	if !errors.Is(err, service.ErrAIDialogAborted) {
		t.Fatalf("expected ErrAIDialogAborted, got %v", err)
	}
	// No description written.
	if f.storyRepo.stories[0].Description != "" {
		t.Fatalf("description should be unchanged; got %q", f.storyRepo.stories[0].Description)
	}
	// No history entry.
	if len(f.history.entries) != 0 {
		t.Fatalf("expected 0 history entries on abort, got %d", len(f.history.entries))
	}
}

func TestExpandStory_StoryIDRequired(t *testing.T) {
	t.Parallel()
	f := newExpandFixture(t, nil, nil, nil)
	_, err := f.svc.ExpandStory(context.Background(), driving.ExpandStoryRequest{StoryID: ""})
	if !errors.Is(err, service.ErrExpandStoryIDRequired) {
		t.Fatalf("expected ErrExpandStoryIDRequired, got %v", err)
	}
}

func TestExpandStory_StoryNotFound(t *testing.T) {
	t.Parallel()
	f := newExpandFixture(t, nil, nil, nil)
	_, err := f.svc.ExpandStory(context.Background(), driving.ExpandStoryRequest{StoryID: "STORY-999"})
	if err == nil || !errors.Is(err, driven.ErrStoryNotFound) {
		t.Fatalf("expected ErrStoryNotFound, got %v", err)
	}
}

func TestExpandStory_ValidatorRejectsEmptyDescription(t *testing.T) {
	t.Parallel()

	// First proposal has an empty description — validator should treat it
	// as malformed and force a retry. Second proposal is valid.
	f := newExpandFixture(t,
		[]expScriptedTurn{
			{toolUse: &driven.ToolUseContent{
				ID: "p1", Name: driven.ToolSubmitProposal,
				Input: map[string]any{"description": "   "},
			}},
			{toolUse: &driven.ToolUseContent{
				ID: "p2", Name: driven.ToolSubmitProposal,
				Input: map[string]any{"description": "Now with content."},
			}},
		},
		nil,
		[]driven.Decision{{Kind: driven.DecisionAccept}},
	)

	got, err := f.svc.ExpandStory(context.Background(), driving.ExpandStoryRequest{StoryID: "STORY-001"})
	if err != nil {
		t.Fatalf("ExpandStory: %v", err)
	}
	if got.Description != "Now with content." {
		t.Fatalf("description: %q", got.Description)
	}
}

func TestExpandStory_NoChangeSkipsHistoryWrite(t *testing.T) {
	t.Parallel()

	// Seed the story with a description, then have the model propose
	// the same text. The service should detect no change and skip the
	// history append / update.
	f := newExpandFixture(t,
		[]expScriptedTurn{
			{toolUse: &driven.ToolUseContent{
				ID: "p1", Name: driven.ToolSubmitProposal,
				Input: map[string]any{"description": "unchanged"},
			}},
		},
		nil,
		[]driven.Decision{{Kind: driven.DecisionAccept}},
	)
	f.storyRepo.stories[0].Description = "unchanged"

	_, err := f.svc.ExpandStory(context.Background(), driving.ExpandStoryRequest{StoryID: "STORY-001"})
	if err != nil {
		t.Fatalf("ExpandStory: %v", err)
	}
	if len(f.history.entries) != 0 {
		t.Fatalf("expected no history entries for no-change, got %d", len(f.history.entries))
	}
}
