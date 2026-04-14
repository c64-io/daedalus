package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/c64-io/daedalus/internal/core/domain"
	"github.com/c64-io/daedalus/internal/core/port/driven"
	"github.com/c64-io/daedalus/internal/core/port/driving"
)

// Sentinel errors shared by every suggest-* command.
var (
	ErrSuggestIDRequired      = errors.New("parent ID is required")
	ErrSuggestProposalItems   = errors.New("proposal is missing a non-empty items array")
	ErrSuggestNoItemsAccepted = errors.New("no items were accepted; nothing was saved")
)

// Defaults shared by every suggest-* command. Same shape as expand,
// tuned identically: the interview budget and backoff policy are a
// property of the dialog loop, not the command.
const (
	defaultSuggestModel           = "claude-haiku-4-5-20251001"
	defaultSuggestMaxInterviews   = 10
	defaultSuggestMalformedBudget = 2
	defaultSuggestBackoffBudget   = 3
	defaultSuggestBackoffBase     = 2 * time.Second
	defaultSuggestMaxTokens       = 8192 // larger than expand — the payload is an array
)

// Compile-time assertions — one per driving port SuggestService satisfies.
var (
	_ driving.EpicsSuggester     = (*SuggestService)(nil)
	_ driving.FeaturesSuggester  = (*SuggestService)(nil)
	_ driving.StoriesSuggester   = (*SuggestService)(nil)
	_ driving.SpecsSuggester     = (*SuggestService)(nil)
	_ driving.ScenariosSuggester = (*SuggestService)(nil)
)

// SuggestService wires the AI-assisted bulk-child-drafting use cases
// for every interior layer of the hierarchy: Epics under an Idea,
// Features under an Epic, Stories under a Feature, Specs under a
// Story, Scenarios under a Spec. Each SuggestX method walks the
// parent chain into a DialogContext, runs the interview-first AI
// dialog, and on acceptance creates one child per accepted item.
//
// Unlike ExpandService, SuggestService depends on the driving Creator
// ports (EpicCreator, FeatureCreator, …) so that every created item
// flows through the same state-machine validation, history tracking,
// and business rules the CLI uses. This is a horizontal core-core
// dependency, not a port inversion — the concrete services satisfy
// the interfaces already.
type SuggestService struct {
	contextSource // embedded: attachCommonContext, resolveDialogLink

	fs        driven.FileSystem
	ideaRepo  driven.IdeaRepository
	epicRepo  driven.EpicRepository
	featRepo  driven.FeatureRepository
	storyRepo driven.StoryRepository
	specRepo  driven.SpecRepository

	// Driving creators — see the struct comment for why.
	epicCreator     driving.EpicCreator
	featureCreator  driving.FeatureCreator
	storyCreator    driving.StoryCreator
	specCreator     driving.SpecCreator
	scenarioCreator driving.ScenarioCreator

	llm    driven.AIAssistant
	inter  driven.Interaction
	status driven.StatusRenderer
	clock  driven.Clock
}

// NewSuggestService wires the suggest service with all of its driven
// dependencies and its driving Creator dependencies.
func NewSuggestService(
	fs driven.FileSystem,
	wsRepo driven.WorkspaceRepository,
	ideaRepo driven.IdeaRepository,
	epicRepo driven.EpicRepository,
	featRepo driven.FeatureRepository,
	storyRepo driven.StoryRepository,
	specRepo driven.SpecRepository,
	linkRepo driven.LinkRepository,
	refRepo driven.RefRepository,
	resolver driven.EntityResolver,
	epicCreator driving.EpicCreator,
	featureCreator driving.FeatureCreator,
	storyCreator driving.StoryCreator,
	specCreator driving.SpecCreator,
	scenarioCreator driving.ScenarioCreator,
	llm driven.AIAssistant,
	inter driven.Interaction,
	status driven.StatusRenderer,
	clock driven.Clock,
) *SuggestService {
	return &SuggestService{
		contextSource: contextSource{
			wsRepo:   wsRepo,
			linkRepo: linkRepo,
			refRepo:  refRepo,
			resolver: resolver,
		},
		fs:              fs,
		ideaRepo:        ideaRepo,
		epicRepo:        epicRepo,
		featRepo:        featRepo,
		storyRepo:       storyRepo,
		specRepo:        specRepo,
		epicCreator:     epicCreator,
		featureCreator:  featureCreator,
		storyCreator:    storyCreator,
		specCreator:     specCreator,
		scenarioCreator: scenarioCreator,
		llm:             llm,
		inter:           inter,
		status:          status,
		clock:           clock,
	}
}

