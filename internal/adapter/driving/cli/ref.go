package cli

import (
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/c64-io/daedalus/internal/core/port/driving"
)

func newRefCmd(adder driving.RefAdder, remover driving.RefRemover, reader driving.RefReader) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ref",
		Short: "Manage external references (URLs) attached to entities",
	}

	cmd.AddCommand(newRefAddCmd(adder))
	cmd.AddCommand(newRefRmCmd(remover))
	cmd.AddCommand(newRefListCmd(reader))
	return cmd
}

func newRefAddCmd(adder driving.RefAdder) *cobra.Command {
	var label string

	cmd := &cobra.Command{
		Use:   "add <entity-id> <url>",
		Short: "Attach an external reference to an entity",
		Long: `Attach an external reference (URL) to any entity.

Examples:
  d7 ref add STORY-047 https://figma.com/file/abc --label "Login mockup"
  d7 ref add EPIC-001 https://github.com/c64-io/foo/issues/42`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			entityID := strings.ToUpper(strings.TrimSpace(args[0]))
			url := strings.TrimSpace(args[1])

			ref, err := adder.AddRef(cmd.Context(), driving.AddRefRequest{
				EntityID: entityID,
				URL:      url,
				Label:    strings.TrimSpace(label),
			})
			if err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			if ref.Label != "" {
				fmt.Fprintf(out, "added ref to %s: %s (%s)\n", ref.EntityID, ref.Label, ref.URL)
			} else {
				fmt.Fprintf(out, "added ref to %s: %s\n", ref.EntityID, ref.URL)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&label, "label", "", "optional display label for the URL")
	return cmd
}

func newRefRmCmd(remover driving.RefRemover) *cobra.Command {
	return &cobra.Command{
		Use:   "rm <entity-id> <url>",
		Short: "Remove an external reference from an entity",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			entityID := strings.ToUpper(strings.TrimSpace(args[0]))
			url := strings.TrimSpace(args[1])

			if err := remover.RemoveRef(cmd.Context(), driving.RemoveRefRequest{
				EntityID: entityID,
				URL:      url,
			}); err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "removed ref from %s: %s\n", entityID, url)
			return nil
		},
	}
}

func newRefListCmd(reader driving.RefReader) *cobra.Command {
	return &cobra.Command{
		Use:   "list [entity-id]",
		Short: "List external references (all or for a specific entity)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var entityID string
			if len(args) > 0 {
				entityID = strings.ToUpper(strings.TrimSpace(args[0]))
			}

			refs, err := reader.ListRefs(cmd.Context(), "", entityID)
			if err != nil {
				return err
			}

			if len(refs) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no refs yet — add one with: d7 ref add <entity-id> <url>")
				return nil
			}

			out := cmd.OutOrStdout()
			w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "ENTITY\tLABEL\tURL")
			for _, ref := range refs {
				fmt.Fprintf(w, "%s\t%s\t%s\n", ref.EntityID, ref.Label, ref.URL)
			}
			return w.Flush()
		},
	}
}
