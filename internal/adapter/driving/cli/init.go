package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/c64-io/daedalus/internal/core/port"
)

func newInitCmd(initializer port.WorkspaceInitializer) *cobra.Command {
	return &cobra.Command{
		Use:   "init [path]",
		Short: "Initialize a d7 workspace in the given directory (default: cwd)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			root := "."
			if len(args) == 1 {
				root = args[0]
			}
			if root == "." {
				cwd, err := os.Getwd()
				if err != nil {
					return fmt.Errorf("resolve cwd: %w", err)
				}
				root = cwd
			}

			ws, err := initializer.Init(cmd.Context(), root)
			if err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "initialized d7 workspace at %s\n", ws.Dir)
			return nil
		},
	}
}
