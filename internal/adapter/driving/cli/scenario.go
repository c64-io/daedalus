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

func newScenarioCmd(creator driving.ScenarioCreator, reader driving.ScenarioReader, setter driving.ScenarioSetter, specReader driving.SpecReader, editor driven.Editor) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "scenario",
		Short: "Manage scenarios — Given/When/Then steps attached to a spec",
	}

	cmd.AddCommand(newScenarioNewCmd(creator))
	cmd.AddCommand(newScenarioListCmd(reader))
	cmd.AddCommand(newScenarioShowCmd(reader, specReader))
	cmd.AddCommand(newScenarioSetCmd(setter))
	cmd.AddCommand(newScenarioEditCmd(reader, setter, editor))
	return cmd
}

func newScenarioNewCmd(creator driving.ScenarioCreator) *cobra.Command {
	var (
		specID   string
		title    string
		givenRaw []string
		whenRaw  []string
		thenRaw  []string
		tagsRaw  []string
	)

	cmd := &cobra.Command{
		Use:   "new",
		Short: "Create a new scenario under a spec",
		Long: `Create a new scenario with optional plain-text Given/When/Then steps.

For quick scripted creation, pass repeatable --given/--when/--then flags
with plain-text step content. For richer authoring (data tables, doc
strings, reordering), use "d7 scenario edit <id>" which opens a
structured YAML document in $EDITOR.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			specID = strings.ToUpper(strings.TrimSpace(specID))

			scen, err := creator.CreateScenario(cmd.Context(), driving.CreateScenarioRequest{
				SpecID: specID,
				Title:  title,
				Tags:   normalizeCLITags(tagsRaw),
				Given:  plainSteps(givenRaw),
				When:   plainSteps(whenRaw),
				Then:   plainSteps(thenRaw),
			})
			if err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "created %s: %s (under %s)\n", scen.ID, scen.Title, scen.SpecID)
			return nil
		},
	}

	cmd.Flags().StringVar(&specID, "spec", "", "parent spec ID (required, e.g. SPEC-001)")
	cmd.Flags().StringVar(&title, "title", "", "scenario title (required)")
	cmd.Flags().StringArrayVar(&givenRaw, "given", nil, "a Given step (repeatable)")
	cmd.Flags().StringArrayVar(&whenRaw, "when", nil, "a When step (repeatable)")
	cmd.Flags().StringArrayVar(&thenRaw, "then", nil, "a Then step (repeatable)")
	cmd.Flags().StringArrayVar(&tagsRaw, "tag", nil, "a user tag, without leading @ (repeatable)")
	_ = cmd.MarkFlagRequired("spec")
	_ = cmd.MarkFlagRequired("title")

	return cmd
}

func newScenarioListCmd(reader driving.ScenarioReader) *cobra.Command {
	var specID string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List scenarios (all or filtered by spec)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if specID != "" {
				specID = strings.ToUpper(strings.TrimSpace(specID))
			}

			scens, err := reader.ListScenarios(cmd.Context(), "", specID)
			if err != nil {
				return err
			}

			if len(scens) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no scenarios yet — create one with: d7 scenario new --spec SPEC-001 --title \"...\"")
				return nil
			}

			out := cmd.OutOrStdout()
			w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "ID\tSPEC\tSTATUS\tSTEPS\tTITLE")
			for _, s := range scens {
				steps := fmt.Sprintf("%dG/%dW/%dT", len(s.Given), len(s.When), len(s.Then))
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
					s.ID, s.SpecID, s.Status, steps, s.Title)
			}
			return w.Flush()
		},
	}

	cmd.Flags().StringVar(&specID, "spec", "", "filter by parent spec ID (e.g. SPEC-001)")
	return cmd
}

func newScenarioShowCmd(reader driving.ScenarioReader, specReader driving.SpecReader) *cobra.Command {
	return &cobra.Command{
		Use:   "show <scenario-id>",
		Short: "Show details of a single scenario, with rendered Gherkin",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := strings.ToUpper(strings.TrimSpace(args[0]))
			scen, err := reader.GetScenario(cmd.Context(), "", id)
			if err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "%s  (%s)\n", scen.ID, scen.Status)
			fmt.Fprintf(out, "Title:   %s\n", scen.Title)

			// Show parent spec info.
			spec, err := specReader.GetSpec(cmd.Context(), "", scen.SpecID)
			if err == nil {
				fmt.Fprintf(out, "Spec:    %s — %s\n", spec.ID, spec.Title)
			} else {
				fmt.Fprintf(out, "Spec:    %s\n", scen.SpecID)
			}

			if len(scen.Tags) > 0 {
				fmt.Fprintf(out, "Tags:    %s\n", strings.Join(scen.Tags, ", "))
			}
			fmt.Fprintf(out, "Created: %s\n", scen.CreatedAt.Format("2006-01-02 15:04:05"))

			fmt.Fprintln(out)
			fmt.Fprint(out, domain.FormatScenarioAsGherkin(*scen))
			return nil
		},
	}
}

func newScenarioSetCmd(setter driving.ScenarioSetter) *cobra.Command {
	var (
		statusRaw string
		title     string
		tagsRaw   []string
	)

	cmd := &cobra.Command{
		Use:   "set <scenario-id>",
		Short: "Update scalar fields on a scenario (status, title, tags)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := strings.ToUpper(strings.TrimSpace(args[0]))

			req := driving.SetScenarioRequest{ID: id}

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
			if cmd.Flags().Changed("tag") {
				normalized := normalizeCLITags(tagsRaw)
				req.Tags = &normalized
			}

			if req.Status == nil && req.Title == nil && req.Tags == nil {
				fmt.Fprintln(cmd.OutOrStdout(), "available flags: --status, --title, --tag")
				fmt.Fprintln(cmd.OutOrStdout(), "to edit steps, use: d7 scenario edit <id>")
				return nil
			}

			scen, err := setter.SetScenario(cmd.Context(), req)
			if err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "%s updated (%s)\n", scen.ID, scen.Status)
			return nil
		},
	}

	cmd.Flags().StringVar(&statusRaw, "status", "", "new status (draft, refined, ready, in-progress, review, done, archived, blocked)")
	cmd.Flags().StringVar(&title, "title", "", "new title")
	cmd.Flags().StringArrayVar(&tagsRaw, "tag", nil, "replace tag list; repeat for multiple tags")

	return cmd
}

func newScenarioEditCmd(reader driving.ScenarioReader, setter driving.ScenarioSetter, editor driven.Editor) *cobra.Command {
	return &cobra.Command{
		Use:   "edit <scenario-id>",
		Short: "Edit a scenario as a structured YAML document in $EDITOR",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := strings.ToUpper(strings.TrimSpace(args[0]))
			scen, err := reader.GetScenario(cmd.Context(), "", id)
			if err != nil {
				return err
			}

			initial, err := domain.FormatScenarioYAML(*scen)
			if err != nil {
				return err
			}

			var updated *domain.Scenario
			_, err = editLoop(editor, initial, func(edited string) error {
				cleaned := stripErrorLines(edited)
				parsed, parseErr := domain.ParseScenarioYAML(cleaned)
				if parseErr != nil {
					return parseErr
				}

				req := driving.SetScenarioRequest{ID: scen.ID}

				if parsed.Status != "" && parsed.Status != string(scen.Status) {
					s, err := domain.ParseStatus(parsed.Status)
					if err != nil {
						return err
					}
					req.Status = &s
				}
				if parsed.Title != "" && parsed.Title != scen.Title {
					req.Title = &parsed.Title
				}
				if !tagsStructurallyEqual(parsed.Tags, scen.Tags) {
					tags := parsed.Tags
					req.Tags = &tags
				}
				if !stepsStructurallyEqual(parsed.Given, scen.Given) {
					given := parsed.Given
					req.Given = &given
				}
				if !stepsStructurallyEqual(parsed.When, scen.When) {
					when := parsed.When
					req.When = &when
				}
				if !stepsStructurallyEqual(parsed.Then, scen.Then) {
					then := parsed.Then
					req.Then = &then
				}

				// No changes — nothing to do.
				if req.Status == nil && req.Title == nil && req.Tags == nil &&
					req.Given == nil && req.When == nil && req.Then == nil {
					return nil
				}

				result, err := setter.SetScenario(cmd.Context(), req)
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

// plainSteps turns a list of raw CLI step strings into domain
// Steps. Blank entries are skipped; no data tables or doc strings
// are supported via flags.
func plainSteps(raw []string) []domain.Step {
	if len(raw) == 0 {
		return nil
	}
	out := make([]domain.Step, 0, len(raw))
	for _, r := range raw {
		r = strings.TrimSpace(r)
		if r == "" {
			continue
		}
		out = append(out, domain.Step{Text: r})
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// normalizeCLITags trims whitespace and strips any leading @ sign
// from user-provided tag flags so the stored form stays canonical.
func normalizeCLITags(raw []string) []string {
	if len(raw) == 0 {
		return nil
	}
	out := make([]string, 0, len(raw))
	for _, t := range raw {
		t = strings.TrimSpace(t)
		t = strings.TrimPrefix(t, "@")
		if t != "" {
			out = append(out, t)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// tagsStructurallyEqual reports whether two tag slices are
// equal, treating nil and empty as equivalent.
func tagsStructurallyEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// stepsStructurallyEqual reports whether two step slices match on
// text, data table, and doc string.
func stepsStructurallyEqual(a, b []domain.Step) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Text != b[i].Text {
			return false
		}
		if !docStringsEqual(a[i].DocString, b[i].DocString) {
			return false
		}
		if !dataTablesEqual(a[i].DataTable, b[i].DataTable) {
			return false
		}
	}
	return true
}

func docStringsEqual(a, b *string) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

func dataTablesEqual(a, b *domain.DataTable) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	if !tagsStructurallyEqual(a.Headers, b.Headers) {
		return false
	}
	if len(a.Rows) != len(b.Rows) {
		return false
	}
	for i := range a.Rows {
		if !tagsStructurallyEqual(a.Rows[i], b.Rows[i]) {
			return false
		}
	}
	return true
}
