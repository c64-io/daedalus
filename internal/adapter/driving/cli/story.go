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

func newStoryCmd(creator driving.StoryCreator, reader driving.StoryReader, setter driving.StorySetter, featureReader driving.FeatureReader, editor driven.Editor) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "story",
		Short: "Manage stories — user-visible slices of a feature",
	}

	cmd.AddCommand(newStoryNewCmd(creator))
	cmd.AddCommand(newStoryListCmd(reader))
	cmd.AddCommand(newStoryShowCmd(reader, featureReader))
	cmd.AddCommand(newStorySetCmd(setter))
	cmd.AddCommand(newStoryEditCmd(reader, setter, editor))
	return cmd
}

func newStoryNewCmd(creator driving.StoryCreator) *cobra.Command {
	var (
		featureID   string
		title       string
		description string
		priorityRaw string
		sizeRaw     string
		expand      bool
	)

	cmd := &cobra.Command{
		Use:   "new",
		Short: "Create a new story under a feature",
		RunE: func(cmd *cobra.Command, _ []string) error {
			featureID = strings.ToUpper(strings.TrimSpace(featureID))

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

			story, err := creator.CreateStory(cmd.Context(), driving.CreateStoryRequest{
				FeatureID:   featureID,
				Title:       title,
				Description: description,
				Priority:    priority,
				Size:        size,
			})
			if err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "created %s: %s (under %s)\n", story.ID, story.Title, story.FeatureID)

			if expand {
				fmt.Fprintln(out, "\nAI-assisted expansion is not yet available.")
				fmt.Fprintf(out, "When ready, use: d7 expand story %s\n", story.ID)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&featureID, "feature", "", "parent feature ID (required, e.g. FEAT-001)")
	cmd.Flags().StringVar(&title, "title", "", "story title (required)")
	cmd.Flags().StringVar(&description, "description", "", "longer prose description (optional)")
	cmd.Flags().StringVar(&priorityRaw, "priority", "", "priority (optional; one of: "+domain.SupportedPriorities()+")")
	cmd.Flags().StringVar(&sizeRaw, "size", "", "size in Fibonacci points (optional; one of: "+domain.SupportedSizes()+")")
	cmd.Flags().BoolVar(&expand, "expand", false,
		"immediately start AI-assisted expansion after creation (coming soon)")
	_ = cmd.MarkFlagRequired("feature")
	_ = cmd.MarkFlagRequired("title")

	return cmd
}

func newStoryListCmd(reader driving.StoryReader) *cobra.Command {
	var featureID string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List stories (all or filtered by feature)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if featureID != "" {
				featureID = strings.ToUpper(strings.TrimSpace(featureID))
			}

			stories, err := reader.ListStories(cmd.Context(), "", featureID)
			if err != nil {
				return err
			}

			if len(stories) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no stories yet — create one with: d7 story new --feature FEAT-001 --title \"...\"")
				return nil
			}

			out := cmd.OutOrStdout()
			w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "ID\tFEATURE\tSTATUS\tPRIORITY\tSIZE\tTITLE")
			for _, s := range stories {
				priority := "-"
				if s.Priority != "" {
					priority = string(s.Priority)
				}
				size := "-"
				if s.Size != 0 {
					size = fmt.Sprintf("%d", s.Size)
				}
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
					s.ID, s.FeatureID, s.Status, priority, size, s.Title)
			}
			return w.Flush()
		},
	}

	cmd.Flags().StringVar(&featureID, "feature", "", "filter by parent feature ID (e.g. FEAT-001)")
	return cmd
}

