package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/c64-io/daedalus/internal/core/domain"
	"github.com/c64-io/daedalus/internal/core/port/driven"
)

// NavToolsHandler dispatches the four read-only navigation tools the
// LLM can call mid-dialog to explore the entity graph on demand.
// Each tool takes one required "id" argument and returns plain text.
// Errors are returned as text content (not Go errors) so the dialog
// stays alive and the model can self-correct.
type NavToolsHandler struct {
	ideaRepo  driven.IdeaRepository
	epicRepo  driven.EpicRepository
	featRepo  driven.FeatureRepository
	storyRepo driven.StoryRepository
	specRepo  driven.SpecRepository
	scenRepo  driven.ScenarioRepository
	linkRepo  driven.LinkRepository
	refRepo   driven.RefRepository
	resolver  driven.EntityResolver
	dbDir     string
}

func NewNavToolsHandler(
	ideaRepo driven.IdeaRepository,
	epicRepo driven.EpicRepository,
	featRepo driven.FeatureRepository,
	storyRepo driven.StoryRepository,
	specRepo driven.SpecRepository,
	scenRepo driven.ScenarioRepository,
	linkRepo driven.LinkRepository,
	refRepo driven.RefRepository,
	resolver driven.EntityResolver,
	dbDir string,
) *NavToolsHandler {
	return &NavToolsHandler{
		ideaRepo:  ideaRepo,
		epicRepo:  epicRepo,
		featRepo:  featRepo,
		storyRepo: storyRepo,
		specRepo:  specRepo,
		scenRepo:  scenRepo,
		linkRepo:  linkRepo,
		refRepo:   refRepo,
		resolver:  resolver,
		dbDir:     dbDir,
	}
}

func (h *NavToolsHandler) Handle(ctx context.Context, tu *driven.ToolUseContent) string {
	switch tu.Name {
	case driven.ToolGetLineage:
		return h.getLineage(ctx, tu)
	case driven.ToolGetItemDetail:
		return h.getItemDetail(ctx, tu)
	case driven.ToolGetSiblings:
		return h.getSiblings(ctx, tu)
	case driven.ToolTraceLinks:
		return h.traceLinks(ctx, tu)
	default:
		return fmt.Sprintf("error: unknown navigation tool %q", tu.Name)
	}
}

func (h *NavToolsHandler) extractID(tu *driven.ToolUseContent) (string, string) {
	id, _ := tu.Input["id"].(string)
	id = strings.TrimSpace(id)
	if id == "" {
		return "", "error: id argument is required"
	}
	return id, ""
}

// getLineage returns the ancestor chain from root down to the given entity,
// formatted as "IDEA-001 > EPIC-003 > FEAT-007".
func (h *NavToolsHandler) getLineage(ctx context.Context, tu *driven.ToolUseContent) string {
	id, errMsg := h.extractID(tu)
	if errMsg != "" {
		return errMsg
	}

	prefix, _, err := domain.ParseEntityPrefix(id)
	if err != nil {
		return fmt.Sprintf("error: %v", err)
	}

	var chain []string
	switch prefix {
	case domain.IdeaIDPrefix:
		chain = []string{id}

	case domain.EpicIDPrefix:
		epic, err := h.epicRepo.GetEpic(ctx, h.dbDir, id)
		if err != nil {
			return h.notFoundMsg(err, id)
		}
		chain = []string{epic.IdeaID, id}

	case domain.FeatureIDPrefix:
		feat, err := h.featRepo.GetFeature(ctx, h.dbDir, id)
		if err != nil {
			return h.notFoundMsg(err, id)
		}
		epic, err := h.epicRepo.GetEpic(ctx, h.dbDir, feat.EpicID)
		if err != nil {
			return h.notFoundMsg(err, feat.EpicID)
		}
		chain = []string{epic.IdeaID, feat.EpicID, id}

	case domain.StoryIDPrefix:
		story, err := h.storyRepo.GetStory(ctx, h.dbDir, id)
		if err != nil {
			return h.notFoundMsg(err, id)
		}
		feat, err := h.featRepo.GetFeature(ctx, h.dbDir, story.FeatureID)
		if err != nil {
			return h.notFoundMsg(err, story.FeatureID)
		}
		epic, err := h.epicRepo.GetEpic(ctx, h.dbDir, feat.EpicID)
		if err != nil {
			return h.notFoundMsg(err, feat.EpicID)
		}
		chain = []string{epic.IdeaID, feat.EpicID, story.FeatureID, id}

	case domain.SpecIDPrefix:
		spec, err := h.specRepo.GetSpec(ctx, h.dbDir, id)
		if err != nil {
			return h.notFoundMsg(err, id)
		}
		story, err := h.storyRepo.GetStory(ctx, h.dbDir, spec.StoryID)
		if err != nil {
			return h.notFoundMsg(err, spec.StoryID)
		}
		feat, err := h.featRepo.GetFeature(ctx, h.dbDir, story.FeatureID)
		if err != nil {
			return h.notFoundMsg(err, story.FeatureID)
		}
		epic, err := h.epicRepo.GetEpic(ctx, h.dbDir, feat.EpicID)
		if err != nil {
			return h.notFoundMsg(err, feat.EpicID)
		}
		chain = []string{epic.IdeaID, feat.EpicID, story.FeatureID, spec.StoryID, id}

	case domain.ScenarioIDPrefix:
		scen, err := h.scenRepo.GetScenario(ctx, h.dbDir, id)
		if err != nil {
			return h.notFoundMsg(err, id)
		}
		spec, err := h.specRepo.GetSpec(ctx, h.dbDir, scen.SpecID)
		if err != nil {
			return h.notFoundMsg(err, scen.SpecID)
		}
		story, err := h.storyRepo.GetStory(ctx, h.dbDir, spec.StoryID)
		if err != nil {
			return h.notFoundMsg(err, spec.StoryID)
		}
		feat, err := h.featRepo.GetFeature(ctx, h.dbDir, story.FeatureID)
		if err != nil {
			return h.notFoundMsg(err, story.FeatureID)
		}
		epic, err := h.epicRepo.GetEpic(ctx, h.dbDir, feat.EpicID)
		if err != nil {
			return h.notFoundMsg(err, feat.EpicID)
		}
		chain = []string{epic.IdeaID, feat.EpicID, story.FeatureID, spec.StoryID, scen.SpecID, id}

	default:
		return fmt.Sprintf("error: unsupported entity prefix %q", prefix)
	}

	return strings.Join(chain, " > ")
}