// ---------------------------------------------------------------------
// Public use-case methods — one per child entity type.
// ---------------------------------------------------------------------

// SuggestEpics runs the interactive dialog to propose a batch of Epics
// under a parent Idea. Returns the slice of created Epics.
func (s *SuggestService) SuggestEpics(ctx context.Context, req driving.SuggestEpicsRequest) ([]domain.Epic, error) {
	if req.IdeaID == "" {
		return nil, ErrSuggestIDRequired
	}
	ws, err := requireWorkspace(s.fs, req.RootDir)
	if err != nil {
		return nil, err
	}
	idea, err := s.ideaRepo.GetIdea(ctx, ws.DBDir, req.IdeaID)
	if err != nil {
		return nil, fmt.Errorf("get idea: %w", err)
	}
	var dc domain.DialogContext
	if err := s.attachCommonContext(ctx, ws.DBDir, idea.ID, &dc); err != nil {
		return nil, fmt.Errorf("build dialog context: %w", err)
	}
	dc.Target = ideaToContextEntity(*idea)

	result, err := s.runSuggest(ctx, runSuggestParams{
		command:           "suggest epics",
		target:            idea.ID,
		model:             req.Model,
		systemPrompt:      suggestEpicsSystemPrompt,
		submitDescription: suggestEpicsSubmitDescription,
		schema:            suggestItemsSchema,
		validate:          validateSuggestItemsPayload,
		dc:                dc,
		apply: func(ctx context.Context, payload map[string]any) (any, error) {
			return s.applySuggestEpics(ctx, req.RootDir, idea.ID, payload)
		},
	})
	if err != nil {
		return nil, err
	}
	epics, ok := result.([]domain.Epic)
	if !ok {
		return nil, fmt.Errorf("suggest epics: unexpected apply result %T", result)
	}
	return epics, nil
}

// SuggestFeatures runs the interactive dialog to propose a batch of
// Features under a parent Epic.
func (s *SuggestService) SuggestFeatures(ctx context.Context, req driving.SuggestFeaturesRequest) ([]domain.Feature, error) {
	if req.EpicID == "" {
		return nil, ErrSuggestIDRequired
	}
	ws, err := requireWorkspace(s.fs, req.RootDir)
	if err != nil {
		return nil, err
	}
	epic, err := s.epicRepo.GetEpic(ctx, ws.DBDir, req.EpicID)
	if err != nil {
		return nil, fmt.Errorf("get epic: %w", err)
	}
	idea, err := s.ideaRepo.GetIdea(ctx, ws.DBDir, epic.IdeaID)
	if err != nil {
		return nil, fmt.Errorf("get parent idea %s: %w", epic.IdeaID, err)
	}
	var dc domain.DialogContext
	if err := s.attachCommonContext(ctx, ws.DBDir, epic.ID, &dc); err != nil {
		return nil, fmt.Errorf("build dialog context: %w", err)
	}
	dc.Ancestors = []domain.DialogContextEntity{ideaToContextEntity(*idea)}
	dc.Target = epicToContextEntity(*epic)

	result, err := s.runSuggest(ctx, runSuggestParams{
		command:           "suggest features",
		target:            epic.ID,
		model:             req.Model,
		systemPrompt:      suggestFeaturesSystemPrompt,
		submitDescription: suggestFeaturesSubmitDescription,
		schema:            suggestItemsSchema,
		validate:          validateSuggestItemsPayload,
		dc:                dc,
		apply: func(ctx context.Context, payload map[string]any) (any, error) {
			return s.applySuggestFeatures(ctx, req.RootDir, epic.ID, payload)
		},
	})
	if err != nil {
		return nil, err
	}
	feats, ok := result.([]domain.Feature)
	if !ok {
		return nil, fmt.Errorf("suggest features: unexpected apply result %T", result)
	}
	return feats, nil
}

