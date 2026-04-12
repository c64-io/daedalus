package cli

import (
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/c64-io/daedalus/internal/core/domain"
	"github.com/c64-io/daedalus/internal/core/port/driven"
	"github.com/c64-io/daedalus/internal/core/port/driving"
)

func newIdeaCmd(creator driving.IdeaCreator, reader driving.IdeaReader, setter driving.IdeaSetter, linkReader driving.LinkReader, refReader driving.RefReader, editor driven.Editor) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "idea",
		Short: "Manage ideas — the top of the d7 hierarchy",
	}

	cmd.AddCommand(newIdeaNewCmd(creator))
	cmd.AddCommand(newIdeaListCmd(reader))
	cmd.AddCommand(newIdeaShowCmd(reader, linkReader, refReader))
	cmd.AddCommand(newIdeaSetCmd(setter))
	cmd.AddCommand(newIdeaEditCmd(reader, setter, editor))
	return cmd
}

func newIdeaNewCmd(creator driving.IdeaCreator) *cobra.Command {
	var (
		title       string
		description string
		expand      bool
	)

	cmd := &cobra.Command{
		Use:   "new",
		Short: "Create a new idea",
		RunE: func(cmd *cobra.Command, _ []string) error {
			idea, err := creator.CreateIdea(cmd.Context(), driving.CreateIdeaRequest{
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

func newIdeaListCmd(reader driving.IdeaReader) *cobra.Command {
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

func newIdeaShowCmd(reader driving.IdeaReader, linkReader driving.LinkReader, refReader driving.RefReader) *cobra.Command {
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
			renderLinksAndRefs(cmd.Context(), out, idea.ID, linkReader, refReader)
			return nil
		},
	}
}

func newIdeaSetCmd(setter driving.IdeaSetter) *cobra.Command {
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

			req := driving.SetIdeaRequest{ID: id}

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

func newIdeaEditCmd(reader driving.IdeaReader, setter driving.IdeaSetter, editor driven.Editor) *cobra.Command {
	return &cobra.Command{
		Use:   "edit <idea-id>",
		Short: "Edit an idea in $EDITOR",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := strings.ToUpper(strings.TrimSpace(args[0]))
			idea, err := reader.GetIdea(cmd.Context(), "", id)
			if err != nil {
				return err
			}

			fields := []domain.FrontMatterField{
				{Key: "id", Value: idea.ID, ReadOnly: true},
				{Key: "status", Value: string(idea.Status)},
				{Key: "title", Value: idea.Title},
				{Key: "created", Value: idea.CreatedAt.Format("2006-01-02"), ReadOnly: true},
			}
			initial := domain.FormatFrontMatter(fields, idea.Description)

			var updated *domain.Idea
			_, err = editLoop(editor, initial, func(edited string) error {
				cleaned := stripErrorLines(edited)
				fm, body, parseErr := domain.ParseFrontMatter(cleaned)
				if parseErr != nil {
					return parseErr
				}

				req := driving.SetIdeaRequest{ID: idea.ID}

				// Warn about read-only fields.
				if val, ok := fm["id"]; ok && val != idea.ID {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: id is read-only, ignoring change\n")
				}
				if val, ok := fm["created"]; ok && val != idea.CreatedAt.Format("2006-01-02") {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: created is read-only, ignoring change\n")
				}

				// Editable fields — only set if changed.
				if val, ok := fm["status"]; ok && val != string(idea.Status) {
					s, err := domain.ParseStatus(val)
					if err != nil {
						return err
					}
					req.Status = &s
				}
				if val, ok := fm["title"]; ok && val != idea.Title {
					req.Title = &val
				}
				if body != idea.Description {
					req.Description = &body
				}

				// No changes — nothing to do.
				if req.Status == nil && req.Title == nil && req.Description == nil {
					return nil
				}

				// Apply changes — validation errors (e.g. invalid status
				// transition) re-open the editor with the error shown.
				result, err := setter.SetIdea(cmd.Context(), req)
				if err != nil {
					return err
				}
				updated = result
				return nil
			})
			if err != nil {
				return err
			}

			if updated == nil {
				fmt.Fprintln(cmd.OutOrStdout(), "no changes")
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "%s updated (%s)\n", updated.ID, updated.Status)
			}
			return nil
		},
	}
}

// stripErrorLines removes leading "# ERROR:" lines that editLoop
// prepends on parse failures.
func stripErrorLines(s string) string {
	for strings.HasPrefix(s, "# ERROR:") {
		if idx := strings.Index(s, "\n"); idx >= 0 {
			s = s[idx+1:]
		} else {
			break
		}
	}
	return s
}