// getItemDetail returns a rendered entity block for the given ID.
func (h *NavToolsHandler) getItemDetail(ctx context.Context, tu *driven.ToolUseContent) string {
	id, errMsg := h.extractID(tu)
	if errMsg != "" {
		return errMsg
	}

	prefix, _, err := domain.ParseEntityPrefix(id)
	if err != nil {
		return fmt.Sprintf("error: %v", err)
	}

	var entity domain.DialogContextEntity
	switch prefix {
	case domain.IdeaIDPrefix:
		idea, err := h.ideaRepo.GetIdea(ctx, h.dbDir, id)
		if err != nil {
			return h.notFoundMsg(err, id)
		}
		entity = ideaToContextEntity(*idea)

	case domain.EpicIDPrefix:
		epic, err := h.epicRepo.GetEpic(ctx, h.dbDir, id)
		if err != nil {
			return h.notFoundMsg(err, id)
		}
		entity = epicToContextEntity(*epic)

	case domain.FeatureIDPrefix:
		feat, err := h.featRepo.GetFeature(ctx, h.dbDir, id)
		if err != nil {
			return h.notFoundMsg(err, id)
		}
		entity = featureToContextEntity(*feat)

	case domain.StoryIDPrefix:
		story, err := h.storyRepo.GetStory(ctx, h.dbDir, id)
		if err != nil {
			return h.notFoundMsg(err, id)
		}
		entity = storyToContextEntity(*story)

	case domain.SpecIDPrefix:
		spec, err := h.specRepo.GetSpec(ctx, h.dbDir, id)
		if err != nil {
			return h.notFoundMsg(err, id)
		}
		entity = specToContextEntity(*spec)

	case domain.ScenarioIDPrefix:
		scen, err := h.scenRepo.GetScenario(ctx, h.dbDir, id)
		if err != nil {
			return h.notFoundMsg(err, id)
		}
		entity = scenarioToContextEntity(*scen)

	default:
		return fmt.Sprintf("error: unsupported entity prefix %q", prefix)
	}

	refs, _ := h.refRepo.ListRefsByEntity(ctx, h.dbDir, id)
	var dcRefs []domain.DialogContextRef
	for _, r := range refs {
		dcRefs = append(dcRefs, domain.DialogContextRef{URL: r.URL, Label: r.Label})
	}

	return domain.WriteEntityDetail(entity, dcRefs)
}