// SuggestStories runs the interactive dialog to propose a batch of
// Stories under a parent Feature.
func (s *SuggestService) SuggestStories(ctx context.Context, req driving.SuggestStoriesRequest) ([]domain.Story, error) {
	if req.FeatureID == "" {
		return nil, ErrSuggestIDRequired
	}
	ws, err := requireWorkspace(s.fs, req.RootDir)
	if err != nil {
		return nil, err
	}
	feat, err := s.featRepo.GetFeature(ctx, ws.DBDir, req.FeatureID)
	if err != nil {
		return nil, fmt.Errorf("get feature: %w", err)
	}
	epic, err := s.epicRepo.GetEpic(ctx, ws.DBDir, feat.EpicID)
	if err != nil {
		return nil, fmt.Errorf("get parent epic %s: %w", feat.EpicID, err)
	}
	idea, err := s.ideaRepo.GetIdea(ctx, ws.DBDir, epic.IdeaID)
	if err != nil {
		return nil, fmt.Errorf("get parent idea %s: %w", epic.IdeaID, err)
	}
	var dc domain.DialogContext
	if err := s.attachCommonContext(ctx, ws.DBDir, feat.ID, &dc); err != nil {
		return nil, fmt.Errorf("build dialog context: %w", err)
	}
	dc.Ancestors = []domain.DialogContextEntity{
		ideaToContextEntity(*idea),
		epicToContextEntity(*epic),
	}
	dc.Target = featureToContextEntity(*feat)

	result, err := s.runSuggest(ctx, runSuggestParams{
		command:           "suggest stories",
		target:            feat.ID,
		model:             req.Model,
		systemPrompt:      suggestStoriesSystemPrompt,
		submitDescription: suggestStoriesSubmitDescription,
		schema:            suggestItemsSchema,
		validate:          validateSuggestItemsPayload,
		dc:                dc,
		apply: func(ctx context.Context, payload map[string]any) (any, error) {
			return s.applySuggestStories(ctx, req.RootDir, feat.ID, payload)
		},
	})
	if err != nil {
		return nil, err
	}
	stories, ok := result.([]domain.Story)
	if !ok {
		return nil, fmt.Errorf("suggest stories: unexpected apply result %T", result)
	}
	return stories, nil
}

