package cli

import (
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/c64-io/daedalus/internal/core/domain"
	"github.com/c64-io/daedalus/internal/core/port"
)

func newFeatureCmd(creator port.FeatureCreator, reader port.FeatureReader, setter port.FeatureSetter, epicReader port.EpicReader, editor port.Editor) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "feature",
		Short: "Manage features — coherent chunks of capability under an epic",
	}

	cmd.AddCommand(newFeatureNewCmd(creator))
	cmd.AddCommand(newFeatureListCmd(reader))
	cmd.AddCommand(newFeatureShowCmd(reader, epicReader))
	cmd.AddCommand(newFeatureSetCmd(setter))
	cmd.AddCommand(newFeatureEditCmd(reader, setter, editor))
	return cmd
}

func newFeatureNewCmd(creator port.FeatureCreator) *cobra.Command {
	var (
		epicID      string
		title       string
		description string
		priorityRaw string
		sizeRaw     string
		expand      bool
	)

	cmd := &cobra.Command{
		Use:   "new",
		Short: "Create a new feature under an epic",
		RunE: func(cmd *cobra.Command, _ []string) error {
			epicID = strings.ToUpper(strings.TrimSpace(epicID))

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

			feature, err := creator.CreateFeature(cmd.Context(), port.CreateFeatureRequest{
				EpicID:      epicID,
				Title:       title,
				Description: description,
				Priority:    priority,
				Size:        size,
			})
			if err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "created %s: %s (under %s)\n", feature.ID, feature.Title, feature.EpicID)

			if expand {
				fmt.Fprintln(out, "\nAI-assisted expansion is not yet available.")
				fmt.Fprintf(out, "When ready, use: d7 suggest stories --feature %s\n", feature.ID)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&epicID, "epic", "", "parent epic ID (required, e.g. EPIC-001)")
	cmd.Flags().StringVar(&title, "title", "", "feature title (required)")
	cmd.Flags().StringVar(&description, "description", "", "longer prose description (optional)")
	cmd.Flags().StringVar(&priorityRaw, "priority", "", "priority (optional; one of: "+domain.SupportedPriorities()+")")
	cmd.Flags().StringVar(&sizeRaw, "size", "", "size in Fibonacci points (optional; one of: "+domain.SupportedSizes()+")")
	cmd.Flags().BoolVar(&expand, "expand", false,
		"immediately start AI-assisted expansion after creation (coming soon)")
	_ = cmd.MarkFlagRequired("epic")
	_ = cmd.MarkFlagRequired("title")

	return cmd
}

func newFeatureListCmd(reader port.FeatureReader) *cobra.Command {
	var epicID string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List features (all or filtered by epic)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if epicID != "" {
				epicID = strings.ToUpper(strings.TrimSpace(epicID))
			}

			features, err := reader.ListFeatures(cmd.Context(), "", epicID)
			if err != nil {
				return err
			}

			if len(features) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no features yet — create one with: d7 feature new --epic EPIC-001 --title \"...\"")
				return nil
			}

			out := cmd.OutOrStdout()
			w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "ID\tEPIC\tSTATUS\tPRIORITY\tSIZE\tTITLE")
			for _, f := range features {
				priority := "-"
				if f.Priority != "" {
					priority = string(f.Priority)
				}
				size := "-"
				if f.Size != 0 {
					size = fmt.Sprintf("%d", f.Size)
				}
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
					f.ID, f.EpicID, f.Status, priority, size, f.Title)
			}
			return w.Flush()
		},
	}

	cmd.Flags().StringVar(&epicID, "epic", "", "filter by parent epic ID (e.g. EPIC-001)")
	return cmd
}