// getSiblings returns one-line summaries for each sibling of the given entity.
func (h *NavToolsHandler) getSiblings(ctx context.Context, tu *driven.ToolUseContent) string {
	id, errMsg := h.extractID(tu)
	if errMsg != "" {
		return errMsg
	}

	prefix, _, err := domain.ParseEntityPrefix(id)
	if err != nil {
		return fmt.Sprintf("error: %v", err)
	}

	switch prefix {
	case domain.IdeaIDPrefix:
		return "(ideas have no parent; no siblings)"

	case domain.EpicIDPrefix:
		epic, err := h.epicRepo.GetEpic(ctx, h.dbDir, id)
		if err != nil {
			return h.notFoundMsg(err, id)
		}
		epics, err := h.epicRepo.ListEpics(ctx, h.dbDir, epic.IdeaID)
		if err != nil {
			return fmt.Sprintf("error: %v", err)
		}
		return h.formatSiblingLines(id, epicsToSiblings(epics))

	case domain.FeatureIDPrefix:
		feat, err := h.featRepo.GetFeature(ctx, h.dbDir, id)
		if err != nil {
			return h.notFoundMsg(err, id)
		}
		features, err := h.featRepo.ListFeatures(ctx, h.dbDir, feat.EpicID)
		if err != nil {
			return fmt.Sprintf("error: %v", err)
		}
		return h.formatSiblingLines(id, featuresToSiblings(features))

	case domain.StoryIDPrefix:
		story, err := h.storyRepo.GetStory(ctx, h.dbDir, id)
		if err != nil {
			return h.notFoundMsg(err, id)
		}
		stories, err := h.storyRepo.ListStories(ctx, h.dbDir, story.FeatureID)
		if err != nil {
			return fmt.Sprintf("error: %v", err)
		}
		return h.formatSiblingLines(id, storiesToSiblings(stories))

	case domain.SpecIDPrefix:
		spec, err := h.specRepo.GetSpec(ctx, h.dbDir, id)
		if err != nil {
			return h.notFoundMsg(err, id)
		}
		specs, err := h.specRepo.ListSpecs(ctx, h.dbDir, spec.StoryID)
		if err != nil {
			return fmt.Sprintf("error: %v", err)
		}
		return h.formatSiblingLines(id, specsToSiblings(specs))

	case domain.ScenarioIDPrefix:
		scen, err := h.scenRepo.GetScenario(ctx, h.dbDir, id)
		if err != nil {
			return h.notFoundMsg(err, id)
		}
		scens, err := h.scenRepo.ListScenarios(ctx, h.dbDir, scen.SpecID)
		if err != nil {
			return fmt.Sprintf("error: %v", err)
		}
		return h.formatSiblingLines(id, scenariosToSiblings(scens))

	default:
		return fmt.Sprintf("error: unsupported entity prefix %q", prefix)
	}
}

// traceLinks returns direction-normalized links touching the given entity.
func (h *NavToolsHandler) traceLinks(ctx context.Context, tu *driven.ToolUseContent) string {
	id, errMsg := h.extractID(tu)
	if errMsg != "" {
		return errMsg
	}

	links, err := h.linkRepo.ListLinksByEntity(ctx, h.dbDir, id)
	if err != nil {
		return fmt.Sprintf("error: %v", err)
	}
	if len(links) == 0 {
		return "(no links)"
	}

	var lines []string
	for _, l := range links {
		var other, relation string
		switch {
		case l.FromID == id:
			other = l.ToID
			relation = string(l.Kind)
		case l.ToID == id:
			other = l.FromID
			relation = l.Kind.InverseLabel()
		default:
			other = l.ToID
			relation = string(l.Kind)
		}
		title, err := h.resolver.ResolveEntity(ctx, h.dbDir, other)
		if err != nil && !errors.Is(err, driven.ErrEntityNotFound) {
			return fmt.Sprintf("error: resolve %s: %v", other, err)
		}
		if title != "" {
			lines = append(lines, fmt.Sprintf("%s  %s  %q", relation, other, title))
		} else {
			lines = append(lines, fmt.Sprintf("%s  %s", relation, other))
		}
	}
	return strings.Join(lines, "\n")
}

func (h *NavToolsHandler) notFoundMsg(err error, id string) string {
	return fmt.Sprintf("error: %s not found (%v)", id, err)
}

type siblingEntry struct {
	id     string
	title  string
	status string
}

func (h *NavToolsHandler) formatSiblingLines(selfID string, siblings []siblingEntry) string {
	var lines []string
	for _, s := range siblings {
		if s.id == selfID {
			continue
		}
		lines = append(lines, fmt.Sprintf("%s  %q  [%s]", s.id, s.title, s.status))
	}
	if len(lines) == 0 {
		return "(no siblings)"
	}
	return strings.Join(lines, "\n")
}

func epicsToSiblings(epics []domain.Epic) []siblingEntry {
	out := make([]siblingEntry, len(epics))
	for i, e := range epics {
		out[i] = siblingEntry{id: e.ID, title: e.Title, status: string(e.Status)}
	}
	return out
}

func featuresToSiblings(features []domain.Feature) []siblingEntry {
	out := make([]siblingEntry, len(features))
	for i, f := range features {
		out[i] = siblingEntry{id: f.ID, title: f.Title, status: string(f.Status)}
	}
	return out
}

func storiesToSiblings(stories []domain.Story) []siblingEntry {
	out := make([]siblingEntry, len(stories))
	for i, s := range stories {
		out[i] = siblingEntry{id: s.ID, title: s.Title, status: string(s.Status)}
	}
	return out
}

func specsToSiblings(specs []domain.Spec) []siblingEntry {
	out := make([]siblingEntry, len(specs))
	for i, s := range specs {
		out[i] = siblingEntry{id: s.ID, title: s.Title, status: string(s.Status)}
	}
	return out
}

func scenariosToSiblings(scens []domain.Scenario) []siblingEntry {
	out := make([]siblingEntry, len(scens))
	for i, s := range scens {
		out[i] = siblingEntry{id: s.ID, title: s.Title, status: string(s.Status)}
	}
	return out
}
