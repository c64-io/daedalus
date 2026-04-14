package cli

import (
	"github.com/spf13/cobra"

	"github.com/c64-io/daedalus/internal/core/port/driven"
	"github.com/c64-io/daedalus/internal/core/port/driving"
)

// NewRootCmd builds the root `d7` command tree, wiring in the driving
// ports provided by the composition root.
//
// Each subtree takes a single bundled interface (e.g. driving.Idea
// embeds IdeaCreator+IdeaReader+IdeaSetter) so the root signature
// stays flat. Individual CLI subcommand builders still narrow down to
// the specific verb they implement, which preserves interface
// segregation at the actual call sites.
func NewRootCmd(
	ws driving.Workspace,
	idea driving.Idea,
	epic driving.Epic,
	feature driving.Feature,
	story driving.Story,
	spec driving.Spec,
	scenario driving.Scenario,
	link driving.Link,
	ref driving.Ref,
	expander driving.Expander,
	editor driven.Editor,
) *cobra.Command {
	root := &cobra.Command{
		Use:           "d7",
		Short:         "Daedalus (d7) — workspace CLI",
		Long:          "Daedalus (d7) manages local workspaces backed by an embedded Clover database.",
		SilenceUsage:  true,
		SilenceErrors: false,
	}

	root.AddCommand(newInitCmd(ws))
	root.AddCommand(newWorkspaceCmd(ws, editor))
	root.AddCommand(newIdeaCmd(idea, link, ref, editor))
	root.AddCommand(newEpicCmd(epic, idea, link, ref, editor))
	root.AddCommand(newFeatureCmd(feature, epic, link, ref, editor))
	root.AddCommand(newStoryCmd(story, feature, link, ref, editor))
	root.AddCommand(newSpecCmd(spec, story, link, ref, editor))
	root.AddCommand(newScenarioCmd(scenario, spec, link, ref, editor))
	root.AddCommand(newLinkCmd(link))
	root.AddCommand(newRefCmd(ref))
	root.AddCommand(newExpandCmd(expander))
	return root
}
