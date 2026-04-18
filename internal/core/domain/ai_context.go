package domain

import (
	"fmt"
	"strings"
)

// DialogContext is a neutral, renderable snapshot of the minimum the
// LLM needs to see in its cached context: the project description and
// the target entity. Ancestors, siblings, cross-links, and detailed
// item bodies are available on demand via navigation tools (get_lineage,
// get_item_detail, get_siblings, trace_links).
//
// Refs attached to the target are rendered inline because they are
// properties of the target, not a separate graph traversal.
type DialogContext struct {
	Workspace string // project description body, already trimmed

	// Target is the entity the command is operating on.
	Target DialogContextEntity

	// Refs are external references attached to the target, rendered
	// inline beneath the target block.
	Refs []DialogContextRef
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

// DialogContextRef describes one external reference on the target.
type DialogContextRef struct {
	URL   string
	Label string // optional
}

// FormatDialogContext renders a DialogContext as the stable text
// block handed to the LLM as cached context. The output is
// deterministic for a given input — same struct in, same bytes out.
//
// Two sections: # Product (workspace description) and # Target (the
// entity the command operates on, with inline refs when present).
// Ancestors and links are not included — the model explores them
// on demand via navigation tools.
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

	writeContextSection(&b, "Target", func() {
		writeTargetEntity(&b, c.Target)
		if len(c.Refs) > 0 {
			b.WriteString("\nRefs:\n")
			for _, r := range c.Refs {
				if r.Label != "" {
					fmt.Fprintf(&b, "- %s (%s)\n", r.Label, r.URL)
				} else {
					fmt.Fprintf(&b, "- %s\n", r.URL)
				}
			}
		}
	})

	return b.String()
}

func writeContextSection(b *strings.Builder, title string, body func()) {
	if b.Len() > 0 {
		b.WriteString("\n")
	}
	fmt.Fprintf(b, "# %s\n\n", title)
	body()
}

// writeTargetEntity renders the target entity in the cached context
// block. No nesting or role labels — this is the only entity in the
// block. The output includes kind/ID/title/status header, optional
// priority/size, the description body, and scenario-specific fields.
func writeTargetEntity(b *strings.Builder, e DialogContextEntity) {
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
	fmt.Fprintf(b, "**%s**\n", header)

	if e.Description != "" {
		b.WriteString("\n")
		for _, line := range strings.Split(strings.TrimRight(e.Description, "\n"), "\n") {
			fmt.Fprintf(b, "  %s\n", line)
		}
		b.WriteString("\n")
	}

	if len(e.Tags) > 0 {
		fmt.Fprintf(b, "Tags: %s\n", strings.Join(e.Tags, ", "))
	}

	if len(e.Given) > 0 || len(e.When) > 0 || len(e.Then) > 0 {
		if len(e.Given) > 0 {
			b.WriteString("Given:\n")
			for _, s := range e.Given {
				fmt.Fprintf(b, "  - %s\n", s)
			}
		}
		if len(e.When) > 0 {
			b.WriteString("When:\n")
			for _, s := range e.When {
				fmt.Fprintf(b, "  - %s\n", s)
			}
		}
		if len(e.Then) > 0 {
			b.WriteString("Then:\n")
			for _, s := range e.Then {
				fmt.Fprintf(b, "  - %s\n", s)
			}
		}
	}
}

// WriteEntityDetail renders an entity in the same format used by
// writeTargetEntity, for use by navigation tools (get_item_detail).
// Refs are appended inline when present.
func WriteEntityDetail(e DialogContextEntity, refs []DialogContextRef) string {
	var b strings.Builder
	writeTargetEntity(&b, e)
	if len(refs) > 0 {
		b.WriteString("\nRefs:\n")
		for _, r := range refs {
			if r.Label != "" {
				fmt.Fprintf(&b, "- %s (%s)\n", r.Label, r.URL)
			} else {
				fmt.Fprintf(&b, "- %s\n", r.URL)
			}
		}
	}
	return b.String()
}
