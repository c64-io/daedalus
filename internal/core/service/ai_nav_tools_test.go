package service_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/c64-io/daedalus/internal/core/domain"
	"github.com/c64-io/daedalus/internal/core/port/driven"
	"github.com/c64-io/daedalus/internal/core/service"
)

// navFixture builds a NavToolsHandler against the standard fake repos,
// seeded with the usual coupon-system hierarchy.
type navFixture struct {
	handler  *service.NavToolsHandler
	ideaRepo *fakeIdeaRepo
	epicRepo *fakeEpicRepo
	featRepo *fakeFeatureRepo
	storyRepo *fakeStoryRepo
	specRepo  *fakeSpecRepo
	scenRepo  *fakeScenarioRepo
	linkRepo  *fakeLinkRepo
	refRepo   *fakeRefRepo
	resolver  *fakeEntityResolver
}

func newNavFixture() *navFixture {
	ideaRepo := &fakeIdeaRepo{}
	ideaRepo.ideas = append(ideaRepo.ideas, domain.Idea{
		ID: "IDEA-001", Title: "Coupons", Description: "Shoppers redeem coupons.",
		Status: domain.StatusRefined, CreatedAt: time.Now(),
	})

	epicRepo := &fakeEpicRepo{}
	epicRepo.epics = append(epicRepo.epics, domain.Epic{
		ID: "EPIC-001", IdeaID: "IDEA-001", Title: "Redemption", Description: "End-to-end redemption.",
		Status: domain.StatusRefined, CreatedAt: time.Now(),
	})
	epicRepo.epics = append(epicRepo.epics, domain.Epic{
		ID: "EPIC-002", IdeaID: "IDEA-001", Title: "Promotions", Description: "Promo campaigns.",
		Status: domain.StatusDraft, CreatedAt: time.Now(),
	})

	featRepo := &fakeFeatureRepo{}
	featRepo.features = append(featRepo.features, domain.Feature{
		ID: "FEAT-001", EpicID: "EPIC-001", Title: "Checkout redemption", Description: "Apply coupons at checkout.",
		Status: domain.StatusRefined, CreatedAt: time.Now(),
	})

	storyRepo := &fakeStoryRepo{}
	storyRepo.stories = append(storyRepo.stories, domain.Story{
		ID: "STORY-001", FeatureID: "FEAT-001", Title: "User redeems a coupon", Description: "Enter code at checkout.",
		Status: domain.StatusRefined, CreatedAt: time.Now(),
	})

	specRepo := &fakeSpecRepo{}
	specRepo.specs = append(specRepo.specs, domain.Spec{
		ID: "SPEC-001", StoryID: "STORY-001", Title: "Redemption rules",
		Description: "Valid codes deduct; expired show error.", Status: domain.StatusRefined, CreatedAt: time.Now(),
	})

	scenRepo := &fakeScenarioRepo{}
	scenRepo.scenarios = append(scenRepo.scenarios, domain.Scenario{
		ID: "SCEN-001", SpecID: "SPEC-001", Title: "Redeem valid coupon",
		Given: []domain.Step{{Text: "user has coupon SAVE10"}},
		When:  []domain.Step{{Text: "user applies at checkout"}},
		Then:  []domain.Step{{Text: "total reduced by 10%"}},
		Tags:  []string{"happy-path"},
		Status: domain.StatusDraft, CreatedAt: time.Now(),
	})

	linkRepo := &fakeLinkRepo{}
	refRepo := &fakeRefRepo{}
	resolver := &fakeEntityResolver{entities: map[string]string{
		"IDEA-001":  "Coupons",
		"EPIC-001":  "Redemption",
		"EPIC-002":  "Promotions",
		"FEAT-001":  "Checkout redemption",
		"STORY-001": "User redeems a coupon",
		"SPEC-001":  "Redemption rules",
		"SCEN-001":  "Redeem valid coupon",
	}}

	return &navFixture{
		handler: service.NewNavToolsHandler(
			ideaRepo, epicRepo, featRepo, storyRepo, specRepo, scenRepo,
			linkRepo, refRepo, resolver, "/db",
		),
		ideaRepo:  ideaRepo,
		epicRepo:  epicRepo,
		featRepo:  featRepo,
		storyRepo: storyRepo,
		specRepo:  specRepo,
		scenRepo:  scenRepo,
		linkRepo:  linkRepo,
		refRepo:   refRepo,
		resolver:  resolver,
	}
}

// --- get_lineage tests ---

func TestNavGetLineage_Idea(t *testing.T) {
	f := newNavFixture()
	got := f.handler.Handle(context.Background(), &driven.ToolUseContent{
		Name: driven.ToolGetLineage, Input: map[string]any{"id": "IDEA-001"},
	})
	if got != "IDEA-001" {
		t.Fatalf("got %q, want IDEA-001", got)
	}
}

