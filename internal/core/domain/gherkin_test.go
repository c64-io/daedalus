package domain_test

import (
	"strings"
	"testing"

	"github.com/c64-io/daedalus/internal/core/domain"
)

func TestFormatScenarioAsGherkin_Basic(t *testing.T) {
	t.Parallel()

	s := domain.Scenario{
		ID:     "SCEN-001",
		SpecID: "SPEC-001",
		Title:  "User redeems a valid coupon",
		Given: []domain.Step{
			{Text: `a user has a valid coupon "SAVE10"`},
			{Text: "the coupon has not been redeemed"},
		},
		When: []domain.Step{
			{Text: "the user applies the coupon at checkout"},
		},
		Then: []domain.Step{
			{Text: "the order total is reduced by 10%"},
			{Text: "the coupon is marked as redeemed"},
		},
	}

	got := domain.FormatScenarioAsGherkin(s)
	want := `Scenario: User redeems a valid coupon
  Given a user has a valid coupon "SAVE10"
  And the coupon has not been redeemed
  When the user applies the coupon at checkout
  Then the order total is reduced by 10%
  And the coupon is marked as redeemed
`
	if got != want {
		t.Errorf("gherkin mismatch\ngot:\n%s\nwant:\n%s", got, want)
	}
}

func TestFormatScenarioAsGherkin_WithTags(t *testing.T) {
	t.Parallel()

	s := domain.Scenario{
		ID:    "SCEN-002",
		Title: "Expired coupon is rejected",
		Tags:  []string{"happy-path", "checkout"},
		Given: []domain.Step{{Text: "an expired coupon"}},
		When:  []domain.Step{{Text: "the user applies it"}},
		Then:  []domain.Step{{Text: "the system rejects it"}},
	}

	got := domain.FormatScenarioAsGherkin(s)
	if !strings.HasPrefix(got, "@happy-path @checkout\n") {
		t.Errorf("tag line missing or wrong: %q", got)
	}
	if !strings.Contains(got, "Scenario: Expired coupon is rejected") {
		t.Errorf("title line missing: %q", got)
	}
}

func TestFormatScenarioAsGherkin_OmitsD7Tag(t *testing.T) {
	t.Parallel()

	// Storage must not include @d7:<ID> — it is added at export.
	s := domain.Scenario{
		ID:    "SCEN-003",
		Title: "test",
		Tags:  []string{"smoke"},
		Given: []domain.Step{{Text: "x"}},
		When:  []domain.Step{{Text: "y"}},
		Then:  []domain.Step{{Text: "z"}},
	}
	got := domain.FormatScenarioAsGherkin(s)
	if strings.Contains(got, "@d7:") {
		t.Errorf("output must not contain @d7: tag, got: %s", got)
	}
}

func TestFormatScenarioAsGherkin_DocString(t *testing.T) {
	t.Parallel()

	payload := `{ "status": "ok" }`
	s := domain.Scenario{
		ID:    "SCEN-004",
		Title: "API responds",
		Given: []domain.Step{
			{
				Text:      "the API responds with:",
				DocString: &payload,
			},
		},
		When: []domain.Step{{Text: "the client reads it"}},
		Then: []domain.Step{{Text: "the client parses it"}},
	}

	got := domain.FormatScenarioAsGherkin(s)
	if !strings.Contains(got, "    \"\"\"\n") {
		t.Errorf("doc string delimiters missing: %s", got)
	}
	if !strings.Contains(got, `    { "status": "ok" }`) {
		t.Errorf("doc string payload missing: %s", got)
	}
}

func TestFormatScenarioAsGherkin_DataTable(t *testing.T) {
	t.Parallel()

	s := domain.Scenario{
		ID:    "SCEN-005",
		Title: "Users exist",
		Given: []domain.Step{
			{
				Text: "the following users exist:",
				DataTable: &domain.DataTable{
					Headers: []string{"name", "email"},
					Rows: [][]string{
						{"alice", "alice@example.com"},
						{"bob", "bob@example.com"},
					},
				},
			},
		},
		When: []domain.Step{{Text: "the admin lists them"}},
		Then: []domain.Step{{Text: "both appear"}},
	}

	got := domain.FormatScenarioAsGherkin(s)
	if !strings.Contains(got, "| name  | email             |") {
		t.Errorf("data table header missing or misaligned:\n%s", got)
	}
	if !strings.Contains(got, "| alice | alice@example.com |") {
		t.Errorf("data table row missing:\n%s", got)
	}
}
