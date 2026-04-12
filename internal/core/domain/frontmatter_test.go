package domain_test

import (
	"errors"
	"testing"

	"github.com/c64-io/daedalus/internal/core/domain"
)

func TestFormatFrontMatter_Basic(t *testing.T) {
	t.Parallel()

	fields := []domain.FrontMatterField{
		{Key: "id", Value: "IDEA-001", ReadOnly: true},
		{Key: "status", Value: "refined"},
		{Key: "title", Value: "My SaaS"},
	}

	got := domain.FormatFrontMatter(fields, "Description here.\n")
	want := "---\n" +
		"id: IDEA-001  # read-only\n" +
		"status: refined\n" +
		"title: My SaaS\n" +
		"---\n" +
		"\n" +
		"Description here.\n"

	if got != want {
		t.Errorf("FormatFrontMatter =\n%q\nwant\n%q", got, want)
	}
}

func TestFormatFrontMatter_EmptyBody(t *testing.T) {
	t.Parallel()

	fields := []domain.FrontMatterField{
		{Key: "target", Value: "go", ReadOnly: true},
	}

	got := domain.FormatFrontMatter(fields, "")
	want := "---\ntarget: go  # read-only\n---\n"

	if got != want {
		t.Errorf("FormatFrontMatter =\n%q\nwant\n%q", got, want)
	}
}

func TestParseFrontMatter_RoundTrip(t *testing.T) {
	t.Parallel()

	fields := []domain.FrontMatterField{
		{Key: "id", Value: "EPIC-001", ReadOnly: true},
		{Key: "status", Value: "draft"},
		{Key: "title", Value: "Billing System"},
		{Key: "priority", Value: "high"},
		{Key: "size", Value: "8"},
	}
	body := "Epic description in markdown.\n"

	content := domain.FormatFrontMatter(fields, body)
	gotFields, gotBody, err := domain.ParseFrontMatter(content)
	if err != nil {
		t.Fatalf("ParseFrontMatter returned error: %v", err)
	}

	if gotBody != body {
		t.Errorf("body = %q, want %q", gotBody, body)
	}

	wantMap := map[string]string{
		"id":       "EPIC-001",
		"status":   "draft",
		"title":    "Billing System",
		"priority": "high",
		"size":     "8",
	}
	for k, v := range wantMap {
		if gotFields[k] != v {
			t.Errorf("field[%q] = %q, want %q", k, gotFields[k], v)
		}
	}
}

func TestParseFrontMatter_MissingSeparator(t *testing.T) {
	t.Parallel()

	_, _, err := domain.ParseFrontMatter("no separator here")
	if !errors.Is(err, domain.ErrMalformedFrontMatter) {
		t.Fatalf("err = %v, want ErrMalformedFrontMatter", err)
	}
}

func TestParseFrontMatter_MissingClosing(t *testing.T) {
	t.Parallel()

	_, _, err := domain.ParseFrontMatter("---\ntitle: foo\n")
	if !errors.Is(err, domain.ErrMalformedFrontMatter) {
		t.Fatalf("err = %v, want ErrMalformedFrontMatter", err)
	}
}

func TestParseFrontMatter_MissingColon(t *testing.T) {
	t.Parallel()

	_, _, err := domain.ParseFrontMatter("---\nbadline\n---\n")
	if !errors.Is(err, domain.ErrMalformedFrontMatter) {
		t.Fatalf("err = %v, want ErrMalformedFrontMatter", err)
	}
}

func TestParseFrontMatter_EmptyBody(t *testing.T) {
	t.Parallel()

	fields, body, err := domain.ParseFrontMatter("---\ntarget: go  # read-only\n---\n")
	if err != nil {
		t.Fatalf("ParseFrontMatter returned error: %v", err)
	}
	if body != "" {
		t.Errorf("body = %q, want empty", body)
	}
	if fields["target"] != "go" {
		t.Errorf("target = %q, want %q", fields["target"], "go")
	}
}
