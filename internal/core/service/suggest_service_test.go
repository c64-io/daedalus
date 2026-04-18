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
// Fake driving Creator ports — one per child entity type. Each records
// the CreateX requests it saw so tests can assert on the result.
// ---------------------------------------------------------------------

type fakeEpicCreator struct {
	reqs []driving.CreateEpicRequest
	seq  int
	err  error
}

func (f *fakeEpicCreator) CreateEpic(_ context.Context, req driving.CreateEpicRequest) (*domain.Epic, error) {
	f.reqs = append(f.reqs, req)
	if f.err != nil {
		return nil, f.err
	}
	f.seq++
	return &domain.Epic{
		ID:          fmt.Sprintf("EPIC-%03d", 100+f.seq),
		IdeaID:      req.IdeaID,
		Title:       req.Title,
		Description: req.Description,
		Status:      domain.StatusDraft,
		CreatedAt:   time.Now(),
	}, nil
}

type fakeFeatureCreator struct {
	reqs []driving.CreateFeatureRequest
	seq  int
	err  error
}

func (f *fakeFeatureCreator) CreateFeature(_ context.Context, req driving.CreateFeatureRequest) (*domain.Feature, error) {
	f.reqs = append(f.reqs, req)
	if f.err != nil {
		return nil, f.err
	}
	f.seq++
	return &domain.Feature{
		ID:          fmt.Sprintf("FEAT-%03d", 100+f.seq),
		EpicID:      req.EpicID,
		Title:       req.Title,
		Description: req.Description,
		Status:      domain.StatusDraft,
		CreatedAt:   time.Now(),
	}, nil
}

type fakeStoryCreator struct {
	reqs []driving.CreateStoryRequest
	seq  int
	err  error
}

func (f *fakeStoryCreator) CreateStory(_ context.Context, req driving.CreateStoryRequest) (*domain.Story, error) {
	f.reqs = append(f.reqs, req)
	if f.err != nil {
		return nil, f.err
	}
	f.seq++
	return &domain.Story{
		ID:          fmt.Sprintf("STORY-%03d", 100+f.seq),
		FeatureID:   req.FeatureID,
		Title:       req.Title,
		Description: req.Description,
		Status:      domain.StatusDraft,
		CreatedAt:   time.Now(),
	}, nil
}

type fakeSpecCreator struct {
	reqs []driving.CreateSpecRequest
	seq  int
	err  error
}

func (f *fakeSpecCreator) CreateSpec(_ context.Context, req driving.CreateSpecRequest) (*domain.Spec, error) {
	f.reqs = append(f.reqs, req)
	if f.err != nil {
		return nil, f.err
	}
	f.seq++
	return &domain.Spec{
		ID:          fmt.Sprintf("SPEC-%03d", 100+f.seq),
		StoryID:     req.StoryID,
		Title:       req.Title,
		Description: req.Description,
		Status:      domain.StatusDraft,
		CreatedAt:   time.Now(),
	}, nil
}

type fakeScenarioCreator struct {
	reqs []driving.CreateScenarioRequest
	seq  int
	err  error
}

func (f *fakeScenarioCreator) CreateScenario(_ context.Context, req driving.CreateScenarioRequest) (*domain.Scenario, error) {
	f.reqs = append(f.reqs, req)
	if f.err != nil {
		return nil, f.err
	}
	f.seq++
	return &domain.Scenario{
		ID:        fmt.Sprintf("SCEN-%03d", 100+f.seq),
		SpecID:    req.SpecID,
		Title:     req.Title,
		Given:     req.Given,
		When:      req.When,
		Then:      req.Then,
		Tags:      req.Tags,
		Status:    domain.StatusDraft,
		CreatedAt: time.Now(),
	}, nil
}

// ---------------------------------------------------------------------
// Fixture — builds a fully-seeded tree + fake Creators and returns the
// SuggestService under test.
// ---------------------------------------------------------------------

