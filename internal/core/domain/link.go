package domain

import (
	"fmt"
	"strings"
	"time"
)

// LinkKind is a typed edge between two entities in the workspace.
type LinkKind string

const (
	LinkBlockedBy  LinkKind = "blocked-by"
	LinkRelatesTo  LinkKind = "relates-to"
	LinkDuplicates LinkKind = "duplicates"
)

// knownLinkKinds indexes the valid LinkKind values for O(1) lookup.
var knownLinkKinds = map[LinkKind]bool{
	LinkBlockedBy:  true,
	LinkRelatesTo:  true,
	LinkDuplicates: true,
}

// ErrUnknownLinkKind is returned by ParseLinkKind for unrecognized values.
var ErrUnknownLinkKind = fmt.Errorf("unknown link kind; valid kinds: blocked-by, relates-to, duplicates")

// ParseLinkKind converts a raw string into a valid LinkKind.
func ParseLinkKind(raw string) (LinkKind, error) {
	norm := LinkKind(strings.ToLower(strings.TrimSpace(raw)))
	if !knownLinkKinds[norm] {
		return "", ErrUnknownLinkKind
	}
	return norm, nil
}

// IsSymmetric reports whether the link kind is semantically symmetric
// (A relates-to B ≡ B relates-to A). Asymmetric kinds like
// blocked-by have a computed inverse (blocks) when rendered.
func (k LinkKind) IsSymmetric() bool {
	return k == LinkRelatesTo || k == LinkDuplicates
}

// InverseLabel returns the human-readable label for the inverse
// direction of this link kind. For symmetric kinds, it returns the
// kind itself. For asymmetric kinds, it returns the inverse verb.
func (k LinkKind) InverseLabel() string {
	switch k {
	case LinkBlockedBy:
		return "blocks"
	case LinkRelatesTo:
		return "relates-to"
	case LinkDuplicates:
		return "duplicates"
	default:
		return string(k)
	}
}

// Link is a typed edge between two entities in the workspace.
// Links are stored one-way (from → to); the inverse is computed
// at read time.
type Link struct {
	FromID    string
	ToID      string
	Kind      LinkKind
	CreatedAt time.Time
}

// Ref is an external reference attached to a workspace entity.
// It points outside the workspace (URLs to Figma, RFCs, tickets,
// prior art).
type Ref struct {
	EntityID  string
	URL       string
	Label     string // optional display label
	CreatedAt time.Time
}

// knownEntityPrefixes maps ID prefixes to their entity type names.
// Used for validating that an entity ID has a recognized format.
var knownEntityPrefixes = map[string]string{
	IdeaIDPrefix:     "idea",
	EpicIDPrefix:     "epic",
	FeatureIDPrefix:  "feature",
	StoryIDPrefix:    "story",
	SpecIDPrefix:     "spec",
	ScenarioIDPrefix: "scenario",
}

// ParseEntityPrefix extracts and validates the entity prefix from an
// ID string. Returns the prefix (e.g. "IDEA") and entity type name
// (e.g. "idea"), or an error if the ID format is not recognized.
func ParseEntityPrefix(id string) (prefix string, typeName string, err error) {
	idx := strings.Index(id, "-")
	if idx < 1 {
		return "", "", fmt.Errorf("invalid entity ID format: %q", id)
	}
	prefix = id[:idx]
	typeName, ok := knownEntityPrefixes[prefix]
	if !ok {
		return "", "", fmt.Errorf("unknown entity prefix %q in ID %q", prefix, id)
	}
	return prefix, typeName, nil
}
