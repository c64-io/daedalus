package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/c64-io/daedalus/internal/core/port"
)

func newStatusCmd(reader port.WorkspaceStatusReader) *cobra.Command {
	return &cobra.Command{
		Use:   "status [path]",
		Short: "Show the current d7 workspace's location and declared target",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			root := ""
			if len(args) == 1 {
				root = args[0]
			}

			st, err := reader.Status(cmd.Context(), root)
			if err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "workspace: %s\n", st.Workspace.Dir)
			fmt.Fprintf(out, "target:    %s\n", st.Metadata.Target)
			return nil
		},
	}
}