type suggestFixture struct {
	svc *service.SuggestService

	ideaRepo  *fakeIdeaRepo
	epicRepo  *fakeEpicRepo
	featRepo  *fakeFeatureRepo
	storyRepo *fakeStoryRepo
	specRepo  *fakeSpecRepo

	epicCreator     *fakeEpicCreator
	featureCreator  *fakeFeatureCreator
	storyCreator    *fakeStoryCreator
	specCreator     *fakeSpecCreator
	scenarioCreator *fakeScenarioCreator

	llm   *expFakeLLM
	inter *expFakeInter
	clock *expFixedClock
}

func newSuggestFixture(t *testing.T, llmScript []expScriptedTurn, answers []driven.Answer, decisions []driven.Decision) *suggestFixture {
	t.Helper()

	fs := newFakeFS("/proj")
	fs.existing["/proj/d7/.db"] = true

	wsRepo := &fakeRepo{description: "A coupon SaaS.", descriptionSet: true}

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

	featRepo := &fakeFeatureRepo{}
	featRepo.features = append(featRepo.features, domain.Feature{
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
		Description: "Shopper enters a code at checkout.",
		Status:      domain.StatusRefined,
		CreatedAt:   time.Now(),
	})

	specRepo := &fakeSpecRepo{}
	specRepo.specs = append(specRepo.specs, domain.Spec{
		ID:          "SPEC-001",
		StoryID:     "STORY-001",
		Title:       "Redemption rules",
		Description: "Valid codes deduct the amount; expired codes show an error.",
		Status:      domain.StatusRefined,
		CreatedAt:   time.Now(),
	})

	linkRepo := &fakeLinkRepo{}
	refRepo := &fakeRefRepo{}
	resolver := &fakeEntityResolver{entities: map[string]string{
		"IDEA-001":  "Coupons",
		"EPIC-001":  "Redemption flow",
		"FEAT-001":  "Checkout redemption",
		"STORY-001": "User redeems a coupon",
		"SPEC-001":  "Redemption rules",
	}}

	epicCreator := &fakeEpicCreator{}
	featureCreator := &fakeFeatureCreator{}
	storyCreator := &fakeStoryCreator{}
	specCreator := &fakeSpecCreator{}
	scenarioCreator := &fakeScenarioCreator{}

	llm := &expFakeLLM{script: llmScript}
	inter := &expFakeInter{answers: answers, decisions: decisions}
	clock := &expFixedClock{now: time.Date(2026, 4, 14, 12, 0, 0, 0, time.UTC)}

	scenRepo := &fakeScenarioRepo{}

	svc := service.NewSuggestService(
		fs, wsRepo, ideaRepo, epicRepo, featRepo, storyRepo, specRepo,
		scenRepo,
		linkRepo, refRepo, resolver,
		epicCreator, featureCreator, storyCreator, specCreator, scenarioCreator,
		llm, inter, expNullStatus{}, clock,
	)

	return &suggestFixture{
		svc:             svc,
		ideaRepo:        ideaRepo,
		epicRepo:        epicRepo,
		featRepo:        featRepo,
		storyRepo:       storyRepo,
		specRepo:        specRepo,
		epicCreator:     epicCreator,
		featureCreator:  featureCreator,
		storyCreator:    storyCreator,
		specCreator:     specCreator,
		scenarioCreator: scenarioCreator,
		llm:             llm,
		inter:           inter,
		clock:           clock,
	}
}

// submitMulti builds a submit_proposal tool_use content with the shared
// {items: [...]} shape used by epics/features/stories/specs.
func submitMulti(items []map[string]any) *driven.ToolUseContent {
	arr := make([]any, len(items))
	for i := range items {
		arr[i] = items[i]
	}
	return &driven.ToolUseContent{
		ID:    "p1",
		Name:  driven.ToolSubmitProposal,
		Input: map[string]any{"items": arr},
	}
}

