package domain_test

import (
	"testing"

	"github.com/c64-io/daedalus/internal/core/domain"
)

func TestFormatScenarioID(t *testing.T) {
	t.Parallel()

	cases := []struct {
		seq  int
		want string
	}{
		{1, "SCEN-001"},
		{42, "SCEN-042"},
		{999, "SCEN-999"},
	}
	for _, c := range cases {
		if got := domain.FormatScenarioID(c.seq); got != c.want {
			t.Errorf("FormatScenarioID(%d) = %q, want %q", c.seq, got, c.want)
		}
	}
}

func TestScenarioIDPrefix(t *testing.T) {
	t.Parallel()

	if domain.ScenarioIDPrefix != "SCEN" {
		t.Errorf("ScenarioIDPrefix = %q, want %q", domain.ScenarioIDPrefix, "SCEN")
	}
}