// SuggestSpecs runs the interactive dialog to propose a batch of Specs
// under a parent Story.
func (s *SuggestService) SuggestSpecs(ctx context.Context, req driving.SuggestSpecsRequest) ([]domain.Spec, error) {
	if req.StoryID == "" {
		return nil, ErrSuggestIDRequired
	}
	ws, err := requireWorkspace(s.fs, req.RootDir)
	if err != nil {
		return nil, err
	}
	story, err := s.storyRepo.GetStory(ctx, ws.DBDir, req.StoryID)
	if err != nil {
		return nil, fmt.Errorf("get story: %w", err)
	}
	feat, err := s.featRepo.GetFeature(ctx, ws.DBDir, story.FeatureID)
	if err != nil {
		return nil, fmt.Errorf("get parent feature %s: %w", story.FeatureID, err)
	}
	epic, err := s.epicRepo.GetEpic(ctx, ws.DBDir, feat.EpicID)
	if err != nil {
		return nil, fmt.Errorf("get parent epic %s: %w", feat.EpicID, err)
	}
	idea, err := s.ideaRepo.GetIdea(ctx, ws.DBDir, epic.IdeaID)
	if err != nil {
		return nil, fmt.Errorf("get parent idea %s: %w", epic.IdeaID, err)
	}
	var dc domain.DialogContext
	if err := s.attachCommonContext(ctx, ws.DBDir, story.ID, &dc); err != nil {
		return nil, fmt.Errorf("build dialog context: %w", err)
	}
	dc.Ancestors = []domain.DialogContextEntity{
		ideaToContextEntity(*idea),
		epicToContextEntity(*epic),
		featureToContextEntity(*feat),
	}
	dc.Target = storyToContextEntity(*story)

	result, err := s.runSuggest(ctx, runSuggestParams{
		command:           "suggest specs",
		target:            story.ID,
		model:             req.Model,
		systemPrompt:      suggestSpecsSystemPrompt,
		submitDescription: suggestSpecsSubmitDescription,
		schema:            suggestItemsSchema,
		validate:          validateSuggestItemsPayload,
		dc:                dc,
		apply: func(ctx context.Context, payload map[string]any) (any, error) {
			return s.applySuggestSpecs(ctx, req.RootDir, story.ID, payload)
		},
	})
	if err != nil {
		return nil, err
	}
	specs, ok := result.([]domain.Spec)
	if !ok {
		return nil, fmt.Errorf("suggest specs: unexpected apply result %T", result)
	}
	return specs, nil
}

// SuggestScenarios runs the interactive dialog to propose a batch of
// Scenarios under a parent Spec. Scenarios are the one structured
// child — their items carry Given/When/Then, not a flat description.
func (s *SuggestService) SuggestScenarios(ctx context.Context, req driving.SuggestScenariosRequest) ([]domain.Scenario, error) {
	if req.SpecID == "" {
		return nil, ErrSuggestIDRequired
	}
	ws, err := requireWorkspace(s.fs, req.RootDir)
	if err != nil {
		return nil, err
	}
	spec, err := s.specRepo.GetSpec(ctx, ws.DBDir, req.SpecID)
	if err != nil {
		return nil, fmt.Errorf("get spec: %w", err)
	}
	story, err := s.storyRepo.GetStory(ctx, ws.DBDir, spec.StoryID)
	if err != nil {
		return nil, fmt.Errorf("get parent story %s: %w", spec.StoryID, err)
	}
	feat, err := s.featRepo.GetFeature(ctx, ws.DBDir, story.FeatureID)
	if err != nil {
		return nil, fmt.Errorf("get parent feature %s: %w", story.FeatureID, err)
	}
	epic, err := s.epicRepo.GetEpic(ctx, ws.DBDir, feat.EpicID)
	if err != nil {
		return nil, fmt.Errorf("get parent epic %s: %w", feat.EpicID, err)
	}
	idea, err := s.ideaRepo.GetIdea(ctx, ws.DBDir, epic.IdeaID)
	if err != nil {
		return nil, fmt.Errorf("get parent idea %s: %w", epic.IdeaID, err)
	}
	var dc domain.DialogContext
	if err := s.attachCommonContext(ctx, ws.DBDir, spec.ID, &dc); err != nil {
		return nil, fmt.Errorf("build dialog context: %w", err)
	}
	dc.Ancestors = []domain.DialogContextEntity{
		ideaToContextEntity(*idea),
		epicToContextEntity(*epic),
		featureToContextEntity(*feat),
		storyToContextEntity(*story),
	}
	dc.Target = specToContextEntity(*spec)

	result, err := s.runSuggest(ctx, runSuggestParams{
		command:           "suggest scenarios",
		target:            spec.ID,
		model:             req.Model,
		systemPrompt:      suggestScenariosSystemPrompt,
		submitDescription: suggestScenariosSubmitDescription,
		schema:            suggestScenariosSchema,
		validate:          validateSuggestScenariosPayload,
		dc:                dc,
		apply: func(ctx context.Context, payload map[string]any) (any, error) {
			return s.applySuggestScenarios(ctx, req.RootDir, spec.ID, payload)
		},
	})
	if err != nil {
		return nil, err
	}
	scens, ok := result.([]domain.Scenario)
	if !ok {
		return nil, fmt.Errorf("suggest scenarios: unexpected apply result %T", result)
	}
	return scens, nil
}

