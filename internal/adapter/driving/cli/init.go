package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/c64-io/daedalus/internal/core/domain"
	"github.com/c64-io/daedalus/internal/core/port/driving"
)

func newInitCmd(initializer driving.WorkspaceInitializer) *cobra.Command {
	var lang string

	cmd := &cobra.Command{
		Use:   "init [path]",
		Short: "Initialize a d7 workspace in the given directory (default: cwd)",
		Long: "Initialize a d7 workspace in the given directory (default: cwd). " +
			"The --lang flag declares the workspace's target code-generation " +
			"ecosystem and is immutable for the life of the project. Supported " +
			"targets: " + domain.SupportedTargets() + ".",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			root := ""
			if len(args) == 1 {
				root = args[0]
			}

			target, err := domain.ParseTarget(lang)
			if err != nil {
				return err
			}

			ws, err := initializer.Init(cmd.Context(), driving.InitRequest{
				RootDir: root,
				Target:  target,
			})
			if err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(),
				"initialized d7 workspace at %s (target: %s)\n",
				ws.Dir, target)
			return nil
		},
	}

	cmd.Flags().StringVar(&lang, "lang", "",
		"target language for code generation (required; one of: "+
			domain.SupportedTargets()+")")
	_ = cmd.MarkFlagRequired("lang")

	return cmd
}
