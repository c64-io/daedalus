package cli

import (
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/c64-io/daedalus/internal/core/domain"
	"github.com/c64-io/daedalus/internal/core/port"
)

func newEpicCmd(creator port.EpicCreator, reader port.EpicReader, ideaReader port.IdeaReader) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "epic",
		Short: "Manage epics — major capabilities under an idea",
	}

	cmd.AddCommand(newEpicNewCmd(creator))
	cmd.AddCommand(newEpicListCmd(reader))
	cmd.AddCommand(newEpicShowCmd(reader, ideaReader))
	return cmd
}

func newEpicNewCmd(creator port.EpicCreator) *cobra.Command {
	var (
		ideaID      string
		title       string
		description string
		priorityRaw string
		sizeRaw     string
		expand      bool
	)

	cmd := &cobra.Command{
		Use:   "new",
		Short: "Create a new epic under an idea",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ideaID = strings.ToUpper(strings.TrimSpace(ideaID))

			var priority domain.Priority
			if priorityRaw != "" {
				var err error
				priority, err = domain.ParsePriority(priorityRaw)
				if err != nil {
					return err
				}
			}

			var size domain.Size
			if sizeRaw != "" {
				var err error
				size, err = domain.ParseSize(sizeRaw)
				if err != nil {
					return err
				}
			}

			epic, err := creator.CreateEpic(cmd.Context(), port.CreateEpicRequest{
				IdeaID:      ideaID,
				Title:       title,
				Description: description,
				Priority:    priority,
				Size:        size,
			})
			if err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "created %s: %s (under %s)\n", epic.ID, epic.Title, epic.IdeaID)

			if expand {
				fmt.Fprintln(out, "\nAI-assisted expansion is not yet available.")
				fmt.Fprintf(out, "When ready, use: d7 suggest features --epic %s\n", epic.ID)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&ideaID, "idea", "", "parent idea ID (required, e.g. IDEA-001)")
	cmd.Flags().StringVar(&title, "title", "", "epic title (required)")
	cmd.Flags().StringVar(&description, "description", "", "longer prose description (optional)")
	cmd.Flags().StringVar(&priorityRaw, "priority", "", "priority (optional; one of: "+domain.SupportedPriorities()+")")
	cmd.Flags().StringVar(&sizeRaw, "size", "", "size in Fibonacci points (optional; one of: "+domain.SupportedSizes()+")")
	cmd.Flags().BoolVar(&expand, "expand", false,
		"immediately start AI-assisted expansion after creation (coming soon)")
	_ = cmd.MarkFlagRequired("idea")
	_ = cmd.MarkFlagRequired("title")

	return cmd
}

func newEpicListCmd(reader port.EpicReader) *cobra.Command {
	var ideaID string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List epics (all or filtered by idea)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if ideaID != "" {
				ideaID = strings.ToUpper(strings.TrimSpace(ideaID))
			}

			epics, err := reader.ListEpics(cmd.Context(), "", ideaID)
			if err != nil {
				return err
			}

			if len(epics) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no epics yet — create one with: d7 epic new --idea IDEA-001 --title \"...\"")
				return nil
			}

			out := cmd.OutOrStdout()
			w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "ID\tIDEA\tSTATUS\tPRIORITY\tSIZE\tTITLE")
			for _, epic := range epics {
				status := string(epic.Status)
				if epic.Blocked {
					status += " [blocked]"
				}
				priority := "-"
				if epic.Priority != "" {
					priority = string(epic.Priority)
				}
				size := "-"
				if epic.Size != 0 {
					size = fmt.Sprintf("%d", epic.Size)
				}
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
					epic.ID, epic.IdeaID, status, priority, size, epic.Title)
			}
			return w.Flush()
		},
	}

	cmd.Flags().StringVar(&ideaID, "idea", "", "filter by parent idea ID (e.g. IDEA-001)")
	return cmd
}

func newEpicShowCmd(reader port.EpicReader, ideaReader port.IdeaReader) *cobra.Command {
	return &cobra.Command{
		Use:   "show <epic-id>",
		Short: "Show details of a single epic",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := strings.ToUpper(strings.TrimSpace(args[0]))
			epic, err := reader.GetEpic(cmd.Context(), "", id)
			if err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "%s  (%s)\n", epic.ID, epic.Status)
			fmt.Fprintf(out, "Title:       %s\n", epic.Title)

			// Show parent idea info.
			idea, err := ideaReader.GetIdea(cmd.Context(), "", epic.IdeaID)
			if err == nil {
				fmt.Fprintf(out, "Idea:        %s — %s\n", idea.ID, idea.Title)
			} else {
				fmt.Fprintf(out, "Idea:        %s\n", epic.IdeaID)
			}

			if epic.Description != "" {
				fmt.Fprintf(out, "Description: %s\n", epic.Description)
			}
			if epic.Priority != "" {
				fmt.Fprintf(out, "Priority:    %s\n", epic.Priority)
			}
			if epic.Size != 0 {
				fmt.Fprintf(out, "Size:        %d\n", epic.Size)
			}
			if epic.Blocked {
				fmt.Fprintln(out, "Blocked:     yes")
			}
			fmt.Fprintf(out, "Created:     %s\n", epic.CreatedAt.Format("2006-01-02 15:04:05"))
			return nil
		},
	}
}
