package domain

import (
	"errors"
	"fmt"
	"strings"
)

// Target is the code-generation and verification ecosystem a d7
// workspace produces and checks against. In v1 a workspace has
// exactly one Target, chosen at `d7 init` and immutable thereafter.
// The set of valid Targets is fixed: Go and TypeScript. Additional
// ecosystems are v2+ work and will be added by introducing new
// ScenarioRunner adapters, not by widening this type.
type Target string

const (
	// TargetGo is the Go + godog target.
	TargetGo Target = "go"
	// TargetTypeScript is the TypeScript + @cucumber/cucumber target.
	TargetTypeScript Target = "typescript"
)

// ErrUnknownTarget is returned by ParseTarget when the input value is
// not one of the supported targets.
var ErrUnknownTarget = errors.New("unknown target")

// ErrMultipleTargets is returned by ParseTarget when the raw input
// looks like a list (contains a comma). v1 workspaces are
// single-target; monorepo / multi-target support is a v2 ambition.
var ErrMultipleTargets = errors.New("only one target is supported per workspace")

// allTargets enumerates the valid Target values in a stable order.
// Used for error messages.
var allTargets = []Target{TargetGo, TargetTypeScript}

// knownTargets indexes the valid Target values for O(1) validation.
var knownTargets = map[Target]struct{}{
	TargetGo:         {},
	TargetTypeScript: {},
}

// ParseTarget converts a single raw string (typically from CLI flag
// input) into a valid Target. The input is case-insensitive and may
// be surrounded by whitespace. Empty input, values containing a
// comma, and unrecognized values all produce errors. ParseTarget is
// the sole constructor of Target values outside the package, so any
// Target a service receives is valid by construction.
func ParseTarget(raw string) (Target, error) {
	norm := strings.ToLower(strings.TrimSpace(raw))
	if norm == "" {
		return "", fmt.Errorf("%w: empty target", ErrUnknownTarget)
	}
	if strings.Contains(norm, ",") {
		return "", fmt.Errorf("%w: got %q", ErrMultipleTargets, raw)
	}
	t := Target(norm)
	if _, ok := knownTargets[t]; !ok {
		return "", fmt.Errorf("%w %q (supported: %s)", ErrUnknownTarget, raw, SupportedTargets())
	}
	return t, nil
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
