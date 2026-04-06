package cli

import (
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/c64-io/daedalus/internal/core/domain"
	"github.com/c64-io/daedalus/internal/core/port"
)

func newEpicCmd(creator port.EpicCreator, reader port.EpicReader, setter port.EpicSetter, ideaReader port.IdeaReader, editor port.Editor) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "epic",
		Short: "Manage epics — major capabilities under an idea",
	}

	cmd.AddCommand(newEpicNewCmd(creator))
	cmd.AddCommand(newEpicListCmd(reader))
	cmd.AddCommand(newEpicShowCmd(reader, ideaReader))
	cmd.AddCommand(newEpicSetCmd(setter))
	cmd.AddCommand(newEpicEditCmd(reader, setter, editor))
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
				priority := "-"
				if epic.Priority != "" {
					priority = string(epic.Priority)
				}
				size := "-"
				if epic.Size != 0 {
					size = fmt.Sprintf("%d", epic.Size)
				}
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
					epic.ID, epic.IdeaID, epic.Status, priority, size, epic.Title)
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
			fmt.Fprintf(out, "Created:     %s\n", epic.CreatedAt.Format("2006-01-02 15:04:05"))
			return nil
		},
	}
}

func newEpicSetCmd(setter port.EpicSetter) *cobra.Command {
	var (
		statusRaw   string
		title       string
		description string
		priorityRaw string
		sizeRaw     string
	)

	cmd := &cobra.Command{
		Use:   "set <epic-id>",
		Short: "Update fields on an epic",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := strings.ToUpper(strings.TrimSpace(args[0]))

			req := port.SetEpicRequest{ID: id}

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
			if cmd.Flags().Changed("priority") {
				p, err := domain.ParsePriority(priorityRaw)
				if err != nil {
					return err
				}
				req.Priority = &p
			}
			if cmd.Flags().Changed("size") {
				sz, err := domain.ParseSize(sizeRaw)
				if err != nil {
					return err
				}
				req.Size = &sz
			}

			if req.Status == nil && req.Title == nil && req.Description == nil && req.Priority == nil && req.Size == nil {
				fmt.Fprintln(cmd.OutOrStdout(), "available flags: --status, --title, --description, --priority, --size")
				return nil
			}

			epic, err := setter.SetEpic(cmd.Context(), req)
			if err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "%s updated (%s)\n", epic.ID, epic.Status)
			return nil
		},
	}

	cmd.Flags().StringVar(&statusRaw, "status", "", "new status (draft, refined, ready, in-progress, review, done, archived, blocked)")
	cmd.Flags().StringVar(&title, "title", "", "new title")
	cmd.Flags().StringVar(&description, "description", "", "new description")
	cmd.Flags().StringVar(&priorityRaw, "priority", "", "new priority ("+domain.SupportedPriorities()+")")
	cmd.Flags().StringVar(&sizeRaw, "size", "", "new size ("+domain.SupportedSizes()+")")

	return cmd
}

func newEpicEditCmd(reader port.EpicReader, setter port.EpicSetter, editor port.Editor) *cobra.Command {
	return &cobra.Command{
		Use:   "edit <epic-id>",
		Short: "Edit an epic in $EDITOR",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := strings.ToUpper(strings.TrimSpace(args[0]))
			epic, err := reader.GetEpic(cmd.Context(), "", id)
			if err != nil {
				return err
			}

			priority := ""
			if epic.Priority != "" {
				priority = string(epic.Priority)
			}
			size := ""
			if epic.Size != 0 {
				size = fmt.Sprintf("%d", epic.Size)
			}

			fields := []domain.FrontMatterField{
				{Key: "id", Value: epic.ID, ReadOnly: true},
				{Key: "idea", Value: epic.IdeaID, ReadOnly: true},
				{Key: "status", Value: string(epic.Status)},
				{Key: "title", Value: epic.Title},
				{Key: "priority", Value: priority},
				{Key: "size", Value: size},
				{Key: "created", Value: epic.CreatedAt.Format("2006-01-02"), ReadOnly: true},
			}
			initial := domain.FormatFrontMatter(fields, epic.Description)

			var updated *domain.Epic
			_, err = editLoop(editor, initial, func(edited string) error {
				cleaned := stripErrorLines(edited)
				fm, body, parseErr := domain.ParseFrontMatter(cleaned)
				if parseErr != nil {
					return parseErr
				}

				req := port.SetEpicRequest{ID: epic.ID}

				// Warn about read-only fields.
				if val, ok := fm["id"]; ok && val != epic.ID {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: id is read-only, ignoring change\n")
				}
				if val, ok := fm["idea"]; ok && val != epic.IdeaID {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: idea is read-only, ignoring change\n")
				}
				if val, ok := fm["created"]; ok && val != epic.CreatedAt.Format("2006-01-02") {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: created is read-only, ignoring change\n")
				}

				// Editable fields — only set if changed.
				if val, ok := fm["status"]; ok && val != string(epic.Status) {
					s, err := domain.ParseStatus(val)
					if err != nil {
						return err
					}
					req.Status = &s
				}
				if val, ok := fm["title"]; ok && val != epic.Title {
					req.Title = &val
				}
				if val, ok := fm["priority"]; ok && val != priority {
					if val == "" {
						// Skip clearing priority.
					} else {
						p, err := domain.ParsePriority(val)
						if err != nil {
							return err
						}
						req.Priority = &p
					}
				}
				if val, ok := fm["size"]; ok && val != size {
					if val == "" {
						// Skip clearing size.
					} else {
						sz, err := domain.ParseSize(val)
						if err != nil {
							return err
						}
						req.Size = &sz
					}
				}
				if body != epic.Description {
					req.Description = &body
				}

				// No changes — nothing to do.
				if req.Status == nil && req.Title == nil && req.Description == nil && req.Priority == nil && req.Size == nil {
					return nil
				}

				// Apply changes — validation errors (e.g. invalid status
				// transition) re-open the editor with the error shown.
				result, err := setter.SetEpic(cmd.Context(), req)
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
