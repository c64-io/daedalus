package cli

import (
	"context"
	"fmt"
	"io"

	"github.com/c64-io/daedalus/internal/core/domain"
	"github.com/c64-io/daedalus/internal/core/port/driving"
)

// renderLinksAndRefs writes the links and refs sections for an entity's
// show command output. If there are no links or refs, nothing is printed.
func renderLinksAndRefs(ctx context.Context, out io.Writer, entityID string, linkReader driving.LinkReader, refReader driving.RefReader) {
	links, err := linkReader.ListLinks(ctx, "", entityID)
	if err != nil {
		return // silently skip on error — links are supplementary
	}

	// Group links by display category relative to this entity.
	type entry struct {
		id    string
		title string
	}
	blockedBy := []entry{}
	blocks := []entry{}
	relatesTo := []entry{}
	duplicates := []entry{}

	for _, rl := range links {
		switch {
		case rl.Kind == domain.LinkBlockedBy && rl.FromID == entityID:
			// This entity is blocked by rl.ToID
			blockedBy = append(blockedBy, entry{rl.ToID, rl.ToTitle})
		case rl.Kind == domain.LinkBlockedBy && rl.ToID == entityID:
			// This entity blocks rl.FromID
			blocks = append(blocks, entry{rl.FromID, rl.FromTitle})
		case rl.Kind == domain.LinkRelatesTo:
			other := rl.ToID
			otherTitle := rl.ToTitle
			if rl.ToID == entityID {
				other = rl.FromID
				otherTitle = rl.FromTitle
			}
			relatesTo = append(relatesTo, entry{other, otherTitle})
		case rl.Kind == domain.LinkDuplicates:
			other := rl.ToID
			otherTitle := rl.ToTitle
			if rl.ToID == entityID {
				other = rl.FromID
				otherTitle = rl.FromTitle
			}
			duplicates = append(duplicates, entry{other, otherTitle})
		}
	}

	printSection := func(label string, entries []entry) {
		if len(entries) == 0 {
			return
		}
		fmt.Fprintf(out, "%s:\n", label)
		for _, e := range entries {
			if e.title != "" {
				fmt.Fprintf(out, "  %s — %s\n", e.id, e.title)
			} else {
				fmt.Fprintf(out, "  %s\n", e.id)
			}
		}
	}

	printSection("Blocked by", blockedBy)
	printSection("Blocks", blocks)
	printSection("Relates to", relatesTo)
	printSection("Duplicates", duplicates)

	// Refs.
	refs, err := refReader.ListRefs(ctx, "", entityID)
	if err != nil {
		return
	}
	if len(refs) > 0 {
		fmt.Fprintln(out, "Refs:")
		for _, ref := range refs {
			if ref.Label != "" {
				fmt.Fprintf(out, "  %s — %s\n", ref.URL, ref.Label)
			} else {
				fmt.Fprintf(out, "  %s\n", ref.URL)
			}
		}
	}
}
