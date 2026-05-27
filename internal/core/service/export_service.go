package service

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"

	"github.com/c64-io/daedalus/internal/core/domain"
	"github.com/c64-io/daedalus/internal/core/port/driven"
	"github.com/c64-io/daedalus/internal/core/port/driving"
)

// Compile-time assertion that ExportService satisfies its driving port.
var _ driving.GherkinExporter = (*ExportService)(nil)

// ExportService implements the GherkinExporter use case: it renders the
// workspace's scenarios to Gherkin .feature files under
// d7/exports/features/, one file per Spec, grouped by parent Story.
//
// The export is one-way — Clover is the source of truth, so nothing is
// read back — and rewrites the features directory from scratch on every
// run so that files for archived or emptied Specs are pruned rather than
// left to drift.
type ExportService struct {
	fs           driven.FileSystem
	storyRepo    driven.StoryRepository
	specRepo     driven.SpecRepository
	scenarioRepo driven.ScenarioRepository
	featureRepo  driven.FeatureRepository
	epicRepo     driven.EpicRepository
}

// NewExportService wires the service with its driven dependencies.
func NewExportService(
	fs driven.FileSystem,
	storyRepo driven.StoryRepository,
	specRepo driven.SpecRepository,
	scenarioRepo driven.ScenarioRepository,
	featureRepo driven.FeatureRepository,
	epicRepo driven.EpicRepository,
) *ExportService {
	return &ExportService{
		fs:           fs,
		storyRepo:    storyRepo,
		specRepo:     specRepo,
		scenarioRepo: scenarioRepo,
		featureRepo:  featureRepo,
		epicRepo:     epicRepo,
	}
}

// ExportGherkin gathers every Spec that has at least one Scenario,
// renders each to a .feature file with its mandatory @d7:<ID> tags, and
// writes the result under d7/exports/features/. The features directory
// is removed and recreated first so stale files are pruned. All content
// is rendered in memory before any filesystem mutation, so a read error
// leaves the previous export untouched.
func (s *ExportService) ExportGherkin(ctx context.Context, req driving.GherkinExportRequest) (*driving.GherkinExportResult, error) {
	ws, err := requireWorkspace(s.fs, req.RootDir)
	if err != nil {
		return nil, err
	}

	specs, err := s.specRepo.ListSpecs(ctx, ws.DBDir, "")
	if err != nil {
		return nil, fmt.Errorf("list specs: %w", err)
	}
	scenarios, err := s.scenarioRepo.ListScenarios(ctx, ws.DBDir, "")
	if err != nil {
		return nil, fmt.Errorf("list scenarios: %w", err)
	}

	scenariosBySpec := make(map[string][]domain.Scenario, len(specs))
	for _, sc := range scenarios {
		scenariosBySpec[sc.SpecID] = append(scenariosBySpec[sc.SpecID], sc)
	}

	// Order the export by ID rather than trusting repository order:
	// IDs are zero-padded and monotonic by creation, so lexicographic
	// order is creation order. This keeps re-exports byte-stable (the
	// files are committed, so diffs must be deterministic) regardless of
	// how the store happens to return rows.
	sort.Slice(specs, func(i, j int) bool { return specs[i].ID < specs[j].ID })
	for id := range scenariosBySpec {
		group := scenariosBySpec[id]
		sort.Slice(group, func(i, j int) bool { return group[i].ID < group[j].ID })
	}

	storyByID, featureByID, epicByID, err := s.lineageIndex(ctx, ws.DBDir)
	if err != nil {
		return nil, err
	}

	// Render every non-empty Spec in memory before touching disk.
	type featureFile struct {
		rel     string
		content string
		count   int
	}
	var files []featureFile
	for _, spec := range specs {
		specScenarios := scenariosBySpec[spec.ID]
		if len(specScenarios) == 0 {
			continue
		}
		tags := featureTags(spec, storyByID, featureByID, epicByID)
		files = append(files, featureFile{
			rel:     filepath.Join(spec.StoryID, spec.ID+".feature"),
			content: domain.FormatFeatureFile(spec, specScenarios, tags),
			count:   len(specScenarios),
		})
	}

	featuresDir := filepath.Join(ws.Dir, "exports", "features")
	if err := s.fs.RemoveAll(featuresDir); err != nil {
		return nil, fmt.Errorf("clear features dir: %w", err)
	}
	if err := s.fs.MkdirAll(featuresDir, 0o755); err != nil {
		return nil, fmt.Errorf("create features dir: %w", err)
	}

	result := &driving.GherkinExportResult{FeaturesDir: featuresDir}
	for _, f := range files {
		abs := filepath.Join(featuresDir, f.rel)
		if err := s.fs.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			return nil, fmt.Errorf("create story dir for %s: %w", f.rel, err)
		}
		if err := s.fs.WriteFile(abs, []byte(f.content), 0o644); err != nil {
			return nil, fmt.Errorf("write %s: %w", f.rel, err)
		}
		result.Files = append(result.Files, f.rel)
		result.ScenarioCount += f.count
	}

	return result, nil
}

// lineageIndex loads all stories, features, and epics into ID-keyed maps
// so per-spec ancestor tags can be resolved without N round-trips.
func (s *ExportService) lineageIndex(ctx context.Context, dbDir string) (
	map[string]domain.Story, map[string]domain.Feature, map[string]domain.Epic, error,
) {
	stories, err := s.storyRepo.ListStories(ctx, dbDir, "")
	if err != nil {
		return nil, nil, nil, fmt.Errorf("list stories: %w", err)
	}
	features, err := s.featureRepo.ListFeatures(ctx, dbDir, "")
	if err != nil {
		return nil, nil, nil, fmt.Errorf("list features: %w", err)
	}
	epics, err := s.epicRepo.ListEpics(ctx, dbDir, "")
	if err != nil {
		return nil, nil, nil, fmt.Errorf("list epics: %w", err)
	}

	storyByID := make(map[string]domain.Story, len(stories))
	for _, st := range stories {
		storyByID[st.ID] = st
	}
	featureByID := make(map[string]domain.Feature, len(features))
	for _, ft := range features {
		featureByID[ft.ID] = ft
	}
	epicByID := make(map[string]domain.Epic, len(epics))
	for _, ep := range epics {
		epicByID[ep.ID] = ep
	}
	return storyByID, featureByID, epicByID, nil
}

// featureTags builds the inherited ancestor d7 tags for a Spec's feature
// file, broadest first: @d7:EPIC, @d7:FEAT, @d7:STORY, @d7:SPEC. The
// Story/Spec tags always resolve from the Spec itself; the Feature/Epic
// tags are added when the lineage resolves cleanly. Tags are returned
// without the leading @, matching the stored tag convention.
func featureTags(
	spec domain.Spec,
	storyByID map[string]domain.Story,
	featureByID map[string]domain.Feature,
	epicByID map[string]domain.Epic,
) []string {
	var tags []string
	if story, ok := storyByID[spec.StoryID]; ok {
		if feature, ok := featureByID[story.FeatureID]; ok {
			if epic, ok := epicByID[feature.EpicID]; ok {
				tags = append(tags, "d7:"+epic.ID)
			}
			tags = append(tags, "d7:"+feature.ID)
		}
	}
	tags = append(tags, "d7:"+spec.StoryID, "d7:"+spec.ID)
	return tags
}
