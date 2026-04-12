package domain_test

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/c64-io/daedalus/internal/core/domain"
)

func TestFormatScenarioYAML_ReadOnlyCommentHeader(t *testing.T) {
	t.Parallel()

	created, _ := time.Parse("2006-01-02", "2026-04-11")
	s := domain.Scenario{
		ID:        "SCEN-001",
		SpecID:    "SPEC-001",
		Title:     "test",
		Status:    domain.StatusDraft,
		CreatedAt: created,
	}

	got, err := domain.FormatScenarioYAML(s)
	if err != nil {
		t.Fatalf("FormatScenarioYAML returned error: %v", err)
	}
	if !strings.Contains(got, "# id: SCEN-001") {
		t.Errorf("id comment missing:\n%s", got)
	}
	if !strings.Contains(got, "# spec: SPEC-001") {
		t.Errorf("spec comment missing:\n%s", got)
	}
	if !strings.Contains(got, "# created: 2026-04-11") {
		t.Errorf("created comment missing:\n%s", got)
	}
}

func TestScenarioYAML_RoundTrip(t *testing.T) {
	t.Parallel()

	payload := `{"status":"ok"}`
	s := domain.Scenario{
		ID:     "SCEN-007",
		SpecID: "SPEC-003",
		Title:  "User redeems a coupon",
		Tags:   []string{"happy-path", "checkout"},
		Status: domain.StatusRefined,
		Given: []domain.Step{
			{Text: `a user has a valid coupon "SAVE10"`},
			{
				Text: "the following users exist:",
				DataTable: &domain.DataTable{
					Headers: []string{"name", "email"},
					Rows: [][]string{
						{"alice", "alice@example.com"},
					},
				},
			},
		},
		When: []domain.Step{
			{Text: "the user applies the coupon"},
			{
				Text:      "the API replies with:",
				DocString: &payload,
			},
		},
		Then: []domain.Step{
			{Text: "the order total is reduced by 10%"},
		},
		CreatedAt: time.Now(),
	}

	text, err := domain.FormatScenarioYAML(s)
	if err != nil {
		t.Fatalf("FormatScenarioYAML returned error: %v", err)
	}

	parsed, err := domain.ParseScenarioYAML(text)
	if err != nil {
		t.Fatalf("ParseScenarioYAML returned error: %v", err)
	}

	if parsed.Status != string(s.Status) {
		t.Errorf("Status = %q, want %q", parsed.Status, s.Status)
	}
	if parsed.Title != s.Title {
		t.Errorf("Title = %q, want %q", parsed.Title, s.Title)
	}
	if !reflect.DeepEqual(parsed.Tags, s.Tags) {
		t.Errorf("Tags = %#v, want %#v", parsed.Tags, s.Tags)
	}
	if !reflect.DeepEqual(parsed.Given, s.Given) {
		t.Errorf("Given mismatch:\ngot  %#v\nwant %#v", parsed.Given, s.Given)
	}
	if !reflect.DeepEqual(parsed.When, s.When) {
		t.Errorf("When mismatch:\ngot  %#v\nwant %#v", parsed.When, s.When)
	}
	if !reflect.DeepEqual(parsed.Then, s.Then) {
		t.Errorf("Then mismatch:\ngot  %#v\nwant %#v", parsed.Then, s.Then)
	}
}

func TestParseScenarioYAML_StripsLeadingAtFromTags(t *testing.T) {
	t.Parallel()

	content := `status: draft
title: test
tags:
  - "@happy-path"
  - checkout
`
	parsed, err := domain.ParseScenarioYAML(content)
	if err != nil {
		t.Fatalf("ParseScenarioYAML returned error: %v", err)
	}
	want := []string{"happy-path", "checkout"}
	if !reflect.DeepEqual(parsed.Tags, want) {
		t.Errorf("Tags = %#v, want %#v", parsed.Tags, want)
	}
}

func TestParseScenarioYAML_Malformed(t *testing.T) {
	t.Parallel()

	_, err := domain.ParseScenarioYAML("status: [not, a, scalar\n")
	if err == nil {
		t.Fatal("ParseScenarioYAML should reject malformed YAML")
	}
}
