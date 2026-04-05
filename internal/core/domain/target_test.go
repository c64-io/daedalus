package domain_test

import (
	"errors"
	"reflect"
	"testing"

	"github.com/c64-io/daedalus/internal/core/domain"
)

func TestParseTargets(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   []string
		want []domain.Target
	}{
		{
			name: "single go",
			in:   []string{"go"},
			want: []domain.Target{domain.TargetGo},
		},
		{
			name: "single typescript",
			in:   []string{"typescript"},
			want: []domain.Target{domain.TargetTypeScript},
		},
		{
			name: "both targets canonicalized",
			in:   []string{"typescript", "go"},
			want: []domain.Target{domain.TargetGo, domain.TargetTypeScript},
		},
		{
			name: "deduplicates repeats",
			in:   []string{"go", "go", "typescript", "go"},
			want: []domain.Target{domain.TargetGo, domain.TargetTypeScript},
		},
		{
			name: "case insensitive with whitespace",
			in:   []string{" GO ", "TypeScript"},
			want: []domain.Target{domain.TargetGo, domain.TargetTypeScript},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := domain.ParseTargets(tc.in)
			if err != nil {
				t.Fatalf("ParseTargets(%v) returned unexpected error: %v", tc.in, err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("ParseTargets(%v) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

func TestParseTargets_Errors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   []string
	}{
		{name: "empty string", in: []string{""}},
		{name: "whitespace only", in: []string{"   "}},
		{name: "unknown target", in: []string{"rust"}},
		{name: "mixed known and unknown", in: []string{"go", "python"}},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := domain.ParseTargets(tc.in)
			if err == nil {
				t.Fatalf("ParseTargets(%v) unexpectedly succeeded", tc.in)
			}
			if !errors.Is(err, domain.ErrUnknownTarget) {
				t.Fatalf("ParseTargets(%v) error = %v, want wrapping ErrUnknownTarget", tc.in, err)
			}
		})
	}
}