// submitScenarios builds a submit_proposal tool_use content with the
// scenario-specific shape.
func submitScenarios(items []map[string]any) *driven.ToolUseContent {
	arr := make([]any, len(items))
	for i := range items {
		arr[i] = items[i]
	}
	return &driven.ToolUseContent{
		ID:    "p1",
		Name:  driven.ToolSubmitProposal,
		Input: map[string]any{"items": arr},
	}
}

// acceptAll builds a PerItem slice of all-Yes decisions for n items,
// matching what cliinteraction would produce for the `a` (all) input.
func acceptAll(n int) driven.Decision {
	per := make([]driven.ItemDecision, n)
	for i := range per {
		per[i] = driven.ItemDecision{Kind: driven.ItemYes}
	}
	return driven.Decision{Kind: driven.DecisionAccept, PerItem: per}
}

// acceptSubset builds a PerItem slice marking only the given 0-based
// indices as Yes.
func acceptSubset(n int, picks ...int) driven.Decision {
	set := map[int]struct{}{}
	for _, p := range picks {
		set[p] = struct{}{}
	}
	per := make([]driven.ItemDecision, n)
	for i := range per {
		if _, ok := set[i]; ok {
			per[i] = driven.ItemDecision{Kind: driven.ItemYes}
		} else {
			per[i] = driven.ItemDecision{Kind: driven.ItemNo}
		}
	}
	return driven.Decision{Kind: driven.DecisionAccept, PerItem: per}
}

// acceptNone builds a PerItem slice of all-No decisions; ai_loop routes
// this to ErrAIDialogAborted.
func acceptNone(n int) driven.Decision {
	per := make([]driven.ItemDecision, n)
	for i := range per {
		per[i] = driven.ItemDecision{Kind: driven.ItemNo}
	}
	return driven.Decision{Kind: driven.DecisionAccept, PerItem: per}
}

// ---------------------------------------------------------------------
// SuggestEpics
// ---------------------------------------------------------------------

func TestSuggestEpics_HappyPath_AcceptAll(t *testing.T) {
	t.Parallel()
	items := []map[string]any{
		{"title": "Redemption", "description": "Enter a coupon at checkout."},
		{"title": "Campaigns", "description": "Create time-boxed coupon campaigns."},
	}
	f := newSuggestFixture(t,
		[]expScriptedTurn{{toolUse: submitMulti(items)}},
		nil,
		[]driven.Decision{acceptAll(len(items))},
	)
	got, err := f.svc.SuggestEpics(context.Background(), driving.SuggestEpicsRequest{IdeaID: "IDEA-001"})
	if err != nil {
		t.Fatalf("SuggestEpics: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d epics, want 2", len(got))
	}
	if len(f.epicCreator.reqs) != 2 {
		t.Fatalf("got %d CreateEpic calls, want 2", len(f.epicCreator.reqs))
	}
	if f.epicCreator.reqs[0].IdeaID != "IDEA-001" {
		t.Errorf("CreateEpic[0].IdeaID: got %q, want IDEA-001", f.epicCreator.reqs[0].IdeaID)
	}
	if f.epicCreator.reqs[0].Title != "Redemption" {
		t.Errorf("CreateEpic[0].Title: got %q", f.epicCreator.reqs[0].Title)
	}

	// Context block: workspace + target only (IDEA-001). No descendants.
	ctxBlock := f.llm.requests[0].Context
	if !strings.Contains(ctxBlock, "IDEA-001") {
		t.Errorf("context missing target IDEA-001:\n%s", ctxBlock)
	}
	for _, forbidden := range []string{"EPIC-001", "FEAT-001"} {
		if strings.Contains(ctxBlock, forbidden) {
			t.Errorf("context should not contain %q:\n%s", forbidden, ctxBlock)
		}
	}
}

