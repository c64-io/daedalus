package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/c64-io/daedalus/internal/core/port/driving"
)

// newSuggestCmd builds the `d7 suggest` command tree. Each subcommand
// asks the AI to propose several new child entities under an existing
// parent and lets the founder pick which proposals to keep.
//
// The single Suggester parameter is a parameter-object collapse at the
// wiring seam; each subcommand builder below narrows down to the exact
// port it implements.
func newSuggestCmd(suggester driving.Suggester) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "suggest",
		Short: "Run AI-assisted bulk drafting of child entities",
	}
	cmd.AddCommand(newSuggestEpicsCmd(suggester))
	cmd.AddCommand(newSuggestFeaturesCmd(suggester))
	cmd.AddCommand(newSuggestStoriesCmd(suggester))
	cmd.AddCommand(newSuggestSpecsCmd(suggester))
	cmd.AddCommand(newSuggestScenariosCmd(suggester))
	return cmd
}

func newSuggestEpicsCmd(suggester driving.EpicsSuggester) *cobra.Command {
	var (
		ideaID string
		model  string
	)
	cmd := &cobra.Command{
		Use:   "epics",
		Short: "Propose several Epics under a parent Idea",
		Long: `Suggest walks through a short interview with the AI, then shows a
batch of proposed Epics. Pick any subset to accept (a, n, or a
comma-list like 1,3,5), critique the whole batch, edit the list in
$EDITOR, or quit. Accepted items are created as new Epics; rejected
ones vanish.

Requires ANTHROPIC_API_KEY to be set in the environment.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			id := strings.ToUpper(strings.TrimSpace(ideaID))
			if id == "" {
				return fmt.Errorf("--idea is required")
			}
			created, err := suggester.SuggestEpics(cmd.Context(), driving.SuggestEpicsRequest{
				IdeaID: id,
				Model:  model,
			})
			if err != nil {
				return aiErrorExit(cmd, err)
			}
			printCreatedCount(cmd.OutOrStdout(), len(created), "Epic")
			for _, e := range created {
				fmt.Fprintf(cmd.OutOrStdout(), "  %s  %s\n", e.ID, e.Title)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&ideaID, "idea", "", "parent Idea ID (required, e.g. IDEA-001)")
	cmd.Flags().StringVar(&model, "model", "", "override the default model (e.g. claude-opus-4-6)")
	_ = cmd.MarkFlagRequired("idea")
	return cmd
}

func newSuggestFeaturesCmd(suggester driving.FeaturesSuggester) *cobra.Command {
	var (
		epicID string
		model  string
	)
	cmd := &cobra.Command{
		Use:   "features",
		Short: "Propose several Features under a parent Epic",
		Long: `Suggest walks through a short interview with the AI, then shows a
batch of proposed Features. Pick any subset to accept (a, n, or a
comma-list like 1,3,5), critique the whole batch, edit the list in
$EDITOR, or quit. Accepted items are created as new Features;
rejected ones vanish.

Requires ANTHROPIC_API_KEY to be set in the environment.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			id := strings.ToUpper(strings.TrimSpace(epicID))
			if id == "" {
				return fmt.Errorf("--epic is required")
			}
			created, err := suggester.SuggestFeatures(cmd.Context(), driving.SuggestFeaturesRequest{
				EpicID: id,
				Model:  model,
			})
			if err != nil {
				return aiErrorExit(cmd, err)
			}
			printCreatedCount(cmd.OutOrStdout(), len(created), "Feature")
			for _, f := range created {
				fmt.Fprintf(cmd.OutOrStdout(), "  %s  %s\n", f.ID, f.Title)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&epicID, "epic", "", "parent Epic ID (required, e.g. EPIC-001)")
	cmd.Flags().StringVar(&model, "model", "", "override the default model (e.g. claude-opus-4-6)")
	_ = cmd.MarkFlagRequired("epic")
	return cmd
}

func newSuggestStoriesCmd(suggester driving.StoriesSuggester) *cobra.Command {
	var (
		featureID string
		model     string
	)
	cmd := &cobra.Command{
		Use:   "stories",
		Short: "Propose several Stories under a parent Feature",
		Long: `Suggest walks through a short interview with the AI, then shows a
batch of proposed Stories. Pick any subset to accept (a, n, or a
comma-list like 1,3,5), critique the whole batch, edit the list in
$EDITOR, or quit. Accepted items are created as new Stories;
rejected ones vanish.

Requires ANTHROPIC_API_KEY to be set in the environment.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			id := strings.ToUpper(strings.TrimSpace(featureID))
			if id == "" {
				return fmt.Errorf("--feature is required")
			}
			created, err := suggester.SuggestStories(cmd.Context(), driving.SuggestStoriesRequest{
				FeatureID: id,
				Model:     model,
			})
			if err != nil {
				return aiErrorExit(cmd, err)
			}
			printCreatedCount(cmd.OutOrStdout(), len(created), "Story")
			for _, s := range created {
				fmt.Fprintf(cmd.OutOrStdout(), "  %s  %s\n", s.ID, s.Title)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&featureID, "feature", "", "parent Feature ID (required, e.g. FEAT-001)")
	cmd.Flags().StringVar(&model, "model", "", "override the default model (e.g. claude-opus-4-6)")
	_ = cmd.MarkFlagRequired("feature")
	return cmd
}

func newSuggestSpecsCmd(suggester driving.SpecsSuggester) *cobra.Command {
	var (
		storyID string
		model   string
	)
	cmd := &cobra.Command{
		Use:   "specs",
		Short: "Propose several Specs under a parent Story",
		Long: `Suggest walks through a short interview with the AI, then shows a
batch of proposed Specs. Pick any subset to accept (a, n, or a
comma-list like 1,3,5), critique the whole batch, edit the list in
$EDITOR, or quit. Accepted items are created as new Specs;
rejected ones vanish.

Requires ANTHROPIC_API_KEY to be set in the environment.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			id := strings.ToUpper(strings.TrimSpace(storyID))
			if id == "" {
				return fmt.Errorf("--story is required")
			}
			created, err := suggester.SuggestSpecs(cmd.Context(), driving.SuggestSpecsRequest{
				StoryID: id,
				Model:   model,
			})
			if err != nil {
				return aiErrorExit(cmd, err)
			}
			printCreatedCount(cmd.OutOrStdout(), len(created), "Spec")
			for _, s := range created {
				fmt.Fprintf(cmd.OutOrStdout(), "  %s  %s\n", s.ID, s.Title)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&storyID, "story", "", "parent Story ID (required, e.g. STORY-001)")
	cmd.Flags().StringVar(&model, "model", "", "override the default model (e.g. claude-opus-4-6)")
	_ = cmd.MarkFlagRequired("story")
	return cmd
}

func newSuggestScenariosCmd(suggester driving.ScenariosSuggester) *cobra.Command {
	var (
		specID string
		model  string
	)
	cmd := &cobra.Command{
		Use:   "scenarios",
		Short: "Propose several Scenarios under a parent Spec",
		Long: `Suggest walks through a short interview with the AI, then shows a
batch of proposed Scenarios with Given/When/Then steps. Pick any
subset to accept (a, n, or a comma-list like 1,3,5), critique the
whole batch, edit the list in $EDITOR, or quit. Accepted items are
created as new Scenarios; rejected ones vanish.

Requires ANTHROPIC_API_KEY to be set in the environment.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			id := strings.ToUpper(strings.TrimSpace(specID))
			if id == "" {
				return fmt.Errorf("--spec is required")
			}
			created, err := suggester.SuggestScenarios(cmd.Context(), driving.SuggestScenariosRequest{
				SpecID: id,
				Model:  model,
			})
			if err != nil {
				return aiErrorExit(cmd, err)
			}
			printCreatedCount(cmd.OutOrStdout(), len(created), "Scenario")
			for _, s := range created {
				fmt.Fprintf(cmd.OutOrStdout(), "  %s  %s\n", s.ID, s.Title)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&specID, "spec", "", "parent Spec ID (required, e.g. SPEC-001)")
	cmd.Flags().StringVar(&model, "model", "", "override the default model (e.g. claude-opus-4-6)")
	_ = cmd.MarkFlagRequired("spec")
	return cmd
}

// printCreatedCount prints the header line above the created-items
// listing. Zero means "nothing was saved" — which is the abort path's
// normal output when the founder picked `n` or declined every item.
func printCreatedCount(w io.Writer, n int, label string) {
	if n == 0 {
		fmt.Fprintln(w, "nothing was saved.")
		return
	}
	suffix := "s"
	if n == 1 {
		suffix = ""
	}
	fmt.Fprintf(w, "\ncreated %d %s%s:\n", n, label, suffix)
}
