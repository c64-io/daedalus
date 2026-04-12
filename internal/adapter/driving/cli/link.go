package cli

import (
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/c64-io/daedalus/internal/core/domain"
	"github.com/c64-io/daedalus/internal/core/port/driving"
)

func newLinkCmd(adder driving.LinkAdder, remover driving.LinkRemover, reader driving.LinkReader) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "link",
		Short: "Manage typed cross-links between entities",
		Long: `Manage typed cross-links between any entities in the workspace.

Link kinds:
  blocked-by   A is blocked by B (asymmetric; shows "blocks" on B)
  relates-to   A and B are related (symmetric)
  duplicates   A duplicates B (symmetric)`,
	}

	cmd.AddCommand(newLinkAddCmd(adder))
	cmd.AddCommand(newLinkRmCmd(remover))
	cmd.AddCommand(newLinkListCmd(reader))
	return cmd
}

func newLinkAddCmd(adder driving.LinkAdder) *cobra.Command {
	return &cobra.Command{
		Use:   "add <from-id> <kind> <to-id>",
		Short: "Create a typed link between two entities",
		Long: `Create a typed link between two entities.

Examples:
  d7 link add STORY-047 blocked-by STORY-012
  d7 link add FEAT-001 relates-to EPIC-003
  d7 link add STORY-002 duplicates STORY-005`,
		Args: cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			fromID := strings.ToUpper(strings.TrimSpace(args[0]))
			kindRaw := strings.ToLower(strings.TrimSpace(args[1]))
			toID := strings.ToUpper(strings.TrimSpace(args[2]))

			kind, err := domain.ParseLinkKind(kindRaw)
			if err != nil {
				return err
			}

			link, err := adder.AddLink(cmd.Context(), driving.AddLinkRequest{
				FromID: fromID,
				Kind:   kind,
				ToID:   toID,
			})
			if err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "linked %s %s %s\n", link.FromID, link.Kind, link.ToID)
			return nil
		},
	}
}

func newLinkRmCmd(remover driving.LinkRemover) *cobra.Command {
	return &cobra.Command{
		Use:   "rm <from-id> <kind> <to-id>",
		Short: "Remove a link between two entities",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			fromID := strings.ToUpper(strings.TrimSpace(args[0]))
			kindRaw := strings.ToLower(strings.TrimSpace(args[1]))
			toID := strings.ToUpper(strings.TrimSpace(args[2]))

			kind, err := domain.ParseLinkKind(kindRaw)
			if err != nil {
				return err
			}

			if err := remover.RemoveLink(cmd.Context(), driving.RemoveLinkRequest{
				FromID: fromID,
				Kind:   kind,
				ToID:   toID,
			}); err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "removed %s %s %s\n", fromID, kind, toID)
			return nil
		},
	}
}

func newLinkListCmd(reader driving.LinkReader) *cobra.Command {
	return &cobra.Command{
		Use:   "list [entity-id]",
		Short: "List links (all or for a specific entity)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var entityID string
			if len(args) > 0 {
				entityID = strings.ToUpper(strings.TrimSpace(args[0]))
			}

			links, err := reader.ListLinks(cmd.Context(), "", entityID)
			if err != nil {
				return err
			}

			if len(links) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no links yet — create one with: d7 link add <from> <kind> <to>")
				return nil
			}

			out := cmd.OutOrStdout()
			w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "FROM\tKIND\tTO")
			for _, rl := range links {
				fromLabel := rl.FromID
				if rl.FromTitle != "" {
					fromLabel = rl.FromID + " — " + rl.FromTitle
				}
				toLabel := rl.ToID
				if rl.ToTitle != "" {
					toLabel = rl.ToID + " — " + rl.ToTitle
				}
				fmt.Fprintf(w, "%s\t%s\t%s\n", fromLabel, rl.Kind, toLabel)
			}
			return w.Flush()
		},
	}
}
