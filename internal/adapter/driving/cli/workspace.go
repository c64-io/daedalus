package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/c64-io/daedalus/internal/core/domain"
	"github.com/c64-io/daedalus/internal/core/port"
)

func newWorkspaceCmd(
	statusReader port.WorkspaceStatusReader,
	descReader port.WorkspaceDescriptionReader,
	descWriter port.WorkspaceDescriptionWriter,
	editor port.Editor,
) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "workspace",
		Short: "Manage the d7 workspace",
	}

	cmd.AddCommand(newWorkspaceStatusCmd(statusReader))
	cmd.AddCommand(newWorkspaceShowCmd(descReader))
	cmd.AddCommand(newWorkspaceEditCmd(descReader, descWriter, statusReader, editor))
	return cmd
}

func newWorkspaceStatusCmd(reader port.WorkspaceStatusReader) *cobra.Command {
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

			if pd := st.ProjectDescription; pd != nil {
				if pd.IsDefault {
					fmt.Fprintln(out, "project:   (default template — edit to describe your product)")
				} else {
					fmt.Fprintln(out, "project:   (customized)")
				}
			}

			return nil
		},
	}
}

func newWorkspaceShowCmd(reader port.WorkspaceDescriptionReader) *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "Show the project description",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			pd, err := reader.ReadWorkspaceDescription(cmd.Context(), "")
			if err != nil {
				return err
			}

			fmt.Fprint(cmd.OutOrStdout(), pd.Content)

			if pd.IsDefault {
				fmt.Fprintln(cmd.ErrOrStderr(), "hint: run `d7 workspace edit` to describe your product")
			}

			return nil
		},
	}
}

func newWorkspaceEditCmd(
	descReader port.WorkspaceDescriptionReader,
	descWriter port.WorkspaceDescriptionWriter,
	statusReader port.WorkspaceStatusReader,
	editor port.Editor,
) *cobra.Command {
	return &cobra.Command{
		Use:   "edit",
		Short: "Edit the project description in $EDITOR",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			st, err := statusReader.Status(cmd.Context(), "")
			if err != nil {
				return err
			}

			pd, err := descReader.ReadWorkspaceDescription(cmd.Context(), "")
			if err != nil {
				return err
			}

			fields := []domain.FrontMatterField{
				{Key: "target", Value: string(st.Metadata.Target), ReadOnly: true},
			}
			initial := domain.FormatFrontMatter(fields, pd.Content)

			var newBody string
			_, err = editLoop(editor, initial, func(edited string) error {
				// Strip any leading # ERROR lines from re-opened content.
				cleaned := edited
				for strings.HasPrefix(cleaned, "# ERROR:") {
					if idx := strings.Index(cleaned, "\n"); idx >= 0 {
						cleaned = cleaned[idx+1:]
					} else {
						break
					}
				}

				fm, body, parseErr := domain.ParseFrontMatter(cleaned)
				if parseErr != nil {
					return parseErr
				}

				// Warn about read-only field changes (target).
				if val, ok := fm["target"]; ok && val != string(st.Metadata.Target) {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: target is read-only, ignoring change %q → %q\n",
						st.Metadata.Target, val)
				}

				newBody = body
				return nil
			})
			if err != nil {
				return err
			}

			if err := descWriter.WriteWorkspaceDescription(cmd.Context(), "", newBody); err != nil {
				return err
			}

			fmt.Fprintln(cmd.OutOrStdout(), "workspace description updated")
			return nil
		},
	}
}
