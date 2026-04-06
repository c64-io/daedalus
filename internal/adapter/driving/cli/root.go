package cli

import (
	"github.com/spf13/cobra"

	"github.com/c64-io/daedalus/internal/core/port"
)

// NewRootCmd builds the root `d7` command tree, wiring in the driving
// ports provided by the composition root.
func NewRootCmd(
	initializer port.WorkspaceInitializer,
	statusReader port.WorkspaceStatusReader,
	descReader port.WorkspaceDescriptionReader,
	descWriter port.WorkspaceDescriptionWriter,
	ideaCreator port.IdeaCreator,
	ideaReader port.IdeaReader,
	ideaSetter port.IdeaSetter,
	epicCreator port.EpicCreator,
	epicReader port.EpicReader,
	epicSetter port.EpicSetter,
	editor port.Editor,
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
	return root
}
