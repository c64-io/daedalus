package domain_test

import (
	"errors"
	"testing"

	"github.com/c64-io/daedalus/internal/core/domain"
)

func TestStatus_ValidTransitions(t *testing.T) {
	t.Parallel()

	valid := []struct{ from, to domain.Status }{
		{domain.StatusDraft, domain.StatusRefined},
		{domain.StatusRefined, domain.StatusReady},
		{domain.StatusReady, domain.StatusInProgress},
		{domain.StatusInProgress, domain.StatusReview},
		{domain.StatusReview, domain.StatusDone},
		{domain.StatusDone, domain.StatusArchived},
		// backward transitions
		{domain.StatusRefined, domain.StatusDraft},
		{domain.StatusReady, domain.StatusRefined},
		{domain.StatusInProgress, domain.StatusReady},
		{domain.StatusReview, domain.StatusInProgress},
		// archived can return to draft
		{domain.StatusArchived, domain.StatusDraft},
		// anything (except done) can archive
		{domain.StatusDraft, domain.StatusArchived},
		{domain.StatusRefined, domain.StatusArchived},
		{domain.StatusReady, domain.StatusArchived},
		{domain.StatusInProgress, domain.StatusArchived},
		{domain.StatusReview, domain.StatusArchived},
		// blocked transitions
		{domain.StatusDraft, domain.StatusBlocked},
		{domain.StatusRefined, domain.StatusBlocked},
		{domain.StatusReady, domain.StatusBlocked},
		{domain.StatusInProgress, domain.StatusBlocked},
		{domain.StatusReview, domain.StatusBlocked},
		{domain.StatusBlocked, domain.StatusDraft},
		{domain.StatusBlocked, domain.StatusRefined},
		{domain.StatusBlocked, domain.StatusReady},
		{domain.StatusBlocked, domain.StatusInProgress},
		{domain.StatusBlocked, domain.StatusReview},
		{domain.StatusBlocked, domain.StatusArchived},
	}

	for _, tc := range valid {
		tc := tc
		t.Run(string(tc.from)+"→"+string(tc.to), func(t *testing.T) {
			t.Parallel()
			if !tc.from.CanTransition(tc.to) {
				t.Errorf("expected %s → %s to be valid", tc.from, tc.to)
			}
			got, err := tc.from.Transition(tc.to)
			if err != nil {
				t.Errorf("Transition(%s → %s) returned unexpected error: %v", tc.from, tc.to, err)
			}
			if got != tc.to {
				t.Errorf("Transition(%s → %s) = %s, want %s", tc.from, tc.to, got, tc.to)
			}
		})
	}
}

func TestStatus_InvalidTransitions(t *testing.T) {
	t.Parallel()

	invalid := []struct{ from, to domain.Status }{
		{domain.StatusDraft, domain.StatusDone},
		{domain.StatusDraft, domain.StatusInProgress},
		{domain.StatusDone, domain.StatusDraft},
		{domain.StatusDone, domain.StatusReview},
		{domain.StatusArchived, domain.StatusRefined},
		{domain.StatusDone, domain.StatusBlocked},
		{domain.StatusBlocked, domain.StatusDone},
	}

	for _, tc := range invalid {
		tc := tc
		t.Run(string(tc.from)+"→"+string(tc.to), func(t *testing.T) {
			t.Parallel()
			if tc.from.CanTransition(tc.to) {
				t.Errorf("expected %s → %s to be invalid", tc.from, tc.to)
			}
			_, err := tc.from.Transition(tc.to)
			if !errors.Is(err, domain.ErrInvalidTransition) {
				t.Errorf("Transition(%s → %s) err = %v, want ErrInvalidTransition", tc.from, tc.to, err)
			}
		})
	}
}

func TestParseStatus(t *testing.T) {
	t.Parallel()

	got, err := domain.ParseStatus("  In-Progress  ")
	if err != nil {
		t.Fatalf("ParseStatus returned unexpected error: %v", err)
	}
	if got != domain.StatusInProgress {
		t.Fatalf("ParseStatus = %q, want %q", got, domain.StatusInProgress)
	}

	_, err = domain.ParseStatus("invalid")
	if !errors.Is(err, domain.ErrUnknownStatus) {
		t.Fatalf("ParseStatus(invalid) err = %v, want ErrUnknownStatus", err)
	}
}
