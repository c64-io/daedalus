package cli

import (
	"github.com/spf13/cobra"

	"github.com/c64-io/daedalus/internal/core/port/driven"
	"github.com/c64-io/daedalus/internal/core/port/driving"
)

// NewRootCmd builds the root `d7` command tree, wiring in the driving
// ports provided by the composition root.
func NewRootCmd(
	initializer driving.WorkspaceInitializer,
	statusReader driving.WorkspaceStatusReader,
	descReader driving.WorkspaceDescriptionReader,
	descWriter driving.WorkspaceDescriptionWriter,
	ideaCreator driving.IdeaCreator,
	ideaReader driving.IdeaReader,
	ideaSetter driving.IdeaSetter,
	epicCreator driving.EpicCreator,
	epicReader driving.EpicReader,
	epicSetter driving.EpicSetter,
	featureCreator driving.FeatureCreator,
	featureReader driving.FeatureReader,
	featureSetter driving.FeatureSetter,
	storyCreator driving.StoryCreator,
	storyReader driving.StoryReader,
	storySetter driving.StorySetter,
	editor driven.Editor,
) *cobra.Command {
	root := &cobra.Command{
		Use:           "d7",
		Short:         "Daedalus (d7) — workspace CLI",
		Long:          "Daedalus (d7) manages local workspaces backed by an embedded Clover database.",
		SilenceUsage:  true,
		SilenceErrors: false,
	}

	root.AddCommand(newInitCmd(initializer))
	root.AddCommand(newWorkspaceCmd(statusReader, descReader, descWriter, editor))
	root.AddCommand(newIdeaCmd(ideaCreator, ideaReader, ideaSetter, editor))
	root.AddCommand(newEpicCmd(epicCreator, epicReader, epicSetter, ideaReader, editor))
	root.AddCommand(newFeatureCmd(featureCreator, featureReader, featureSetter, epicReader, editor))
	root.AddCommand(newStoryCmd(storyCreator, storyReader, storySetter, featureReader, editor))
	return root
}