func TestSuggestEpics_AcceptSubset_CreatesOnlyPicked(t *testing.T) {
	t.Parallel()
	items := []map[string]any{
		{"title": "A", "description": "first"},
		{"title": "B", "description": "second"},
		{"title": "C", "description": "third"},
	}
	f := newSuggestFixture(t,
		[]expScriptedTurn{{toolUse: submitMulti(items)}},
		nil,
		[]driven.Decision{acceptSubset(3, 0, 2)},
	)
	got, err := f.svc.SuggestEpics(context.Background(), driving.SuggestEpicsRequest{IdeaID: "IDEA-001"})
	if err != nil {
		t.Fatalf("SuggestEpics: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d epics, want 2", len(got))
	}
	if len(f.epicCreator.reqs) != 2 {
		t.Fatalf("CreateEpic calls: got %d, want 2", len(f.epicCreator.reqs))
	}
	titles := []string{f.epicCreator.reqs[0].Title, f.epicCreator.reqs[1].Title}
	if titles[0] != "A" || titles[1] != "C" {
		t.Errorf("created titles: got %v, want [A C]", titles)
	}
}

func TestSuggestEpics_AcceptNone_Aborts(t *testing.T) {
	t.Parallel()
	items := []map[string]any{
		{"title": "A", "description": "first"},
		{"title": "B", "description": "second"},
	}
	f := newSuggestFixture(t,
		[]expScriptedTurn{{toolUse: submitMulti(items)}},
		nil,
		[]driven.Decision{acceptNone(2)},
	)
	_, err := f.svc.SuggestEpics(context.Background(), driving.SuggestEpicsRequest{IdeaID: "IDEA-001"})
	if !errors.Is(err, service.ErrAIDialogAborted) {
		t.Fatalf("expected ErrAIDialogAborted, got %v", err)
	}
	if len(f.epicCreator.reqs) != 0 {
		t.Errorf("no epics should be created on abort; got %d", len(f.epicCreator.reqs))
	}
}

func TestSuggestEpics_EmptyIDIsError(t *testing.T) {
	t.Parallel()
	f := newSuggestFixture(t, nil, nil, nil)
	_, err := f.svc.SuggestEpics(context.Background(), driving.SuggestEpicsRequest{IdeaID: ""})
	if err == nil {
		t.Fatalf("expected error for empty IdeaID")
	}
	if !errors.Is(err, service.ErrSuggestIDRequired) {
		t.Fatalf("got %v, want ErrSuggestIDRequired", err)
	}
}

func TestSuggestEpics_CreatorError_ReturnsPartialAndWraps(t *testing.T) {
	t.Parallel()
	items := []map[string]any{
		{"title": "A", "description": "first"},
	}
	f := newSuggestFixture(t,
		[]expScriptedTurn{{toolUse: submitMulti(items)}},
		nil,
		[]driven.Decision{acceptAll(1)},
	)
	f.epicCreator.err = fmt.Errorf("parent not refined")
	_, err := f.svc.SuggestEpics(context.Background(), driving.SuggestEpicsRequest{IdeaID: "IDEA-001"})
	if err == nil {
		t.Fatalf("expected creator error to propagate")
	}
	if !strings.Contains(err.Error(), "create epic") {
		t.Errorf("expected wrapping to include 'create epic'; got %v", err)
	}
}

// ---------------------------------------------------------------------
// SuggestFeatures — context has target only; ancestors via nav tools.
// ---------------------------------------------------------------------