func TestNavGetLineage_Epic(t *testing.T) {
	f := newNavFixture()
	got := f.handler.Handle(context.Background(), &driven.ToolUseContent{
		Name: driven.ToolGetLineage, Input: map[string]any{"id": "EPIC-001"},
	})
	if got != "IDEA-001 > EPIC-001" {
		t.Fatalf("got %q", got)
	}
}

func TestNavGetLineage_Feature(t *testing.T) {
	f := newNavFixture()
	got := f.handler.Handle(context.Background(), &driven.ToolUseContent{
		Name: driven.ToolGetLineage, Input: map[string]any{"id": "FEAT-001"},
	})
	if got != "IDEA-001 > EPIC-001 > FEAT-001" {
		t.Fatalf("got %q", got)
	}
}

func TestNavGetLineage_Story(t *testing.T) {
	f := newNavFixture()
	got := f.handler.Handle(context.Background(), &driven.ToolUseContent{
		Name: driven.ToolGetLineage, Input: map[string]any{"id": "STORY-001"},
	})
	if got != "IDEA-001 > EPIC-001 > FEAT-001 > STORY-001" {
		t.Fatalf("got %q", got)
	}
}

func TestNavGetLineage_Spec(t *testing.T) {
	f := newNavFixture()
	got := f.handler.Handle(context.Background(), &driven.ToolUseContent{
		Name: driven.ToolGetLineage, Input: map[string]any{"id": "SPEC-001"},
	})
	if got != "IDEA-001 > EPIC-001 > FEAT-001 > STORY-001 > SPEC-001" {
		t.Fatalf("got %q", got)
	}
}

func TestNavGetLineage_Scenario(t *testing.T) {
	f := newNavFixture()
	got := f.handler.Handle(context.Background(), &driven.ToolUseContent{
		Name: driven.ToolGetLineage, Input: map[string]any{"id": "SCEN-001"},
	})
	if got != "IDEA-001 > EPIC-001 > FEAT-001 > STORY-001 > SPEC-001 > SCEN-001" {
		t.Fatalf("got %q", got)
	}
}

func TestNavGetLineage_NotFound(t *testing.T) {
	f := newNavFixture()
	got := f.handler.Handle(context.Background(), &driven.ToolUseContent{
		Name: driven.ToolGetLineage, Input: map[string]any{"id": "EPIC-999"},
	})
	if !strings.HasPrefix(got, "error:") {
		t.Fatalf("expected error, got %q", got)
	}
}

func TestNavGetLineage_EmptyID(t *testing.T) {
	f := newNavFixture()
	got := f.handler.Handle(context.Background(), &driven.ToolUseContent{
		Name: driven.ToolGetLineage, Input: map[string]any{},
	})
	if !strings.HasPrefix(got, "error:") {
		t.Fatalf("expected error, got %q", got)
	}
}

// --- get_item_detail tests ---

func TestNavGetItemDetail_Idea(t *testing.T) {
	f := newNavFixture()
	got := f.handler.Handle(context.Background(), &driven.ToolUseContent{
		Name: driven.ToolGetItemDetail, Input: map[string]any{"id": "IDEA-001"},
	})
	if !strings.Contains(got, `Idea IDEA-001`) {
		t.Errorf("missing Idea header; got:\n%s", got)
	}
	if !strings.Contains(got, "Shoppers redeem coupons.") {
		t.Errorf("missing description; got:\n%s", got)
	}
}

func TestNavGetItemDetail_Epic(t *testing.T) {
	f := newNavFixture()
	got := f.handler.Handle(context.Background(), &driven.ToolUseContent{
		Name: driven.ToolGetItemDetail, Input: map[string]any{"id": "EPIC-001"},
	})
	if !strings.Contains(got, "Epic EPIC-001") {
		t.Errorf("missing header; got:\n%s", got)
	}
}

func TestNavGetItemDetail_Scenario(t *testing.T) {
	f := newNavFixture()
	got := f.handler.Handle(context.Background(), &driven.ToolUseContent{
		Name: driven.ToolGetItemDetail, Input: map[string]any{"id": "SCEN-001"},
	})
	if !strings.Contains(got, "Scenario SCEN-001") {
		t.Errorf("missing header; got:\n%s", got)
	}
	if !strings.Contains(got, "Given:") {
		t.Errorf("missing Given section; got:\n%s", got)
	}
	if !strings.Contains(got, "user has coupon SAVE10") {
		t.Errorf("missing Given step; got:\n%s", got)
	}
	if !strings.Contains(got, "Tags: happy-path") {
		t.Errorf("missing tags; got:\n%s", got)
	}
}

