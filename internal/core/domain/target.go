package domain

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

// Target is a supported code-generation and verification ecosystem for a
// d7 workspace. The set of valid Targets is fixed for v1: Go and
// TypeScript. Additional ecosystems are v2+ work and will be added by
// introducing new ScenarioRunner adapters, not by widening this type.
type Target string

const (
	// TargetGo is the Go + godog target.
	TargetGo Target = "go"
	// TargetTypeScript is the TypeScript + @cucumber/cucumber target.
	TargetTypeScript Target = "typescript"
)

// ErrUnknownTarget is returned by ParseTargets when an input value is
// not one of the supported targets.
var ErrUnknownTarget = errors.New("unknown target")

// allTargets enumerates the valid Target values in a stable order.
// Used for error messages and for the canonical sort applied by
// ParseTargets.
var allTargets = []Target{TargetGo, TargetTypeScript}

// knownTargets indexes the valid Target values for O(1) validation.
var knownTargets = map[Target]struct{}{
	TargetGo:         {},
	TargetTypeScript: {},
}

// ParseTargets converts a list of raw strings (typically from CLI flag
// input) into a deduplicated, canonically sorted slice of valid
// Targets. Inputs are case-insensitive and may be surrounded by
// whitespace. An empty or unknown value produces an error wrapping
// ErrUnknownTarget. ParseTargets is the sole constructor of Target
// values outside the package, so any []Target a service receives is
// valid by construction.
func ParseTargets(raw []string) ([]Target, error) {
	seen := make(map[Target]struct{}, len(raw))
	for _, r := range raw {
		norm := strings.ToLower(strings.TrimSpace(r))
		if norm == "" {
			return nil, fmt.Errorf("%w: empty target", ErrUnknownTarget)
		}
		t := Target(norm)
		if _, ok := knownTargets[t]; !ok {
			return nil, fmt.Errorf("%w %q (supported: %s)", ErrUnknownTarget, r, SupportedTargets())
		}
		seen[t] = struct{}{}
	}
	out := make([]Target, 0, len(seen))
	for t := range seen {
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out, nil
}

// SupportedTargets returns a comma-separated list of the supported
// target identifiers, suitable for user-facing error messages.
func SupportedTargets() string {
	s := make([]string, len(allTargets))
	for i, t := range allTargets {
		s[i] = string(t)
	}
	return strings.Join(s, ", ")
}