func TestSuggestFeatures_ContextHasTargetOnly(t *testing.T) {
	t.Parallel()
	items := []map[string]any{
		{"title": "Checkout", "description": "Apply at checkout."},
	}
	f := newSuggestFixture(t,
		[]expScriptedTurn{{toolUse: submitMulti(items)}},
		nil,
		[]driven.Decision{acceptAll(1)},
	)
	_, err := f.svc.SuggestFeatures(context.Background(), driving.SuggestFeaturesRequest{EpicID: "EPIC-001"})
	if err != nil {
		t.Fatalf("SuggestFeatures: %v", err)
	}
	ctxBlock := f.llm.requests[0].Context
	if !strings.Contains(ctxBlock, "EPIC-001") {
		t.Errorf("context missing target EPIC-001:\n%s", ctxBlock)
	}
	if strings.Contains(ctxBlock, "IDEA-001") {
		t.Errorf("context should not contain ancestor IDEA-001:\n%s", ctxBlock)
	}
	if len(f.featureCreator.reqs) != 1 {
		t.Errorf("CreateFeature calls: got %d, want 1", len(f.featureCreator.reqs))
	}
	if f.featureCreator.reqs[0].EpicID != "EPIC-001" {
		t.Errorf("CreateFeature.EpicID: got %q", f.featureCreator.reqs[0].EpicID)
	}
}

// ---------------------------------------------------------------------
// SuggestStories
// ---------------------------------------------------------------------

func TestSuggestStories_HappyPath(t *testing.T) {
	t.Parallel()
	items := []map[string]any{
		{"title": "Enter code", "description": "Shopper enters a code at checkout."},
	}
	f := newSuggestFixture(t,
		[]expScriptedTurn{{toolUse: submitMulti(items)}},
		nil,
		[]driven.Decision{acceptAll(1)},
	)
	got, err := f.svc.SuggestStories(context.Background(), driving.SuggestStoriesRequest{FeatureID: "FEAT-001"})
	if err != nil {
		t.Fatalf("SuggestStories: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d, want 1", len(got))
	}
	if f.storyCreator.reqs[0].FeatureID != "FEAT-001" {
		t.Errorf("CreateStory.FeatureID: got %q", f.storyCreator.reqs[0].FeatureID)
	}
}

// ---------------------------------------------------------------------
// SuggestSpecs
// ---------------------------------------------------------------------

func TestSuggestSpecs_HappyPath(t *testing.T) {
	t.Parallel()
	items := []map[string]any{
		{"title": "Expiry", "description": "Codes past their end date are rejected."},
	}
	f := newSuggestFixture(t,
		[]expScriptedTurn{{toolUse: submitMulti(items)}},
		nil,
		[]driven.Decision{acceptAll(1)},
	)
	got, err := f.svc.SuggestSpecs(context.Background(), driving.SuggestSpecsRequest{StoryID: "STORY-001"})
	if err != nil {
		t.Fatalf("SuggestSpecs: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d, want 1", len(got))
	}
	if f.specCreator.reqs[0].StoryID != "STORY-001" {
		t.Errorf("CreateSpec.StoryID: got %q", f.specCreator.reqs[0].StoryID)
	}
}

// ---------------------------------------------------------------------
// SuggestScenarios — payload carries Given/When/Then
// ---------------------------------------------------------------------

func TestSuggestScenarios_HappyPath_StepsPassedThrough(t *testing.T) {
	t.Parallel()
	items := []map[string]any{
		{
			"title": "Valid code deducts amount",
			"given": []any{"a shopper has a valid code"},
			"when":  []any{"the code is applied at checkout"},
			"then":  []any{"the order total drops by the discount", "the receipt shows the discount line"},
		},
		{
			"title": "Expired code is rejected",
			"given": []any{"a shopper has a code past its end date"},
			"when":  []any{"the code is applied"},
			"then":  []any{"an 'expired' error is shown"},
		},
	}
	f := newSuggestFixture(t,
		[]expScriptedTurn{{toolUse: submitScenarios(items)}},
		nil,
		[]driven.Decision{acceptAll(2)},
	)
	got, err := f.svc.SuggestScenarios(context.Background(), driving.SuggestScenariosRequest{SpecID: "SPEC-001"})
	if err != nil {
		t.Fatalf("SuggestScenarios: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d scenarios, want 2", len(got))
	}
	if len(f.scenarioCreator.reqs) != 2 {
		t.Fatalf("CreateScenario calls: got %d, want 2", len(f.scenarioCreator.reqs))
	}
	first := f.scenarioCreator.reqs[0]
	if first.SpecID != "SPEC-001" {
		t.Errorf("CreateScenario[0].SpecID: got %q", first.SpecID)
	}
	if len(first.Given) != 1 || first.Given[0].Text != "a shopper has a valid code" {
		t.Errorf("Given not passed through: got %+v", first.Given)
	}
	if len(first.When) != 1 || first.When[0].Text != "the code is applied at checkout" {
		t.Errorf("When not passed through: got %+v", first.When)
	}
	if len(first.Then) != 2 {
		t.Errorf("Then count: got %d, want 2", len(first.Then))
	}
}