// ---------------------------------------------------------------------
// Shared dialog runner.
// ---------------------------------------------------------------------

type runSuggestParams struct {
	command           string
	target            string
	model             string
	systemPrompt      string
	submitDescription string
	schema            map[string]any
	validate          func(map[string]any) error
	dc                domain.DialogContext
	apply             func(ctx context.Context, payload map[string]any) (any, error)
}

// runSuggest builds and runs the shared dialog loop for every suggest
// command. The per-entity variation is the system prompt, the
// submit_proposal schema, the validator, and the apply callback.
func (s *SuggestService) runSuggest(ctx context.Context, p runSuggestParams) (any, error) {
	model := p.model
	if model == "" {
		model = defaultSuggestModel
	}
	dlg := &aiDialog{
		command:          p.command,
		target:           p.target,
		model:            model,
		system:           p.systemPrompt,
		contextBlock:     domain.FormatDialogContext(p.dc),
		tools:            suggestTools(p.submitDescription, p.schema),
		maxTokensPerTurn: defaultSuggestMaxTokens,
		maxInterviews:    defaultSuggestMaxInterviews,
		malformedBudget:  defaultSuggestMalformedBudget,
		backoffBudget:    defaultSuggestBackoffBudget,
		backoffBase:      defaultSuggestBackoffBase,

		validate:       p.validate,
		renderProposal: renderSuggestProposal,
		apply:          p.apply,
	}
	return runDialog(ctx, dlg, s.llm, s.inter, s.status, s.clock)
}

// ---------------------------------------------------------------------
// Per-entity apply — each creates one child per accepted item via
// the appropriate driving Creator port, so state-machine validation
// and history are honored exactly as they are in the CLI flow.
// ---------------------------------------------------------------------

func (s *SuggestService) applySuggestEpics(ctx context.Context, rootDir, ideaID string, payload map[string]any) ([]domain.Epic, error) {
	items, accepted, err := extractSuggestItems(payload)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Epic, 0, len(accepted))
	for _, idx := range accepted {
		it := items[idx]
		title, _ := it["title"].(string)
		desc, _ := it["description"].(string)
		epic, err := s.epicCreator.CreateEpic(ctx, driving.CreateEpicRequest{
			RootDir:     rootDir,
			IdeaID:      ideaID,
			Title:       strings.TrimSpace(title),
			Description: strings.TrimSpace(desc),
		})
		if err != nil {
			return out, fmt.Errorf("create epic %d (%q): %w", idx+1, title, err)
		}
		out = append(out, *epic)
	}
	if len(out) == 0 {
		return nil, ErrSuggestNoItemsAccepted
	}
	return out, nil
}

func (s *SuggestService) applySuggestFeatures(ctx context.Context, rootDir, epicID string, payload map[string]any) ([]domain.Feature, error) {
	items, accepted, err := extractSuggestItems(payload)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Feature, 0, len(accepted))
	for _, idx := range accepted {
		it := items[idx]
		title, _ := it["title"].(string)
		desc, _ := it["description"].(string)
		feat, err := s.featureCreator.CreateFeature(ctx, driving.CreateFeatureRequest{
			RootDir:     rootDir,
			EpicID:      epicID,
			Title:       strings.TrimSpace(title),
			Description: strings.TrimSpace(desc),
		})
		if err != nil {
			return out, fmt.Errorf("create feature %d (%q): %w", idx+1, title, err)
		}
		out = append(out, *feat)
	}
	if len(out) == 0 {
		return nil, ErrSuggestNoItemsAccepted
	}
	return out, nil
}

