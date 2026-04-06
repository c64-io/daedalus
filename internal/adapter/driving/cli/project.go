package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/c64-io/daedalus/internal/core/port"
)

func newProjectCmd(reader port.ProjectDescriptionReader) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "project",
		Short: "Manage the project description",
	}

	cmd.AddCommand(newProjectShowCmd(reader))
	return cmd
}

func newProjectShowCmd(reader port.ProjectDescriptionReader) *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "Show the project description (d7/project.md)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			pd, err := reader.ReadProjectDescription(cmd.Context(), "")
			if err != nil {
				return err
			}

			fmt.Fprint(cmd.OutOrStdout(), pd.Content)

			if pd.IsDefault {
				fmt.Fprintln(cmd.ErrOrStderr(), "hint: edit d7/project.md to describe your product")
			}

			return nil
		},
	}
}