func TestNavGetItemDetail_WithRefs(t *testing.T) {
	f := newNavFixture()
	f.refRepo.refs = append(f.refRepo.refs, domain.Ref{
		EntityID: "EPIC-001", URL: "https://example.com/design", Label: "Design doc",
	})
	got := f.handler.Handle(context.Background(), &driven.ToolUseContent{
		Name: driven.ToolGetItemDetail, Input: map[string]any{"id": "EPIC-001"},
	})
	if !strings.Contains(got, "Design doc (https://example.com/design)") {
		t.Errorf("missing ref; got:\n%s", got)
	}
}

func TestNavGetItemDetail_NotFound(t *testing.T) {
	f := newNavFixture()
	got := f.handler.Handle(context.Background(), &driven.ToolUseContent{
		Name: driven.ToolGetItemDetail, Input: map[string]any{"id": "IDEA-999"},
	})
	if !strings.HasPrefix(got, "error:") {
		t.Fatalf("expected error, got %q", got)
	}
}

// --- get_siblings tests ---

func TestNavGetSiblings_IdeaHasNoSiblings(t *testing.T) {
	f := newNavFixture()
	got := f.handler.Handle(context.Background(), &driven.ToolUseContent{
		Name: driven.ToolGetSiblings, Input: map[string]any{"id": "IDEA-001"},
	})
	if got != "(ideas have no parent; no siblings)" {
		t.Fatalf("got %q", got)
	}
}

func TestNavGetSiblings_Epic(t *testing.T) {
	f := newNavFixture()
	got := f.handler.Handle(context.Background(), &driven.ToolUseContent{
		Name: driven.ToolGetSiblings, Input: map[string]any{"id": "EPIC-001"},
	})
	// EPIC-002 is a sibling under the same Idea.
	if !strings.Contains(got, "EPIC-002") {
		t.Errorf("missing sibling EPIC-002; got:\n%s", got)
	}
	if !strings.Contains(got, `"Promotions"`) {
		t.Errorf("missing sibling title; got:\n%s", got)
	}
	// Self should be excluded.
	if strings.Contains(got, "EPIC-001") {
		t.Errorf("self should be excluded; got:\n%s", got)
	}
}

func TestNavGetSiblings_OnlyChild(t *testing.T) {
	f := newNavFixture()
	got := f.handler.Handle(context.Background(), &driven.ToolUseContent{
		Name: driven.ToolGetSiblings, Input: map[string]any{"id": "FEAT-001"},
	})
	if got != "(no siblings)" {
		t.Fatalf("expected no siblings for only child; got %q", got)
	}
}

// --- trace_links tests ---

func TestNavTraceLinks_NoLinks(t *testing.T) {
	f := newNavFixture()
	got := f.handler.Handle(context.Background(), &driven.ToolUseContent{
		Name: driven.ToolTraceLinks, Input: map[string]any{"id": "STORY-001"},
	})
	if got != "(no links)" {
		t.Fatalf("got %q, want (no links)", got)
	}
}

func TestNavTraceLinks_WithLinks(t *testing.T) {
	f := newNavFixture()
	f.linkRepo.links = append(f.linkRepo.links, domain.Link{
		FromID: "STORY-001", ToID: "EPIC-001", Kind: domain.LinkRelatesTo,
	})
	got := f.handler.Handle(context.Background(), &driven.ToolUseContent{
		Name: driven.ToolTraceLinks, Input: map[string]any{"id": "STORY-001"},
	})
	if !strings.Contains(got, "relates-to") {
		t.Errorf("missing relation; got:\n%s", got)
	}
	if !strings.Contains(got, "EPIC-001") {
		t.Errorf("missing other ID; got:\n%s", got)
	}
}

func TestNavTraceLinks_InverseDirection(t *testing.T) {
	f := newNavFixture()
	f.linkRepo.links = append(f.linkRepo.links, domain.Link{
		FromID: "STORY-001", ToID: "EPIC-001", Kind: domain.LinkBlockedBy,
	})
	// Query from EPIC-001 perspective — should show inverse "blocks"
	got := f.handler.Handle(context.Background(), &driven.ToolUseContent{
		Name: driven.ToolTraceLinks, Input: map[string]any{"id": "EPIC-001"},
	})
	if !strings.Contains(got, "blocks") {
		t.Errorf("expected inverse label 'blocks'; got:\n%s", got)
	}
	if !strings.Contains(got, "STORY-001") {
		t.Errorf("expected other endpoint STORY-001; got:\n%s", got)
	}
}

// --- unknown tool ---

func TestNavHandle_UnknownTool(t *testing.T) {
	f := newNavFixture()
	got := f.handler.Handle(context.Background(), &driven.ToolUseContent{
		Name: "unknown_tool", Input: map[string]any{"id": "IDEA-001"},
	})
	if !strings.HasPrefix(got, "error:") {
		t.Fatalf("expected error, got %q", got)
	}
}