func (s *SuggestService) applySuggestStories(ctx context.Context, rootDir, featureID string, payload map[string]any) ([]domain.Story, error) {
	items, accepted, err := extractSuggestItems(payload)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Story, 0, len(accepted))
	for _, idx := range accepted {
		it := items[idx]
		title, _ := it["title"].(string)
		desc, _ := it["description"].(string)
		story, err := s.storyCreator.CreateStory(ctx, driving.CreateStoryRequest{
			RootDir:     rootDir,
			FeatureID:   featureID,
			Title:       strings.TrimSpace(title),
			Description: strings.TrimSpace(desc),
		})
		if err != nil {
			return out, fmt.Errorf("create story %d (%q): %w", idx+1, title, err)
		}
		out = append(out, *story)
	}
	if len(out) == 0 {
		return nil, ErrSuggestNoItemsAccepted
	}
	return out, nil
}

func (s *SuggestService) applySuggestSpecs(ctx context.Context, rootDir, storyID string, payload map[string]any) ([]domain.Spec, error) {
	items, accepted, err := extractSuggestItems(payload)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Spec, 0, len(accepted))
	for _, idx := range accepted {
		it := items[idx]
		title, _ := it["title"].(string)
		desc, _ := it["description"].(string)
		spec, err := s.specCreator.CreateSpec(ctx, driving.CreateSpecRequest{
			RootDir:     rootDir,
			StoryID:     storyID,
			Title:       strings.TrimSpace(title),
			Description: strings.TrimSpace(desc),
		})
		if err != nil {
			return out, fmt.Errorf("create spec %d (%q): %w", idx+1, title, err)
		}
		out = append(out, *spec)
	}
	if len(out) == 0 {
		return nil, ErrSuggestNoItemsAccepted
	}
	return out, nil
}

func (s *SuggestService) applySuggestScenarios(ctx context.Context, rootDir, specID string, payload map[string]any) ([]domain.Scenario, error) {
	items, accepted, err := extractSuggestItems(payload)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Scenario, 0, len(accepted))
	for _, idx := range accepted {
		it := items[idx]
		title, _ := it["title"].(string)
		given := plainSteps(it["given"])
		when := plainSteps(it["when"])
		then := plainSteps(it["then"])
		scen, err := s.scenarioCreator.CreateScenario(ctx, driving.CreateScenarioRequest{
			RootDir: rootDir,
			SpecID:  specID,
			Title:   strings.TrimSpace(title),
			Given:   given,
			When:    when,
			Then:    then,
		})
		if err != nil {
			return out, fmt.Errorf("create scenario %d (%q): %w", idx+1, title, err)
		}
		out = append(out, *scen)
	}
	if len(out) == 0 {
		return nil, ErrSuggestNoItemsAccepted
	}
	return out, nil
}

// plainSteps coerces a JSON-parsed []any of step strings into the
// []domain.Step shape the CreateScenario request expects. Any non-
// string entries are skipped (the schema's minLength=1 gate means the
// model shouldn't produce them).
func plainSteps(raw any) []domain.Step {
	arr, ok := raw.([]any)
	if !ok {
		return nil
	}
	out := make([]domain.Step, 0, len(arr))
	for _, v := range arr {
		if s, ok := v.(string); ok {
			s = strings.TrimSpace(s)
			if s == "" {
				continue
			}
			out = append(out, domain.Step{Text: s})
		}
	}
	return out
}

// ---------------------------------------------------------------------
// Shared proposal helpers.
// ---------------------------------------------------------------------

// suggestTools returns the two tool definitions every suggest-* command
// uses. The submit_proposal description and schema are entity-specific.
func suggestTools(submitDescription string, schema map[string]any) []driven.Tool {
	return []driven.Tool{
		{
			Name:        driven.ToolAskQuestion,
			Description: askQuestionDescription,
			InputSchema: askQuestionSchema,
		},
		{
			Name:        driven.ToolSubmitProposal,
			Description: submitDescription,
			InputSchema: schema,
		},
	}
}