func TestSuggestScenarios_ValidatorRejectsMissingWhen(t *testing.T) {
	t.Parallel()
	// One item has no `when` — the validator should reject the whole
	// payload. The LLM only gets one chance in the script, so the loop
	// exhausts malformed retries and returns a failure.
	bad := []map[string]any{
		{
			"title": "Missing when",
			"given": []any{"a precondition"},
			"then":  []any{"an outcome"},
		},
	}
	f := newSuggestFixture(t,
		[]expScriptedTurn{
			{toolUse: submitScenarios(bad)},
			{toolUse: submitScenarios(bad)},
			{toolUse: submitScenarios(bad)},
		},
		nil,
		nil, // never reaches review
	)
	_, err := f.svc.SuggestScenarios(context.Background(), driving.SuggestScenariosRequest{SpecID: "SPEC-001"})
	if err == nil {
		t.Fatalf("expected validation error")
	}
	if len(f.scenarioCreator.reqs) != 0 {
		t.Errorf("no scenarios should be created; got %d", len(f.scenarioCreator.reqs))
	}
}

// ---------------------------------------------------------------------
// EditList path end-to-end: the founder rewrites the items array and
// the rewritten items are what gets created.
// ---------------------------------------------------------------------

func TestSuggestEpics_EditListReplacesItems(t *testing.T) {
	t.Parallel()
	original := []map[string]any{
		{"title": "original-only", "description": "the AI's draft"},
	}
	edited := `{"items":[{"title":"edited-A","description":"founder's rewrite A"},{"title":"edited-B","description":"founder's rewrite B"}]}`
	f := newSuggestFixture(t,
		[]expScriptedTurn{{toolUse: submitMulti(original)}},
		nil,
		[]driven.Decision{{Kind: driven.DecisionEditList, Edited: edited}},
	)
	got, err := f.svc.SuggestEpics(context.Background(), driving.SuggestEpicsRequest{IdeaID: "IDEA-001"})
	if err != nil {
		t.Fatalf("SuggestEpics: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d epics, want 2", len(got))
	}
	if f.epicCreator.reqs[0].Title != "edited-A" || f.epicCreator.reqs[1].Title != "edited-B" {
		t.Errorf("titles after edit: got [%q %q], want [edited-A edited-B]",
			f.epicCreator.reqs[0].Title, f.epicCreator.reqs[1].Title)
	}
}

// ---------------------------------------------------------------------
// Pre-flight parent-status checks — every SuggestX command must fail
// fast when the parent entity isn't at least `refined`, before the
// dialog starts, so the founder doesn't sit through a full interview
// only to hit the state-machine at apply time. Each test sets the
// direct parent to `draft` and asserts:
//   1. the command errors with the expected *NotReady sentinel,
//   2. zero LLM turns were issued, and
//   3. zero Creator calls were made.
// ---------------------------------------------------------------------

