package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/c64-io/daedalus/internal/core/port/driven"
	"github.com/c64-io/daedalus/internal/core/port/driving"
	"github.com/c64-io/daedalus/internal/core/service"
)

// newExpandCmd builds the `d7 expand` command tree. All four
// subcommands share the same interview-first UX; they differ only in
// which entity's Description they rewrite on acceptance.
//
// The single Expander parameter is a parameter-object collapse — the
// subcommand builders each take the narrow port they actually need
// (IdeaExpander / EpicExpander / FeatureExpander / StoryExpander),
// so the interface-segregation discipline is preserved at the real
// call sites.
func newExpandCmd(expander driving.Expander) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "expand",
		Short: "Run AI-assisted expansion on a planning entity",
	}
	cmd.AddCommand(newExpandIdeaCmd(expander))
	cmd.AddCommand(newExpandEpicCmd(expander))
	cmd.AddCommand(newExpandFeatureCmd(expander))
	cmd.AddCommand(newExpandStoryCmd(expander))
	return cmd
}

// aiErrorExit translates the two outcome classes a user cares about
// into the right CLI surface: a clean exit for aborts, a helpful hint
// for a missing API key. Any other error propagates as-is.
func aiErrorExit(cmd *cobra.Command, err error) error {
	if errors.Is(err, service.ErrAIDialogAborted) {
		fmt.Fprintln(cmd.OutOrStdout(), "aborted; nothing was saved.")
		return nil
	}
	if errors.Is(err, driven.ErrAIAuthFailed) {
		return fmt.Errorf("%w (hint: export ANTHROPIC_API_KEY=...)", err)
	}
	return err
}

func newExpandIdeaCmd(expander driving.IdeaExpander) *cobra.Command {
	var model string
	cmd := &cobra.Command{
		Use:   "idea <idea-id>",
		Short: "Expand an Idea's description via an interactive AI dialog",
		Long: `Expand walks through a short interview with the AI to draft the
Idea's description, then shows the proposal for review. You can
accept, critique, edit, or quit at any prompt. Nothing is written to
the database until you accept.

Requires ANTHROPIC_API_KEY to be set in the environment.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := strings.ToUpper(strings.TrimSpace(args[0]))
			idea, err := expander.ExpandIdea(cmd.Context(), driving.ExpandIdeaRequest{
				IdeaID: id,
				Model:  model,
			})
			if err != nil {
				return aiErrorExit(cmd, err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "\n%s description updated.\n", idea.ID)
			return nil
		},
	}
	cmd.Flags().StringVar(&model, "model", "", "override the default model (e.g. claude-opus-4-6)")
	return cmd
}

func newExpandEpicCmd(expander driving.EpicExpander) *cobra.Command {
	var model string
	cmd := &cobra.Command{
		Use:   "epic <epic-id>",
		Short: "Expand an Epic's description via an interactive AI dialog",
		Long: `Expand walks through a short interview with the AI to draft the
Epic's description, then shows the proposal for review. You can
accept, critique, edit, or quit at any prompt. Nothing is written to
the database until you accept.

Requires ANTHROPIC_API_KEY to be set in the environment.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := strings.ToUpper(strings.TrimSpace(args[0]))
			epic, err := expander.ExpandEpic(cmd.Context(), driving.ExpandEpicRequest{
				EpicID: id,
				Model:  model,
			})
			if err != nil {
				return aiErrorExit(cmd, err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "\n%s description updated.\n", epic.ID)
			return nil
		},
	}
	cmd.Flags().StringVar(&model, "model", "", "override the default model (e.g. claude-opus-4-6)")
	return cmd
}

func newExpandFeatureCmd(expander driving.FeatureExpander) *cobra.Command {
	var model string
	cmd := &cobra.Command{
		Use:   "feature <feature-id>",
		Short: "Expand a Feature's description via an interactive AI dialog",
		Long: `Expand walks through a short interview with the AI to draft the
Feature's description, then shows the proposal for review. You can
accept, critique, edit, or quit at any prompt. Nothing is written to
the database until you accept.

Requires ANTHROPIC_API_KEY to be set in the environment.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := strings.ToUpper(strings.TrimSpace(args[0]))
			feat, err := expander.ExpandFeature(cmd.Context(), driving.ExpandFeatureRequest{
				FeatureID: id,
				Model:     model,
			})
			if err != nil {
				return aiErrorExit(cmd, err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "\n%s description updated.\n", feat.ID)
			return nil
		},
	}
	cmd.Flags().StringVar(&model, "model", "", "override the default model (e.g. claude-opus-4-6)")
	return cmd
}

func newExpandStoryCmd(expander driving.StoryExpander) *cobra.Command {
	var model string
	cmd := &cobra.Command{
		Use:   "story <story-id>",
		Short: "Expand a Story's description via an interactive AI dialog",
		Long: `Expand walks through a short interview with the AI to draft the
Story's description, then shows the proposal for review. You can
accept, critique, edit, or quit at any prompt. Nothing is written to
the database until you accept.

Requires ANTHROPIC_API_KEY to be set in the environment.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := strings.ToUpper(strings.TrimSpace(args[0]))
			story, err := expander.ExpandStory(cmd.Context(), driving.ExpandStoryRequest{
				StoryID: id,
				Model:   model,
			})
			if err != nil {
				return aiErrorExit(cmd, err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "\n%s description updated.\n", story.ID)
			return nil
		},
	}
	cmd.Flags().StringVar(&model, "model", "", "override the default model (e.g. claude-opus-4-6)")
	return cmd
}
