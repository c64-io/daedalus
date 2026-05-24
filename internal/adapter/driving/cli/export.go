package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/c64-io/daedalus/internal/core/port/driving"
)

func newExportCmd(export driving.Export) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export d7 data to external formats",
	}
	cmd.AddCommand(newExportGherkinCmd(export))
	return cmd
}

func newExportGherkinCmd(exporter driving.GherkinExporter) *cobra.Command {
	return &cobra.Command{
		Use:   "gherkin",
		Short: "Export scenarios to Gherkin .feature files under d7/exports/features/",
		Long: `Render every scenario in the workspace to Gherkin .feature files.

One file is written per spec (grouped by parent story) under
d7/exports/features/<story>/<spec>.feature. Each scenario is tagged with
its stable @d7:<SCEN-ID>, and each feature file inherits its ancestor
tags (@d7:EPIC / @d7:FEAT / @d7:STORY / @d7:SPEC) so a runner can select
a whole subtree by tag. These files are generated artifacts: Clover is
the source of truth, the features directory is rewritten on every run,
and files for specs with no scenarios are pruned.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			res, err := exporter.ExportGherkin(cmd.Context(), driving.GherkinExportRequest{})
			if err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			if len(res.Files) == 0 {
				fmt.Fprintln(out, "no scenarios to export — add scenarios first with: d7 scenario new --spec SPEC-001 --title \"...\"")
				return nil
			}

			fmt.Fprintf(out, "exported %d scenario(s) across %d feature file(s) to %s\n",
				res.ScenarioCount, len(res.Files), res.FeaturesDir)
			for _, f := range res.Files {
				fmt.Fprintf(out, "  %s\n", f)
			}
			return nil
		},
	}
}