func TestSuggestEpics_ParentIdeaDraft_FailsFastBeforeDialog(t *testing.T) {
	t.Parallel()
	f := newSuggestFixture(t, nil, nil, nil)
	f.ideaRepo.ideas[0].Status = domain.StatusDraft

	_, err := f.svc.SuggestEpics(context.Background(), driving.SuggestEpicsRequest{IdeaID: "IDEA-001"})
	if !errors.Is(err, service.ErrIdeaNotReady) {
		t.Fatalf("expected ErrIdeaNotReady, got %v", err)
	}
	if len(f.llm.requests) != 0 {
		t.Errorf("pre-flight should short-circuit before LLM; got %d requests", len(f.llm.requests))
	}
	if len(f.epicCreator.reqs) != 0 {
		t.Errorf("no epics should be created on pre-flight failure; got %d", len(f.epicCreator.reqs))
	}
}

func TestSuggestFeatures_ParentEpicDraft_FailsFastBeforeDialog(t *testing.T) {
	t.Parallel()
	f := newSuggestFixture(t, nil, nil, nil)
	f.epicRepo.epics[0].Status = domain.StatusDraft

	_, err := f.svc.SuggestFeatures(context.Background(), driving.SuggestFeaturesRequest{EpicID: "EPIC-001"})
	if !errors.Is(err, service.ErrEpicNotReady) {
		t.Fatalf("expected ErrEpicNotReady, got %v", err)
	}
	if len(f.llm.requests) != 0 {
		t.Errorf("pre-flight should short-circuit before LLM; got %d requests", len(f.llm.requests))
	}
	if len(f.featureCreator.reqs) != 0 {
		t.Errorf("no features should be created on pre-flight failure; got %d", len(f.featureCreator.reqs))
	}
}

func TestSuggestStories_ParentFeatureDraft_FailsFastBeforeDialog(t *testing.T) {
	t.Parallel()
	f := newSuggestFixture(t, nil, nil, nil)
	f.featRepo.features[0].Status = domain.StatusDraft

	_, err := f.svc.SuggestStories(context.Background(), driving.SuggestStoriesRequest{FeatureID: "FEAT-001"})
	if !errors.Is(err, service.ErrFeatureNotReady) {
		t.Fatalf("expected ErrFeatureNotReady, got %v", err)
	}
	if len(f.llm.requests) != 0 {
		t.Errorf("pre-flight should short-circuit before LLM; got %d requests", len(f.llm.requests))
	}
	if len(f.storyCreator.reqs) != 0 {
		t.Errorf("no stories should be created on pre-flight failure; got %d", len(f.storyCreator.reqs))
	}
}

func TestSuggestSpecs_ParentStoryDraft_FailsFastBeforeDialog(t *testing.T) {
	t.Parallel()
	f := newSuggestFixture(t, nil, nil, nil)
	f.storyRepo.stories[0].Status = domain.StatusDraft

	_, err := f.svc.SuggestSpecs(context.Background(), driving.SuggestSpecsRequest{StoryID: "STORY-001"})
	if !errors.Is(err, service.ErrStoryNotReady) {
		t.Fatalf("expected ErrStoryNotReady, got %v", err)
	}
	if len(f.llm.requests) != 0 {
		t.Errorf("pre-flight should short-circuit before LLM; got %d requests", len(f.llm.requests))
	}
	if len(f.specCreator.reqs) != 0 {
		t.Errorf("no specs should be created on pre-flight failure; got %d", len(f.specCreator.reqs))
	}
}

func TestSuggestScenarios_ParentSpecDraft_FailsFastBeforeDialog(t *testing.T) {
	t.Parallel()
	f := newSuggestFixture(t, nil, nil, nil)
	f.specRepo.specs[0].Status = domain.StatusDraft

	_, err := f.svc.SuggestScenarios(context.Background(), driving.SuggestScenariosRequest{SpecID: "SPEC-001"})
	if !errors.Is(err, service.ErrSpecNotReady) {
		t.Fatalf("expected ErrSpecNotReady, got %v", err)
	}
	if len(f.llm.requests) != 0 {
		t.Errorf("pre-flight should short-circuit before LLM; got %d requests", len(f.llm.requests))
	}
	if len(f.scenarioCreator.reqs) != 0 {
		t.Errorf("no scenarios should be created on pre-flight failure; got %d", len(f.scenarioCreator.reqs))
	}
}

