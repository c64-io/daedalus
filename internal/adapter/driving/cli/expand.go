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

// newExpandCmd builds the `d7 expand` command tree. v1 has one
// subcommand — `expand story <id>` — and the tree is laid out this
// way so refine/generate/etc. can slot in alongside it without a
// second top-level command name.
func newExpandCmd(expander driving.StoryExpander) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "expand",
		Short: "Run AI-assisted expansion on a planning entity",
	}
	cmd.AddCommand(newExpandStoryCmd(expander))
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
				// Aborting is a clean exit, not a failure.
				if errors.Is(err, service.ErrAIDialogAborted) {
					fmt.Fprintln(cmd.OutOrStdout(), "aborted; nothing was saved.")
					return nil
				}
				if errors.Is(err, driven.ErrAIAuthFailed) {
					return fmt.Errorf("%w (hint: export ANTHROPIC_API_KEY=...)", err)
				}
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "\n%s description updated.\n", story.ID)
			return nil
		},
	}

	cmd.Flags().StringVar(&model, "model", "", "override the default model (e.g. claude-opus-4-6)")

	return cmd
}