func newStoryShowCmd(reader driving.StoryReader, featureReader driving.FeatureReader) *cobra.Command {
	return &cobra.Command{
		Use:   "show <story-id>",
		Short: "Show details of a single story",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := strings.ToUpper(strings.TrimSpace(args[0]))
			story, err := reader.GetStory(cmd.Context(), "", id)
			if err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "%s  (%s)\n", story.ID, story.Status)
			fmt.Fprintf(out, "Title:       %s\n", story.Title)

			// Show parent feature info.
			feature, err := featureReader.GetFeature(cmd.Context(), "", story.FeatureID)
			if err == nil {
				fmt.Fprintf(out, "Feature:     %s — %s\n", feature.ID, feature.Title)
			} else {
				fmt.Fprintf(out, "Feature:     %s\n", story.FeatureID)
			}

			if story.Description != "" {
				fmt.Fprintf(out, "Description: %s\n", story.Description)
			}
			if story.Priority != "" {
				fmt.Fprintf(out, "Priority:    %s\n", story.Priority)
			}
			if story.Size != 0 {
				fmt.Fprintf(out, "Size:        %d\n", story.Size)
			}
			fmt.Fprintf(out, "Created:     %s\n", story.CreatedAt.Format("2006-01-02 15:04:05"))
			return nil
		},
	}
}

func newStorySetCmd(setter driving.StorySetter) *cobra.Command {
	var (
		statusRaw   string
		title       string
		description string
		priorityRaw string
		sizeRaw     string
	)

	cmd := &cobra.Command{
		Use:   "set <story-id>",
		Short: "Update fields on a story",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := strings.ToUpper(strings.TrimSpace(args[0]))

			req := driving.SetStoryRequest{ID: id}

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

			story, err := setter.SetStory(cmd.Context(), req)
			if err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "%s updated (%s)\n", story.ID, story.Status)
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

func newStoryEditCmd(reader driving.StoryReader, setter driving.StorySetter, editor driven.Editor) *cobra.Command {
	return &cobra.Command{
		Use:   "edit <story-id>",
		Short: "Edit a story in $EDITOR",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := strings.ToUpper(strings.TrimSpace(args[0]))
			story, err := reader.GetStory(cmd.Context(), "", id)
			if err != nil {
				return err
			}

			priority := ""
			if story.Priority != "" {
				priority = string(story.Priority)
			}
			size := ""
			if story.Size != 0 {
				size = fmt.Sprintf("%d", story.Size)
			}

			fields := []domain.FrontMatterField{
				{Key: "id", Value: story.ID, ReadOnly: true},
				{Key: "feature", Value: story.FeatureID, ReadOnly: true},
				{Key: "status", Value: string(story.Status)},
				{Key: "title", Value: story.Title},
				{Key: "priority", Value: priority},
				{Key: "size", Value: size},
				{Key: "created", Value: story.CreatedAt.Format("2006-01-02"), ReadOnly: true},
			}
			initial := domain.FormatFrontMatter(fields, story.Description)

			var updated *domain.Story
			_, err = editLoop(editor, initial, func(edited string) error {
				cleaned := stripErrorLines(edited)
				fm, body, parseErr := domain.ParseFrontMatter(cleaned)
				if parseErr != nil {
					return parseErr
				}

				req := driving.SetStoryRequest{ID: story.ID}

				// Warn about read-only fields.
				if val, ok := fm["id"]; ok && val != story.ID {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: id is read-only, ignoring change\n")
				}
				if val, ok := fm["feature"]; ok && val != story.FeatureID {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: feature is read-only, ignoring change\n")
				}
				if val, ok := fm["created"]; ok && val != story.CreatedAt.Format("2006-01-02") {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: created is read-only, ignoring change\n")
				}

				// Editable fields — only set if changed.
				if val, ok := fm["status"]; ok && val != string(story.Status) {
					s, err := domain.ParseStatus(val)
					if err != nil {
						return err
					}
					req.Status = &s
				}
				if val, ok := fm["title"]; ok && val != story.Title {
					req.Title = &val
				}
				if val, ok := fm["priority"]; ok && val != priority {
					if val != "" {
						p, err := domain.ParsePriority(val)
						if err != nil {
							return err
						}
						req.Priority = &p
					}
				}
				if val, ok := fm["size"]; ok && val != size {
					if val != "" {
						sz, err := domain.ParseSize(val)
						if err != nil {
							return err
						}
						req.Size = &sz
					}
				}
				if body != story.Description {
					req.Description = &body
				}

				// No changes — nothing to do.
				if req.Status == nil && req.Title == nil && req.Description == nil && req.Priority == nil && req.Size == nil {
					return nil
				}

				// Apply changes — validation errors re-open the editor.
				result, err := setter.SetStory(cmd.Context(), req)
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
