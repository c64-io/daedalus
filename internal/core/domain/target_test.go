package domain_test

import (
	"errors"
	"testing"

	"github.com/c64-io/daedalus/internal/core/domain"
)

func TestParseTarget(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want domain.Target
	}{
		{name: "go", in: "go", want: domain.TargetGo},
		{name: "typescript", in: "typescript", want: domain.TargetTypeScript},
		{name: "uppercase go", in: "GO", want: domain.TargetGo},
		{name: "mixed case typescript", in: "TypeScript", want: domain.TargetTypeScript},
		{name: "trimmed whitespace", in: "  go  ", want: domain.TargetGo},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := domain.ParseTarget(tc.in)
			if err != nil {
				t.Fatalf("ParseTarget(%q) returned unexpected error: %v", tc.in, err)
			}
			if got != tc.want {
				t.Fatalf("ParseTarget(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestParseTarget_UnknownTarget(t *testing.T) {
	t.Parallel()

	tests := []string{"", "   ", "rust", "python", "js"}
	for _, in := range tests {
		in := in
		t.Run(in, func(t *testing.T) {
			t.Parallel()
			_, err := domain.ParseTarget(in)
			if err == nil {
				t.Fatalf("ParseTarget(%q) unexpectedly succeeded", in)
			}
			if !errors.Is(err, domain.ErrUnknownTarget) {
				t.Fatalf("ParseTarget(%q) error = %v, want wrapping ErrUnknownTarget", in, err)
			}
		})
	}
}

func TestParseTarget_MultipleTargetsRejected(t *testing.T) {
	t.Parallel()

	tests := []string{"go,typescript", "go,go", "typescript,go"}
	for _, in := range tests {
		in := in
		t.Run(in, func(t *testing.T) {
			t.Parallel()
			_, err := domain.ParseTarget(in)
			if err == nil {
				t.Fatalf("ParseTarget(%q) unexpectedly succeeded", in)
			}
			if !errors.Is(err, domain.ErrMultipleTargets) {
				t.Fatalf("ParseTarget(%q) error = %v, want wrapping ErrMultipleTargets", in, err)
			}
		})
	}
}
