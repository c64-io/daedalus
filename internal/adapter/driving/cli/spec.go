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

func newSpecCmd(creator driving.SpecCreator, reader driving.SpecReader, setter driving.SpecSetter, storyReader driving.StoryReader, linkReader driving.LinkReader, refReader driving.RefReader, editor driven.Editor) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "spec",
		Short: "Manage specs — prose requirements attached to a story",
	}

	cmd.AddCommand(newSpecNewCmd(creator))
	cmd.AddCommand(newSpecListCmd(reader))
	cmd.AddCommand(newSpecShowCmd(reader, storyReader, linkReader, refReader))
	cmd.AddCommand(newSpecSetCmd(setter))
	cmd.AddCommand(newSpecEditCmd(reader, setter, editor))
	return cmd
}

func newSpecNewCmd(creator driving.SpecCreator) *cobra.Command {
	var (
		storyID     string
		title       string
		description string
		expand      bool
	)

	cmd := &cobra.Command{
		Use:   "new",
		Short: "Create a new spec under a story",
		RunE: func(cmd *cobra.Command, _ []string) error {
			storyID = strings.ToUpper(strings.TrimSpace(storyID))

			spec, err := creator.CreateSpec(cmd.Context(), driving.CreateSpecRequest{
				StoryID:     storyID,
				Title:       title,
				Description: description,
			})
			if err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "created %s: %s (under %s)\n", spec.ID, spec.Title, spec.StoryID)

			if expand {
				fmt.Fprintln(out, "\nAI-assisted expansion is not yet available.")
				fmt.Fprintf(out, "When ready, use: d7 expand spec %s\n", spec.ID)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&storyID, "story", "", "parent story ID (required, e.g. STORY-001)")
	cmd.Flags().StringVar(&title, "title", "", "spec title (required)")
	cmd.Flags().StringVar(&description, "description", "", "prose body (optional)")
	cmd.Flags().BoolVar(&expand, "expand", false,
		"immediately start AI-assisted expansion after creation (coming soon)")
	_ = cmd.MarkFlagRequired("story")
	_ = cmd.MarkFlagRequired("title")

	return cmd
}

func newSpecListCmd(reader driving.SpecReader) *cobra.Command {
	var storyID string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List specs (all or filtered by story)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if storyID != "" {
				storyID = strings.ToUpper(strings.TrimSpace(storyID))
			}

			specs, err := reader.ListSpecs(cmd.Context(), "", storyID)
			if err != nil {
				return err
			}

			if len(specs) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no specs yet — create one with: d7 spec new --story STORY-001 --title \"...\"")
				return nil
			}

			out := cmd.OutOrStdout()
			w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "ID\tSTORY\tSTATUS\tTITLE")
			for _, s := range specs {
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
					s.ID, s.StoryID, s.Status, s.Title)
			}
			return w.Flush()
		},
	}

	cmd.Flags().StringVar(&storyID, "story", "", "filter by parent story ID (e.g. STORY-001)")
	return cmd
}

func newSpecShowCmd(reader driving.SpecReader, storyReader driving.StoryReader, linkReader driving.LinkReader, refReader driving.RefReader) *cobra.Command {
	return &cobra.Command{
		Use:   "show <spec-id>",
		Short: "Show details of a single spec",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := strings.ToUpper(strings.TrimSpace(args[0]))
			spec, err := reader.GetSpec(cmd.Context(), "", id)
			if err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "%s  (%s)\n", spec.ID, spec.Status)
			fmt.Fprintf(out, "Title:       %s\n", spec.Title)

			// Show parent story info.
			story, err := storyReader.GetStory(cmd.Context(), "", spec.StoryID)
			if err == nil {
				fmt.Fprintf(out, "Story:       %s — %s\n", story.ID, story.Title)
			} else {
				fmt.Fprintf(out, "Story:       %s\n", spec.StoryID)
			}

			if spec.Description != "" {
				fmt.Fprintf(out, "Description: %s\n", spec.Description)
			}
			fmt.Fprintf(out, "Created:     %s\n", spec.CreatedAt.Format("2006-01-02 15:04:05"))
			renderLinksAndRefs(cmd.Context(), out, spec.ID, linkReader, refReader)
			return nil
		},
	}
}

func newSpecSetCmd(setter driving.SpecSetter) *cobra.Command {
	var (
		statusRaw   string
		title       string
		description string
	)

	cmd := &cobra.Command{
		Use:   "set <spec-id>",
		Short: "Update fields on a spec",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := strings.ToUpper(strings.TrimSpace(args[0]))

			req := driving.SetSpecRequest{ID: id}

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

			spec, err := setter.SetSpec(cmd.Context(), req)
			if err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "%s updated (%s)\n", spec.ID, spec.Status)
			return nil
		},
	}

	cmd.Flags().StringVar(&statusRaw, "status", "", "new status (draft, refined, ready, in-progress, review, done, archived, blocked)")
	cmd.Flags().StringVar(&title, "title", "", "new title")
	cmd.Flags().StringVar(&description, "description", "", "new description")

	return cmd
}

func newSpecEditCmd(reader driving.SpecReader, setter driving.SpecSetter, editor driven.Editor) *cobra.Command {
	return &cobra.Command{
		Use:   "edit <spec-id>",
		Short: "Edit a spec in $EDITOR",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := strings.ToUpper(strings.TrimSpace(args[0]))
			spec, err := reader.GetSpec(cmd.Context(), "", id)
			if err != nil {
				return err
			}

			fields := []domain.FrontMatterField{
				{Key: "id", Value: spec.ID, ReadOnly: true},
				{Key: "story", Value: spec.StoryID, ReadOnly: true},
				{Key: "status", Value: string(spec.Status)},
				{Key: "title", Value: spec.Title},
				{Key: "created", Value: spec.CreatedAt.Format("2006-01-02"), ReadOnly: true},
			}
			initial := domain.FormatFrontMatter(fields, spec.Description)

			var updated *domain.Spec
			_, err = editLoop(editor, initial, func(edited string) error {
				cleaned := stripErrorLines(edited)
				fm, body, parseErr := domain.ParseFrontMatter(cleaned)
				if parseErr != nil {
					return parseErr
				}

				req := driving.SetSpecRequest{ID: spec.ID}

				// Warn about read-only fields.
				if val, ok := fm["id"]; ok && val != spec.ID {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: id is read-only, ignoring change\n")
				}
				if val, ok := fm["story"]; ok && val != spec.StoryID {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: story is read-only, ignoring change\n")
				}
				if val, ok := fm["created"]; ok && val != spec.CreatedAt.Format("2006-01-02") {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: created is read-only, ignoring change\n")
				}

				// Editable fields — only set if changed.
				if val, ok := fm["status"]; ok && val != string(spec.Status) {
					s, err := domain.ParseStatus(val)
					if err != nil {
						return err
					}
					req.Status = &s
				}
				if val, ok := fm["title"]; ok && val != spec.Title {
					req.Title = &val
				}
				if body != spec.Description {
					req.Description = &body
				}

				// No changes — nothing to do.
				if req.Status == nil && req.Title == nil && req.Description == nil {
					return nil
				}

				// Apply changes — validation errors re-open the editor.
				result, err := setter.SetSpec(cmd.Context(), req)
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