// validateSuggestItemsPayload enforces the shared {items:[{title,
// description}]} schema. Used for epics, features, stories, and specs.
func validateSuggestItemsPayload(payload map[string]any) error {
	items, err := requireItemsArray(payload)
	if err != nil {
		return err
	}
	for i, raw := range items {
		it, ok := raw.(map[string]any)
		if !ok {
			return fmt.Errorf("%w: item %d is not an object", ErrSuggestProposalItems, i+1)
		}
		title, _ := it["title"].(string)
		if strings.TrimSpace(title) == "" {
			return fmt.Errorf("%w: item %d has no title", ErrSuggestProposalItems, i+1)
		}
		desc, _ := it["description"].(string)
		if strings.TrimSpace(desc) == "" {
			return fmt.Errorf("%w: item %d has no description", ErrSuggestProposalItems, i+1)
		}
	}
	return nil
}

// validateSuggestScenariosPayload enforces the scenario schema: each
// item has a title, optional given[], non-empty when[], non-empty
// then[] — all strings.
func validateSuggestScenariosPayload(payload map[string]any) error {
	items, err := requireItemsArray(payload)
	if err != nil {
		return err
	}
	for i, raw := range items {
		it, ok := raw.(map[string]any)
		if !ok {
			return fmt.Errorf("%w: item %d is not an object", ErrSuggestProposalItems, i+1)
		}
		title, _ := it["title"].(string)
		if strings.TrimSpace(title) == "" {
			return fmt.Errorf("%w: item %d has no title", ErrSuggestProposalItems, i+1)
		}
		if err := validateSteps(it["given"], i+1, "given", false); err != nil {
			return err
		}
		if err := validateSteps(it["when"], i+1, "when", true); err != nil {
			return err
		}
		if err := validateSteps(it["then"], i+1, "then", true); err != nil {
			return err
		}
	}
	return nil
}

// requireItemsArray extracts and sanity-checks the "items" array,
// tolerating the _accepted annotation we add post-review.
func requireItemsArray(payload map[string]any) ([]any, error) {
	if payload == nil {
		return nil, ErrSuggestProposalItems
	}
	raw, ok := payload["items"]
	if !ok {
		return nil, ErrSuggestProposalItems
	}
	arr, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("%w: items must be an array", ErrSuggestProposalItems)
	}
	if len(arr) == 0 {
		return nil, fmt.Errorf("%w: items is empty", ErrSuggestProposalItems)
	}
	for k := range payload {
		switch k {
		case "items", "_accepted":
			continue
		default:
			return nil, fmt.Errorf("proposal has unexpected key %q", k)
		}
	}
	return arr, nil
}

// validateSteps checks a []any of step strings. required=true rejects
// empty arrays.
func validateSteps(raw any, item int, field string, required bool) error {
	if raw == nil {
		if required {
			return fmt.Errorf("%w: item %d has no %s steps", ErrSuggestProposalItems, item, field)
		}
		return nil
	}
	arr, ok := raw.([]any)
	if !ok {
		return fmt.Errorf("%w: item %d %s must be an array", ErrSuggestProposalItems, item, field)
	}
	if required && len(arr) == 0 {
		return fmt.Errorf("%w: item %d has no %s steps", ErrSuggestProposalItems, item, field)
	}
	for j, v := range arr {
		s, ok := v.(string)
		if !ok {
			return fmt.Errorf("%w: item %d %s[%d] is not a string", ErrSuggestProposalItems, item, field, j+1)
		}
		if strings.TrimSpace(s) == "" {
			return fmt.Errorf("%w: item %d %s[%d] is blank", ErrSuggestProposalItems, item, field, j+1)
		}
	}
	return nil
}

