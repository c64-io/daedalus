package domain_test

import (
	"testing"

	"github.com/c64-io/daedalus/internal/core/domain"
)

func TestParseLinkKind(t *testing.T) {
	tests := []struct {
		input string
		want  domain.LinkKind
		err   bool
	}{
		{"blocked-by", domain.LinkBlockedBy, false},
		{"relates-to", domain.LinkRelatesTo, false},
		{"duplicates", domain.LinkDuplicates, false},
		{"BLOCKED-BY", domain.LinkBlockedBy, false},
		{" relates-to ", domain.LinkRelatesTo, false},
		{"unknown", "", true},
		{"", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := domain.ParseLinkKind(tt.input)
			if tt.err && err == nil {
				t.Fatalf("expected error for input %q", tt.input)
			}
			if !tt.err && err != nil {
				t.Fatalf("unexpected error for input %q: %v", tt.input, err)
			}
			if got != tt.want {
				t.Errorf("ParseLinkKind(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestLinkKindSymmetry(t *testing.T) {
	if domain.LinkBlockedBy.IsSymmetric() {
		t.Error("blocked-by should not be symmetric")
	}
	if !domain.LinkRelatesTo.IsSymmetric() {
		t.Error("relates-to should be symmetric")
	}
	if !domain.LinkDuplicates.IsSymmetric() {
		t.Error("duplicates should be symmetric")
	}
}

func TestLinkKindInverseLabel(t *testing.T) {
	if got := domain.LinkBlockedBy.InverseLabel(); got != "blocks" {
		t.Errorf("blocked-by inverse = %q, want %q", got, "blocks")
	}
	if got := domain.LinkRelatesTo.InverseLabel(); got != "relates-to" {
		t.Errorf("relates-to inverse = %q, want %q", got, "relates-to")
	}
	if got := domain.LinkDuplicates.InverseLabel(); got != "duplicates" {
		t.Errorf("duplicates inverse = %q, want %q", got, "duplicates")
	}
}

func TestParseEntityPrefix(t *testing.T) {
	tests := []struct {
		id       string
		prefix   string
		typeName string
		err      bool
	}{
		{"IDEA-001", "IDEA", "idea", false},
		{"EPIC-012", "EPIC", "epic", false},
		{"FEAT-003", "FEAT", "feature", false},
		{"STORY-047", "STORY", "story", false},
		{"SPEC-001", "SPEC", "spec", false},
		{"SCEN-114", "SCEN", "scenario", false},
		{"UNKNOWN-001", "", "", true},
		{"nohyphen", "", "", true},
		{"", "", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			prefix, typeName, err := domain.ParseEntityPrefix(tt.id)
			if tt.err && err == nil {
				t.Fatalf("expected error for ID %q", tt.id)
			}
			if !tt.err && err != nil {
				t.Fatalf("unexpected error for ID %q: %v", tt.id, err)
			}
			if prefix != tt.prefix {
				t.Errorf("prefix = %q, want %q", prefix, tt.prefix)
			}
			if typeName != tt.typeName {
				t.Errorf("typeName = %q, want %q", typeName, tt.typeName)
			}
		})
	}
}
