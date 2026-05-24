package driving

import "context"

// GherkinExportRequest is the input shape for the GherkinExporter use
// case. A workspace has a single target, so there are no per-invocation
// target flags; the export covers the whole workspace.
type GherkinExportRequest struct {
	RootDir string // workspace root; empty = cwd
}

// GherkinExportResult summarizes a completed Gherkin export.
type GherkinExportResult struct {
	// FeaturesDir is the absolute path to the rewritten
	// d7/exports/features directory.
	FeaturesDir string
	// Files are the .feature files written, as paths relative to
	// FeaturesDir (e.g. "STORY-047/SPEC-098.feature"), in export order.
	Files []string
	// ScenarioCount is the total number of scenarios exported across all
	// files.
	ScenarioCount int
}

// GherkinExporter is the driving port exposing the "export scenarios to
// Gherkin .feature files" use case. The export is one-way (Clover is the
// source of truth) and rewrites the features directory from scratch on
// every run, pruning files for specs that no longer have scenarios.
type GherkinExporter interface {
	ExportGherkin(ctx context.Context, req GherkinExportRequest) (*GherkinExportResult, error)
}

// Export is the combined driving surface for the export subcommand
// group. It bundles the export use cases so root-command wiring stays
// flat; individual CLI subcommands still take the narrow port they need.
type Export interface {
	GherkinExporter
}
