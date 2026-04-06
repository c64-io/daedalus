package cli

import (
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/c64-io/daedalus/internal/core/domain"
	"github.com/c64-io/daedalus/internal/core/port"
)

func newIdeaCmd(creator port.IdeaCreator, reader port.IdeaReader, setter port.IdeaSetter) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "idea",
		Short: "Manage ideas — the top of the d7 hierarchy",
	}

	cmd.AddCommand(newIdeaNewCmd(creator))
	cmd.AddCommand(newIdeaListCmd(reader))
	cmd.AddCommand(newIdeaShowCmd(reader))
	cmd.AddCommand(newIdeaSetCmd(setter))
	return cmd
}

func newIdeaNewCmd(creator port.IdeaCreator) *cobra.Command {
	var (
		title       string
		description string
		expand      bool
	)

	cmd := &cobra.Command{
		Use:   "new",
		Short: "Create a new idea",
		RunE: func(cmd *cobra.Command, _ []string) error {
			idea, err := creator.CreateIdea(cmd.Context(), port.CreateIdeaRequest{
				Title:       title,
				Description: description,
			})
			if err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "created %s: %s\n", idea.ID, idea.Title)

			if expand {
				fmt.Fprintln(out, "\nAI-assisted expansion is not yet available.")
				fmt.Fprintf(out, "When ready, use: d7 suggest epics --idea %s\n", idea.ID)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&title, "title", "", "idea title (required)")
	cmd.Flags().StringVar(&description, "description", "", "longer prose description (optional)")
	cmd.Flags().BoolVar(&expand, "expand", false,
		"immediately start AI-assisted expansion after creation (coming soon)")
	_ = cmd.MarkFlagRequired("title")

	return cmd
}

func newIdeaListCmd(reader port.IdeaReader) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all ideas in the workspace",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ideas, err := reader.ListIdeas(cmd.Context(), "")
			if err != nil {
				return err
			}

			if len(ideas) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no ideas yet — create one with: d7 idea new --title \"...\"")
				return nil
			}

			out := cmd.OutOrStdout()
			w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "ID\tSTATUS\tTITLE")
			for _, idea := range ideas {
				fmt.Fprintf(w, "%s\t%s\t%s\n", idea.ID, idea.Status, idea.Title)
			}
			return w.Flush()
		},
	}
}

func newIdeaShowCmd(reader port.IdeaReader) *cobra.Command {
	return &cobra.Command{
		Use:   "show <idea-id>",
		Short: "Show details of a single idea",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := strings.ToUpper(strings.TrimSpace(args[0]))
			idea, err := reader.GetIdea(cmd.Context(), "", id)
			if err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "%s  (%s)\n", idea.ID, idea.Status)
			fmt.Fprintf(out, "Title:       %s\n", idea.Title)
			if idea.Description != "" {
				fmt.Fprintf(out, "Description: %s\n", idea.Description)
			}
			fmt.Fprintf(out, "Created:     %s\n", idea.CreatedAt.Format("2006-01-02 15:04:05"))
			return nil
		},
	}
}

func newIdeaSetCmd(setter port.IdeaSetter) *cobra.Command {
	var (
		statusRaw   string
		title       string
		description string
	)

	cmd := &cobra.Command{
		Use:   "set <idea-id>",
		Short: "Update fields on an idea",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := strings.ToUpper(strings.TrimSpace(args[0]))

			req := port.SetIdeaRequest{ID: id}

			if cmd.Flags().Changed("status") {
				s, err := domain.ParseStatus(statusRaw)
				if err != nil {
					return err
				}
				req.Status = &s
			}
			if cmd.Flags().Changed("title") {
				req.Title = &title
			}
			if cmd.Flags().Changed("description") {
				req.Description = &description
			}

			if req.Status == nil && req.Title == nil && req.Description == nil {
				fmt.Fprintln(cmd.OutOrStdout(), "available flags: --status, --title, --description")
				return nil
			}

			idea, err := setter.SetIdea(cmd.Context(), req)
			if err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "%s updated (%s)\n", idea.ID, idea.Status)
			return nil
		},
	}

	cmd.Flags().StringVar(&statusRaw, "status", "", "new status (draft, refined, ready, in-progress, review, done, archived, blocked)")
	cmd.Flags().StringVar(&title, "title", "", "new title")
	cmd.Flags().StringVar(&description, "description", "", "new description")

	return cmd
}
