package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/c64-io/daedalus/internal/core/domain"
	"github.com/c64-io/daedalus/internal/core/port"
)

func newInitCmd(initializer port.WorkspaceInitializer) *cobra.Command {
	var langs []string

	cmd := &cobra.Command{
		Use:   "init [path]",
		Short: "Initialize a d7 workspace in the given directory (default: cwd)",
		Long: "Initialize a d7 workspace in the given directory (default: cwd). " +
			"The --lang flag declares the target code-generation ecosystem(s) for " +
			"the workspace and is immutable for the life of the project. Supported " +
			"targets: " + domain.SupportedTargets() + ".",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			root := ""
			if len(args) == 1 {
				root = args[0]
			}

			targets, err := domain.ParseTargets(langs)
			if err != nil {
				return err
			}

			ws, err := initializer.Init(cmd.Context(), port.InitRequest{
				RootDir: root,
				Targets: targets,
			})
			if err != nil {
				return err
			}

			names := make([]string, len(targets))
			for i, t := range targets {
				names[i] = string(t)
			}
			fmt.Fprintf(cmd.OutOrStdout(),
				"initialized d7 workspace at %s (targets: %s)\n",
				ws.Dir, strings.Join(names, ", "))
			return nil
		},
	}

	cmd.Flags().StringSliceVar(&langs, "lang", nil,
		"target language(s) for code generation; repeatable or comma-separated "+
			"(e.g. --lang go, --lang go,typescript). Required.")
	_ = cmd.MarkFlagRequired("lang")

	return cmd
}
