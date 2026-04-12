package domain_test

import (
	"strings"
	"testing"

	"github.com/c64-io/daedalus/internal/core/domain"
)

func TestFormatDialogContext_MinimalIdea(t *testing.T) {
	c := domain.DialogContext{
		Workspace: "A coupon system for small online shops.",
		Target: domain.DialogContextEntity{
			Kind:        "Idea",
			ID:          "IDEA-001",
			Title:       "Coupon system",
			Status:      "draft",
			Description: "We want shoppers to be able to redeem coupons at checkout.",
		},
	}

	got := domain.FormatDialogContext(c)

	mustContain(t, got, "# Product")
	mustContain(t, got, "A coupon system for small online shops.")
	mustContain(t, got, "# What we're working on")
	mustContain(t, got, `Idea IDEA-001 — "Coupon system"  (draft)`)
	mustContain(t, got, "We want shoppers to be able to redeem coupons at checkout.")

	if strings.Contains(got, "# Context from the parent tree") {
		t.Fatalf("no ancestors should not render an ancestor section; got:\n%s", got)
	}
	if strings.Contains(got, "# Related work") {
		t.Fatalf("no links should not render a links section; got:\n%s", got)
	}
	if strings.Contains(got, "# External references") {
		t.Fatalf("no refs should not render a refs section; got:\n%s", got)
	}
}

func TestFormatDialogContext_StoryWithFullAncestorChain(t *testing.T) {
	c := domain.DialogContext{
		Workspace: "A coupon system for small online shops.",
		Ancestors: []domain.DialogContextEntity{
			{Kind: "Idea", ID: "IDEA-001", Title: "Coupons", Status: "refined", Description: "Shoppers redeem coupons."},
			{Kind: "Epic", ID: "EPIC-003", Title: "Redemption", Status: "refined", Description: "The whole redemption flow.", Priority: "high", Size: "8"},
			{Kind: "Feature", ID: "FEAT-007", Title: "Checkout redemption", Status: "refined", Description: "Redeem at checkout."},
		},
		Target: domain.DialogContextEntity{
			Kind:        "Story",
			ID:          "STORY-042",
			Title:       "User redeems a valid coupon",
			Status:      "draft",
			Description: "Rough wording: user enters code, gets discount.",
			Priority:    "high",
			Size:        "5",
		},
		Links: []domain.DialogContextLink{
			{Relation: "blocked by", OtherID: "STORY-050", OtherTitle: "Logged-in user"},
			{Relation: "relates to", OtherID: "SPEC-012", OtherTitle: "Pricing rules"},
		},
		Refs: []domain.DialogContextRef{
			{URL: "https://figma.com/coupons", Label: "Checkout mockup"},
			{URL: "https://example.com/notes"},
		},
	}

	got := domain.FormatDialogContext(c)

	mustContain(t, got, "# Context from the parent tree")
	mustContain(t, got, `Idea IDEA-001 — "Coupons"`)
	mustContain(t, got, `Epic EPIC-003 — "Redemption"`)
	mustContain(t, got, "priority: high  size: 8")
	mustContain(t, got, `Feature FEAT-007 — "Checkout redemption"`)

	mustContain(t, got, "# What we're working on")
	mustContain(t, got, `Story STORY-042 — "User redeems a valid coupon"  (draft)`)
	mustContain(t, got, "priority: high  size: 5")
	mustContain(t, got, "Rough wording: user enters code, gets discount.")

	mustContain(t, got, "# Related work")
	mustContain(t, got, "- blocked by STORY-050 — Logged-in user")
	mustContain(t, got, "- relates to SPEC-012 — Pricing rules")

	mustContain(t, got, "# External references")
	mustContain(t, got, "- Checkout mockup (https://figma.com/coupons)")
	mustContain(t, got, "- https://example.com/notes")
}

func TestFormatDialogContext_Scenario(t *testing.T) {
	c := domain.DialogContext{
		Workspace: "Coupons.",
		Target: domain.DialogContextEntity{
			Kind:   "Scenario",
			ID:     "SCEN-001",
			Title:  "Redeem a valid coupon",
			Status: "draft",
			Tags:   []string{"happy-path", "checkout"},
			Given:  []string{"a user has coupon SAVE10", "the coupon has not been redeemed"},
			When:   []string{"the user applies the coupon at checkout"},
			Then:   []string{"the total is reduced by 10%", "the coupon is marked redeemed"},
		},
	}

	got := domain.FormatDialogContext(c)

	mustContain(t, got, "Tags: happy-path, checkout")
	mustContain(t, got, "Given:")
	mustContain(t, got, "- a user has coupon SAVE10")
	mustContain(t, got, "When:")
	mustContain(t, got, "- the user applies the coupon at checkout")
	mustContain(t, got, "Then:")
	mustContain(t, got, "- the total is reduced by 10%")
}

func TestFormatDialogContext_EmptyWorkspace(t *testing.T) {
	c := domain.DialogContext{
		Target: domain.DialogContextEntity{Kind: "Idea", ID: "IDEA-001", Title: "x", Status: "draft"},
	}
	got := domain.FormatDialogContext(c)
	mustContain(t, got, "(no project description yet)")
}

func TestFormatDialogContext_IsDeterministic(t *testing.T) {
	c := domain.DialogContext{
		Workspace: "x",
		Target:    domain.DialogContextEntity{Kind: "Idea", ID: "IDEA-001", Title: "y", Status: "draft", Description: "z"},
	}
	a := domain.FormatDialogContext(c)
	b := domain.FormatDialogContext(c)
	if a != b {
		t.Fatalf("FormatDialogContext is not deterministic;\nfirst:\n%s\nsecond:\n%s", a, b)
	}
}

func TestFormatDialogContext_MultilineDescriptionIndented(t *testing.T) {
	c := domain.DialogContext{
		Target: domain.DialogContextEntity{
			Kind: "Story", ID: "STORY-001", Title: "x", Status: "draft",
			Description: "first line\nsecond line\nthird line",
		},
	}
	got := domain.FormatDialogContext(c)
	mustContain(t, got, "    first line\n    second line\n    third line")
}

func mustContain(t *testing.T, s, sub string) {
	t.Helper()
	if !strings.Contains(s, sub) {
		t.Fatalf("expected output to contain %q but got:\n%s", sub, s)
	}
}
