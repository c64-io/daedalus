package domain

import (
	"fmt"
	"strings"
)

// DialogContext is a neutral, renderable snapshot of everything the
// LLM needs to see about a target entity in an AI-assisted command:
// the project description, the ancestor chain up to the target, the
// target itself, and any cross-links or external refs that touch the
// target.
//
// Services build a DialogContext from typed domain objects; the
// FormatDialogContext function below turns it into a stable text
// block that is sent (and prompt-cached) on every turn of a dialog.
//
// All fields are renderable as-is. The builder does no loading and
// no formatting decisions beyond text layout — it is pure.
type DialogContext struct {
	Workspace string // project description body, already trimmed

	// Ancestors is ordered from root down to the entity immediately
	// above Target (e.g. [Idea, Epic, Feature] for a Story). Empty
	// for Ideas.
	Ancestors []DialogContextEntity

	// Target is the entity the command is operating on.
	Target DialogContextEntity

	Links []DialogContextLink
	Refs  []DialogContextRef
}

// DialogContextEntity is a neutral description of one entity in the
// context block. Service adapters convert typed domain objects into
// this shape.
type DialogContextEntity struct {
	Kind        string // "Idea", "Epic", "Feature", "Story", "Spec", "Scenario"
	ID          string
	Title       string
	Status      string
	Description string

	// Priority and Size are optional. Empty string means "not set"
	// or "not applicable for this entity type".
	Priority string
	Size     string

	// Scenario-specific content. Non-nil only for Kind == "Scenario".
	Tags  []string
	Given []string
	When  []string
	Then  []string
}

// DialogContextLink describes one cross-link that touches the target,
// already direction-normalized ("blocked by", "blocks", "relates to",
// "duplicates") and with the other endpoint's title resolved.
type DialogContextLink struct {
	Relation  string // "blocked by" | "blocks" | "relates to" | "duplicates"
	OtherID   string
	OtherTitle string
}

// DialogContextRef describes one external reference on the target.
type DialogContextRef struct {
	URL   string
	Label string // optional
}

// FormatDialogContext renders a DialogContext as the stable text
// block handed to the LLM as cached context. The output is
// deterministic for a given input — same struct in, same bytes out.
//
// The hierarchy section uses nested markdown lists so the parent-child
// chain is structurally visible. Each node is labeled [context] (read-
// only ancestors) or [target] (the entity the command operates on).
func FormatDialogContext(c DialogContext) string {
	var b strings.Builder

	writeContextSection(&b, "Product", func() {
		if c.Workspace == "" {
			b.WriteString("(no project description yet)\n")
			return
		}
		b.WriteString(strings.TrimSpace(c.Workspace))
		b.WriteString("\n")
	})

	writeContextSection(&b, "Hierarchy", func() {
		depth := 0
		for _, a := range c.Ancestors {
			writeNestedEntity(&b, depth, "context", a)
			depth++
		}
		writeNestedEntity(&b, depth, "target", c.Target)
	})

	if len(c.Links) > 0 {
		writeContextSection(&b, "Related work", func() {
			for _, l := range c.Links {
				fmt.Fprintf(&b, "- %s %s", l.Relation, l.OtherID)
				if l.OtherTitle != "" {
					fmt.Fprintf(&b, " — %s", l.OtherTitle)
				}
				b.WriteString("\n")
			}
		})
	}

	if len(c.Refs) > 0 {
		writeContextSection(&b, "External references", func() {
			for _, r := range c.Refs {
				if r.Label != "" {
					fmt.Fprintf(&b, "- %s (%s)\n", r.Label, r.URL)
				} else {
					fmt.Fprintf(&b, "- %s\n", r.URL)
				}
			}
		})
	}

	return b.String()
}

func writeContextSection(b *strings.Builder, title string, body func()) {
	if b.Len() > 0 {
		b.WriteString("\n")
	}
	fmt.Fprintf(b, "# %s\n\n", title)
	body()
}

// writeNestedEntity renders one entity as a markdown list item at the
// given nesting depth. Each depth level adds two spaces of indent so
// children sit visually inside their parent. The role label is either
// "context" (read-only ancestor) or "target" (the item to decompose).
func writeNestedEntity(b *strings.Builder, depth int, role string, e DialogContextEntity) {
	indent := strings.Repeat("  ", depth)
	inner := indent + "  "

	header := fmt.Sprintf("%s %s — %q (%s)", e.Kind, e.ID, e.Title, e.Status)
	var meta []string
	if e.Priority != "" {
		meta = append(meta, fmt.Sprintf("priority: %s", e.Priority))
	}
	if e.Size != "" {
		meta = append(meta, fmt.Sprintf("size: %s", e.Size))
	}
	if len(meta) > 0 {
		header += " · " + strings.Join(meta, " · ")
	}
	fmt.Fprintf(b, "%s- [%s] **%s**\n", indent, role, header)

	if e.Description != "" {
		b.WriteString("\n")
		for _, line := range strings.Split(strings.TrimRight(e.Description, "\n"), "\n") {
			fmt.Fprintf(b, "%s%s\n", inner, line)
		}
		b.WriteString("\n")
	}

	if len(e.Tags) > 0 {
		fmt.Fprintf(b, "%sTags: %s\n", inner, strings.Join(e.Tags, ", "))
	}

	if len(e.Given) > 0 || len(e.When) > 0 || len(e.Then) > 0 {
		if len(e.Given) > 0 {
			fmt.Fprintf(b, "%sGiven:\n", inner)
			for _, s := range e.Given {
				fmt.Fprintf(b, "%s  - %s\n", inner, s)
			}
		}
		if len(e.When) > 0 {
			fmt.Fprintf(b, "%sWhen:\n", inner)
			for _, s := range e.When {
				fmt.Fprintf(b, "%s  - %s\n", inner, s)
			}
		}
		if len(e.Then) > 0 {
			fmt.Fprintf(b, "%sThen:\n", inner)
			for _, s := range e.Then {
				fmt.Fprintf(b, "%s  - %s\n", inner, s)
			}
		}
	}
}