// extractSuggestItems returns the items array and the indices the
// founder accepted (as annotated by the review step in ai_loop).
// If _accepted is absent, every item is considered accepted — handy
// for tests and for the DecisionEditList path where the founder
// implicitly approves whatever they typed.
func extractSuggestItems(payload map[string]any) ([]map[string]any, []int, error) {
	rawItems, ok := payload["items"].([]any)
	if !ok {
		return nil, nil, ErrSuggestProposalItems
	}
	items := make([]map[string]any, 0, len(rawItems))
	for i, r := range rawItems {
		m, ok := r.(map[string]any)
		if !ok {
			return nil, nil, fmt.Errorf("%w: item %d is not an object", ErrSuggestProposalItems, i+1)
		}
		items = append(items, m)
	}
	accepted := extractAccepted(payload, len(items))
	return items, accepted, nil
}

// extractAccepted reads the _accepted annotation. If missing, all
// indices are returned.
func extractAccepted(payload map[string]any, n int) []int {
	raw, ok := payload["_accepted"]
	if !ok {
		all := make([]int, n)
		for i := range all {
			all[i] = i
		}
		return all
	}
	switch v := raw.(type) {
	case []int:
		return v
	case []any:
		out := make([]int, 0, len(v))
		for _, x := range v {
			switch n := x.(type) {
			case int:
				out = append(out, n)
			case float64:
				out = append(out, int(n))
			}
		}
		return out
	}
	return nil
}

// renderSuggestProposal turns a {items:[...]} payload into a multi-
// item Proposal. Each item is rendered with its title and description
// (or Given/When/Then for scenarios). RawJSON holds the canonical
// whole-payload JSON for the editor-list flow.
func renderSuggestProposal(payload map[string]any) driven.Proposal {
	rawItems, _ := payload["items"].([]any)
	proposalItems := make([]driven.ProposalItem, 0, len(rawItems))
	for _, r := range rawItems {
		m, _ := r.(map[string]any)
		proposalItems = append(proposalItems, renderSuggestItem(m))
	}
	return driven.Proposal{
		Format:   driven.ProposalMultiple,
		Multiple: proposalItems,
		RawJSON:  marshalSuggestPayload(payload),
	}
}

func renderSuggestItem(it map[string]any) driven.ProposalItem {
	title, _ := it["title"].(string)
	var b strings.Builder
	if desc, ok := it["description"].(string); ok && strings.TrimSpace(desc) != "" {
		b.WriteString(strings.TrimSpace(desc))
		b.WriteString("\n")
	}
	if given := plainStepStrings(it["given"]); len(given) > 0 {
		b.WriteString("  Given:\n")
		for _, s := range given {
			fmt.Fprintf(&b, "    - %s\n", s)
		}
	}
	if when := plainStepStrings(it["when"]); len(when) > 0 {
		b.WriteString("  When:\n")
		for _, s := range when {
			fmt.Fprintf(&b, "    - %s\n", s)
		}
	}
	if then := plainStepStrings(it["then"]); len(then) > 0 {
		b.WriteString("  Then:\n")
		for _, s := range then {
			fmt.Fprintf(&b, "    - %s\n", s)
		}
	}
	return driven.ProposalItem{
		Title: strings.TrimSpace(title),
		Body:  strings.TrimRight(b.String(), "\n"),
	}
}

func plainStepStrings(raw any) []string {
	arr, ok := raw.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(arr))
	for _, v := range arr {
		if s, ok := v.(string); ok {
			s = strings.TrimSpace(s)
			if s != "" {
				out = append(out, s)
			}
		}
	}
	return out
}

// marshalSuggestPayload renders the editable JSON the founder sees
// when they pick [e]dit on a multi-item review. Keeping it simple —
// no ordering guarantees, no pretty-printing of keys we can't
// reflect on — just whatever the JSON encoder produces.
func marshalSuggestPayload(payload map[string]any) string {
	// Strip the _accepted annotation before handing the buffer to the
	// editor — it's an internal marker, not user-visible content.
	clone := make(map[string]any, len(payload))
	for k, v := range payload {
		if k == "_accepted" {
			continue
		}
		clone[k] = v
	}
	b, err := json.MarshalIndent(clone, "", "  ")
	if err != nil {
		return ""
	}
	return string(b) + "\n"
}