func newFeatureShowCmd(reader port.FeatureReader, epicReader port.EpicReader) *cobra.Command {
	return &cobra.Command{
		Use:   "show <feature-id>",
		Short: "Show details of a single feature",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := strings.ToUpper(strings.TrimSpace(args[0]))
			feature, err := reader.GetFeature(cmd.Context(), "", id)
			if err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "%s  (%s)\n", feature.ID, feature.Status)
			fmt.Fprintf(out, "Title:       %s\n", feature.Title)

			// Show parent epic info.
			epic, err := epicReader.GetEpic(cmd.Context(), "", feature.EpicID)
			if err == nil {
				fmt.Fprintf(out, "Epic:        %s — %s\n", epic.ID, epic.Title)
			} else {
				fmt.Fprintf(out, "Epic:        %s\n", feature.EpicID)
			}

			if feature.Description != "" {
				fmt.Fprintf(out, "Description: %s\n", feature.Description)
			}
			if feature.Priority != "" {
				fmt.Fprintf(out, "Priority:    %s\n", feature.Priority)
			}
			if feature.Size != 0 {
				fmt.Fprintf(out, "Size:        %d\n", feature.Size)
			}
			fmt.Fprintf(out, "Created:     %s\n", feature.CreatedAt.Format("2006-01-02 15:04:05"))
			return nil
		},
	}
}

func newFeatureSetCmd(setter port.FeatureSetter) *cobra.Command {
	var (
		statusRaw   string
		title       string
		description string
		priorityRaw string
		sizeRaw     string
	)

	cmd := &cobra.Command{
		Use:   "set <feature-id>",
		Short: "Update fields on a feature",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := strings.ToUpper(strings.TrimSpace(args[0]))

			req := port.SetFeatureRequest{ID: id}

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

			feature, err := setter.SetFeature(cmd.Context(), req)
			if err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "%s updated (%s)\n", feature.ID, feature.Status)
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

func newFeatureEditCmd(reader port.FeatureReader, setter port.FeatureSetter, editor port.Editor) *cobra.Command {
	return &cobra.Command{
		Use:   "edit <feature-id>",
		Short: "Edit a feature in $EDITOR",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := strings.ToUpper(strings.TrimSpace(args[0]))
			feature, err := reader.GetFeature(cmd.Context(), "", id)
			if err != nil {
				return err
			}

			priority := ""
			if feature.Priority != "" {
				priority = string(feature.Priority)
			}
			size := ""
			if feature.Size != 0 {
				size = fmt.Sprintf("%d", feature.Size)
			}

			fields := []domain.FrontMatterField{
				{Key: "id", Value: feature.ID, ReadOnly: true},
				{Key: "epic", Value: feature.EpicID, ReadOnly: true},
				{Key: "status", Value: string(feature.Status)},
				{Key: "title", Value: feature.Title},
				{Key: "priority", Value: priority},
				{Key: "size", Value: size},
				{Key: "created", Value: feature.CreatedAt.Format("2006-01-02"), ReadOnly: true},
			}
			initial := domain.FormatFrontMatter(fields, feature.Description)

			var updated *domain.Feature
			_, err = editLoop(editor, initial, func(edited string) error {
				cleaned := stripErrorLines(edited)
				fm, body, parseErr := domain.ParseFrontMatter(cleaned)
				if parseErr != nil {
					return parseErr
				}

				req := port.SetFeatureRequest{ID: feature.ID}

				// Warn about read-only fields.
				if val, ok := fm["id"]; ok && val != feature.ID {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: id is read-only, ignoring change\n")
				}
				if val, ok := fm["epic"]; ok && val != feature.EpicID {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: epic is read-only, ignoring change\n")
				}
				if val, ok := fm["created"]; ok && val != feature.CreatedAt.Format("2006-01-02") {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: created is read-only, ignoring change\n")
				}

				// Editable fields — only set if changed.
				if val, ok := fm["status"]; ok && val != string(feature.Status) {
					s, err := domain.ParseStatus(val)
					if err != nil {
						return err
					}
					req.Status = &s
				}
				if val, ok := fm["title"]; ok && val != feature.Title {
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
				if body != feature.Description {
					req.Description = &body
				}

				// No changes — nothing to do.
				if req.Status == nil && req.Title == nil && req.Description == nil && req.Priority == nil && req.Size == nil {
					return nil
				}

				// Apply changes — validation errors re-open the editor.
				result, err := setter.SetFeature(cmd.Context(), req)
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