// Non-ready intermediate-ancestor statuses do not block the command —
// only the *direct* parent gates child creation. If, for example, a
// Feature's grandparent Idea were archived, the dialog should still
// run (the Feature is refined, the Story is being created under the
// Feature, not under the Idea). This test locks in that behavior so
// a future "walk the whole chain" mistake is caught.
func TestSuggestStories_OnlyDirectParentGates_NotAncestors(t *testing.T) {
	t.Parallel()
	items := []map[string]any{
		{"title": "Enter code", "description": "Shopper enters a code at checkout."},
	}
	f := newSuggestFixture(t,
		[]expScriptedTurn{{toolUse: submitMulti(items)}},
		nil,
		[]driven.Decision{acceptAll(1)},
	)
	// Grandparent (Idea) and great-grandparent don't gate — only the
	// direct parent (Feature) does. Push the Idea to draft; the dialog
	// should still run because FEAT-001 is still refined.
	f.ideaRepo.ideas[0].Status = domain.StatusDraft

	_, err := f.svc.SuggestStories(context.Background(), driving.SuggestStoriesRequest{FeatureID: "FEAT-001"})
	if err != nil {
		t.Fatalf("SuggestStories should succeed with only ancestor in draft: %v", err)
	}
	if len(f.storyCreator.reqs) != 1 {
		t.Errorf("expected 1 CreateStory call; got %d", len(f.storyCreator.reqs))
	}
}

// TestSuggestEpics_NavTool_GetSiblings_ThenSubmit scripts a dialog where the
// model calls get_siblings before submitting a batch. This verifies that nav
// tool results appear in subsequent LLM requests and that the proposal is
// applied correctly.
func TestSuggestEpics_NavTool_GetSiblings_ThenSubmit(t *testing.T) {
	t.Parallel()

	items := []map[string]any{
		{"title": "Coupon analytics", "description": "Track redemption rates."},
	}
	f := newSuggestFixture(t,
		[]expScriptedTurn{
			// Turn 1: model calls get_siblings to see existing epics
			{toolUse: &driven.ToolUseContent{
				ID: "nav1", Name: driven.ToolGetSiblings,
				Input: map[string]any{"id": "EPIC-001"},
			}},
			// Turn 2: model submits proposal
			{toolUse: submitMulti(items)},
		},
		nil,
		[]driven.Decision{acceptAll(1)},
	)

	_, err := f.svc.SuggestEpics(context.Background(), driving.SuggestEpicsRequest{IdeaID: "IDEA-001"})
	if err != nil {
		t.Fatalf("SuggestEpics: %v", err)
	}

	// 2 LLM calls: nav tool, then submit_proposal.
	if len(f.llm.requests) != 2 {
		t.Fatalf("expected 2 LLM requests, got %d", len(f.llm.requests))
	}

	// The second request should contain the nav tool result in messages.
	msgs := f.llm.requests[1].Messages
	foundNavResult := false
	for _, m := range msgs {
		if m.ToolResult != nil && strings.Contains(m.ToolResult.Content, "(no siblings)") {
			foundNavResult = true
			break
		}
	}
	if !foundNavResult {
		t.Error("second LLM request should contain get_siblings tool result")
	}

	// 1 epic was created.
	if len(f.epicCreator.reqs) != 1 {
		t.Errorf("expected 1 CreateEpic call; got %d", len(f.epicCreator.reqs))
	}
	if f.epicCreator.reqs[0].Title != "Coupon analytics" {
		t.Errorf("title: got %q", f.epicCreator.reqs[0].Title)
	}
}
