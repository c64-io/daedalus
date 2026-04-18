package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/c64-io/daedalus/internal/core/domain"
	"github.com/c64-io/daedalus/internal/core/port/driven"
	"github.com/c64-io/daedalus/internal/core/port/driving"
)

// Shared helpers (ToContextEntity converters, sizeString, loadWorkspace,
// loadTargetRefs) live in ai_context.go so SuggestService can reuse them.

// Sentinel errors shared by every expand-* command.
var (
	ErrExpandIDRequired          = errors.New("entity ID is required")
	ErrExpandProposalDescription = errors.New("proposal is missing a description string")
)

// Defaults shared by every AI command service. Tuned for a solo-founder
// interview flow: many questions are expected, and network hiccups
// should retry a couple of times before giving up.
const (
	defaultExpandModel           = "claude-haiku-4-5-20251001"
	defaultExpandMaxInterviews   = 10
	defaultExpandMalformedBudget = 2
	defaultExpandBackoffBudget   = 3
	defaultExpandBackoffBase     = 2 * time.Second
	defaultExpandMaxTokens       = 4096
)

// Compile-time assertions — one per driving port the service satisfies.
var (
	_ driving.IdeaExpander    = (*ExpandService)(nil)
	_ driving.EpicExpander    = (*ExpandService)(nil)
	_ driving.FeatureExpander = (*ExpandService)(nil)
	_ driving.StoryExpander   = (*ExpandService)(nil)
)

// ExpandService wires the AI-assisted expansion use cases for all four
// planning-hierarchy entities (Idea, Epic, Feature, Story). Each
// ExpandX method builds a minimal DialogContext (workspace + target),
// constructs a NavToolsHandler for on-demand graph exploration, and
// runs the interview-first AI dialog via runDialog. On acceptance it
// updates the entity's Description field.
type ExpandService struct {
	fs        driven.FileSystem
	wsRepo    driven.WorkspaceRepository
	ideaRepo  driven.IdeaRepository
	epicRepo  driven.EpicRepository
	featRepo  driven.FeatureRepository
	storyRepo driven.StoryRepository
	specRepo  driven.SpecRepository
	scenRepo  driven.ScenarioRepository
	linkRepo  driven.LinkRepository
	refRepo   driven.RefRepository
	resolver  driven.EntityResolver
	history   driven.HistoryRepository

	llm    driven.AIAssistant
	inter  driven.Interaction
	status driven.StatusRenderer
	clock  driven.Clock
}

// NewExpandService wires the expand service with all of its driven
// dependencies.
func NewExpandService(
	fs driven.FileSystem,
	wsRepo driven.WorkspaceRepository,
	ideaRepo driven.IdeaRepository,
	epicRepo driven.EpicRepository,
	featRepo driven.FeatureRepository,
	storyRepo driven.StoryRepository,
	specRepo driven.SpecRepository,
	scenRepo driven.ScenarioRepository,
	linkRepo driven.LinkRepository,
	refRepo driven.RefRepository,
	resolver driven.EntityResolver,
	history driven.HistoryRepository,
	llm driven.AIAssistant,
	inter driven.Interaction,
	status driven.StatusRenderer,
	clock driven.Clock,
) *ExpandService {
	return &ExpandService{
		fs:        fs,
		wsRepo:    wsRepo,
		ideaRepo:  ideaRepo,
		epicRepo:  epicRepo,
		featRepo:  featRepo,
		storyRepo: storyRepo,
		specRepo:  specRepo,
		scenRepo:  scenRepo,
		linkRepo:  linkRepo,
		refRepo:   refRepo,
		resolver:  resolver,
		history:   history,
		llm:       llm,
		inter:     inter,
		status:    status,
		clock:     clock,
	}
}

// ---------------------------------------------------------------------
// Public use-case methods — one per entity type.
// ---------------------------------------------------------------------

// ExpandIdea runs the interactive dialog to draft an Idea's Description.
func (s *ExpandService) ExpandIdea(ctx context.Context, req driving.ExpandIdeaRequest) (*domain.Idea, error) {
	if req.IdeaID == "" {
		return nil, ErrExpandIDRequired
	}
	ws, err := requireWorkspace(s.fs, req.RootDir)
	if err != nil {
		return nil, err
	}
	idea, err := s.ideaRepo.GetIdea(ctx, ws.DBDir, req.IdeaID)
	if err != nil {
		return nil, fmt.Errorf("get idea: %w", err)
	}
	dc, err := s.buildTargetContext(ctx, ws.DBDir, idea.ID, ideaToContextEntity(*idea))
	if err != nil {
		return nil, fmt.Errorf("build dialog context: %w", err)
	}
	result, err := s.runExpand(ctx, runExpandParams{
		command:           "expand idea",
		target:            idea.ID,
		model:             req.Model,
		systemPrompt:      expandIdeaSystemPrompt,
		submitDescription: expandIdeaSubmitDescription,
		dc:                dc,
		dbDir:             ws.DBDir,
		apply: func(ctx context.Context, payload map[string]any) (any, error) {
			return s.applyExpandIdea(ctx, ws.DBDir, idea.ID, payload)
		},
	})
	if err != nil {
		return nil, err
	}
	updated, ok := result.(*domain.Idea)
	if !ok || updated == nil {
		return nil, fmt.Errorf("expand idea: unexpected apply result %T", result)
	}
	return updated, nil
}

// ExpandEpic runs the interactive dialog to draft an Epic's Description.
func (s *ExpandService) ExpandEpic(ctx context.Context, req driving.ExpandEpicRequest) (*domain.Epic, error) {
	if req.EpicID == "" {
		return nil, ErrExpandIDRequired
	}
	ws, err := requireWorkspace(s.fs, req.RootDir)
	if err != nil {
		return nil, err
	}
	epic, err := s.epicRepo.GetEpic(ctx, ws.DBDir, req.EpicID)
	if err != nil {
		return nil, fmt.Errorf("get epic: %w", err)
	}
	dc, err := s.buildTargetContext(ctx, ws.DBDir, epic.ID, epicToContextEntity(*epic))
	if err != nil {
		return nil, fmt.Errorf("build dialog context: %w", err)
	}
	result, err := s.runExpand(ctx, runExpandParams{
		command:           "expand epic",
		target:            epic.ID,
		model:             req.Model,
		systemPrompt:      expandEpicSystemPrompt,
		submitDescription: expandEpicSubmitDescription,
		dc:                dc,
		dbDir:             ws.DBDir,
		apply: func(ctx context.Context, payload map[string]any) (any, error) {
			return s.applyExpandEpic(ctx, ws.DBDir, epic.ID, payload)
		},
	})
	if err != nil {
		return nil, err
	}
	updated, ok := result.(*domain.Epic)
	if !ok || updated == nil {
		return nil, fmt.Errorf("expand epic: unexpected apply result %T", result)
	}
	return updated, nil
}

// ExpandFeature runs the interactive dialog to draft a Feature's Description.
func (s *ExpandService) ExpandFeature(ctx context.Context, req driving.ExpandFeatureRequest) (*domain.Feature, error) {
	if req.FeatureID == "" {
		return nil, ErrExpandIDRequired
	}
	ws, err := requireWorkspace(s.fs, req.RootDir)
	if err != nil {
		return nil, err
	}
	feat, err := s.featRepo.GetFeature(ctx, ws.DBDir, req.FeatureID)
	if err != nil {
		return nil, fmt.Errorf("get feature: %w", err)
	}
	dc, err := s.buildTargetContext(ctx, ws.DBDir, feat.ID, featureToContextEntity(*feat))
	if err != nil {
		return nil, fmt.Errorf("build dialog context: %w", err)
	}
	result, err := s.runExpand(ctx, runExpandParams{
		command:           "expand feature",
		target:            feat.ID,
		model:             req.Model,
		systemPrompt:      expandFeatureSystemPrompt,
		submitDescription: expandFeatureSubmitDescription,
		dc:                dc,
		dbDir:             ws.DBDir,
		apply: func(ctx context.Context, payload map[string]any) (any, error) {
			return s.applyExpandFeature(ctx, ws.DBDir, feat.ID, payload)
		},
	})
	if err != nil {
		return nil, err
	}
	updated, ok := result.(*domain.Feature)
	if !ok || updated == nil {
		return nil, fmt.Errorf("expand feature: unexpected apply result %T", result)
	}
	return updated, nil
}

// ExpandStory runs the interactive dialog to draft a Story's Description.
func (s *ExpandService) ExpandStory(ctx context.Context, req driving.ExpandStoryRequest) (*domain.Story, error) {
	if req.StoryID == "" {
		return nil, ErrExpandIDRequired
	}
	ws, err := requireWorkspace(s.fs, req.RootDir)
	if err != nil {
		return nil, err
	}
	story, err := s.storyRepo.GetStory(ctx, ws.DBDir, req.StoryID)
	if err != nil {
		return nil, fmt.Errorf("get story: %w", err)
	}
	dc, err := s.buildTargetContext(ctx, ws.DBDir, story.ID, storyToContextEntity(*story))
	if err != nil {
		return nil, fmt.Errorf("build dialog context: %w", err)
	}
	result, err := s.runExpand(ctx, runExpandParams{
		command:           "expand story",
		target:            story.ID,
		model:             req.Model,
		systemPrompt:      expandStorySystemPrompt,
		submitDescription: expandStorySubmitDescription,
		dc:                dc,
		dbDir:             ws.DBDir,
		apply: func(ctx context.Context, payload map[string]any) (any, error) {
			return s.applyExpandStory(ctx, ws.DBDir, story.ID, payload)
		},
	})
	if err != nil {
		return nil, err
	}
	updated, ok := result.(*domain.Story)
	if !ok || updated == nil {
		return nil, fmt.Errorf("expand story: unexpected apply result %T", result)
	}
	return updated, nil
}

// ---------------------------------------------------------------------
// Shared dialog runner.
// ---------------------------------------------------------------------

type runExpandParams struct {
	command           string
	target            string
	model             string
	systemPrompt      string
	submitDescription string
	dc                domain.DialogContext
	dbDir             string
	apply             func(ctx context.Context, payload map[string]any) (any, error)
}

// runExpand builds and runs the shared dialog loop. Every Expand* method
// routes through here; the only per-entity variation is the system
// prompt, the submit_proposal tool description, and the apply callback.
func (s *ExpandService) runExpand(ctx context.Context, p runExpandParams) (any, error) {
	model := p.model
	if model == "" {
		model = defaultExpandModel
	}

	handler := &NavToolsHandler{
		ideaRepo:  s.ideaRepo,
		epicRepo:  s.epicRepo,
		featRepo:  s.featRepo,
		storyRepo: s.storyRepo,
		specRepo:  s.specRepo,
		scenRepo:  s.scenRepo,
		linkRepo:  s.linkRepo,
		refRepo:   s.refRepo,
		resolver:  s.resolver,
		dbDir:     p.dbDir,
	}

	tools := expandTools(p.submitDescription)
	tools = append(tools, navToolDefs()...)

	dlg := &aiDialog{
		command:          p.command,
		target:           p.target,
		model:            model,
		system:           p.systemPrompt + navOrientationClause,
		contextBlock:     domain.FormatDialogContext(p.dc),
		tools:            tools,
		maxTokensPerTurn: defaultExpandMaxTokens,
		maxInterviews:    defaultExpandMaxInterviews,
		malformedBudget:  defaultExpandMalformedBudget,
		backoffBudget:    defaultExpandBackoffBudget,
		backoffBase:      defaultExpandBackoffBase,

		navHandler:     handler.Handle,
		validate:       validateExpandPayload,
		renderProposal: renderExpandProposal,
		apply:          p.apply,
	}
	return runDialog(ctx, dlg, s.llm, s.inter, s.status, s.clock)
}

// buildTargetContext builds a DialogContext with just the workspace
// description, the target entity, and its refs. Ancestors and links
// are not included — the model explores them via navigation tools.
func (s *ExpandService) buildTargetContext(ctx context.Context, dbDir, entityID string, target domain.DialogContextEntity) (domain.DialogContext, error) {
	ws, err := loadWorkspace(ctx, s.wsRepo, dbDir)
	if err != nil {
		return domain.DialogContext{}, err
	}
	refs, err := loadTargetRefs(ctx, s.refRepo, dbDir, entityID)
	if err != nil {
		return domain.DialogContext{}, err
	}
	return domain.DialogContext{
		Workspace: ws,
		Target:    target,
		Refs:      refs,
	}, nil
}

// ---------------------------------------------------------------------
// Per-entity apply — each persists the accepted description and
// records a history entry. The duplication is minor and avoids
// fighting Go's generics over method sets.
// ---------------------------------------------------------------------

func (s *ExpandService) applyExpandIdea(ctx context.Context, dbDir string, id string, payload map[string]any) (any, error) {
	desc, err := extractDescription(payload)
	if err != nil {
		return nil, err
	}
	idea, err := s.ideaRepo.GetIdea(ctx, dbDir, id)
	if err != nil {
		return nil, fmt.Errorf("get idea for apply: %w", err)
	}
	if desc == idea.Description {
		return idea, nil
	}
	now := s.clock.Now()
	entry := domain.HistoryEntry{EntityID: id, Field: "description", OldValue: idea.Description, NewValue: desc, Timestamp: now}
	idea.Description = desc
	if err := s.ideaRepo.UpdateIdea(ctx, dbDir, *idea); err != nil {
		return nil, fmt.Errorf("update idea: %w", err)
	}
	if err := s.history.AppendHistory(ctx, dbDir, []domain.HistoryEntry{entry}); err != nil {
		return nil, fmt.Errorf("record history: %w", err)
	}
	return idea, nil
}

func (s *ExpandService) applyExpandEpic(ctx context.Context, dbDir string, id string, payload map[string]any) (any, error) {
	desc, err := extractDescription(payload)
	if err != nil {
		return nil, err
	}
	epic, err := s.epicRepo.GetEpic(ctx, dbDir, id)
	if err != nil {
		return nil, fmt.Errorf("get epic for apply: %w", err)
	}
	if desc == epic.Description {
		return epic, nil
	}
	now := s.clock.Now()
	entry := domain.HistoryEntry{EntityID: id, Field: "description", OldValue: epic.Description, NewValue: desc, Timestamp: now}
	epic.Description = desc
	if err := s.epicRepo.UpdateEpic(ctx, dbDir, *epic); err != nil {
		return nil, fmt.Errorf("update epic: %w", err)
	}
	if err := s.history.AppendHistory(ctx, dbDir, []domain.HistoryEntry{entry}); err != nil {
		return nil, fmt.Errorf("record history: %w", err)
	}
	return epic, nil
}

func (s *ExpandService) applyExpandFeature(ctx context.Context, dbDir string, id string, payload map[string]any) (any, error) {
	desc, err := extractDescription(payload)
	if err != nil {
		return nil, err
	}
	feat, err := s.featRepo.GetFeature(ctx, dbDir, id)
	if err != nil {
		return nil, fmt.Errorf("get feature for apply: %w", err)
	}
	if desc == feat.Description {
		return feat, nil
	}
	now := s.clock.Now()
	entry := domain.HistoryEntry{EntityID: id, Field: "description", OldValue: feat.Description, NewValue: desc, Timestamp: now}
	feat.Description = desc
	if err := s.featRepo.UpdateFeature(ctx, dbDir, *feat); err != nil {
		return nil, fmt.Errorf("update feature: %w", err)
	}
	if err := s.history.AppendHistory(ctx, dbDir, []domain.HistoryEntry{entry}); err != nil {
		return nil, fmt.Errorf("record history: %w", err)
	}
	return feat, nil
}

func (s *ExpandService) applyExpandStory(ctx context.Context, dbDir string, id string, payload map[string]any) (any, error) {
	desc, err := extractDescription(payload)
	if err != nil {
		return nil, err
	}
	story, err := s.storyRepo.GetStory(ctx, dbDir, id)
	if err != nil {
		return nil, fmt.Errorf("get story for apply: %w", err)
	}
	if desc == story.Description {
		return story, nil
	}
	now := s.clock.Now()
	entry := domain.HistoryEntry{EntityID: id, Field: "description", OldValue: story.Description, NewValue: desc, Timestamp: now}
	story.Description = desc
	if err := s.storyRepo.UpdateStory(ctx, dbDir, *story); err != nil {
		return nil, fmt.Errorf("update story: %w", err)
	}
	if err := s.history.AppendHistory(ctx, dbDir, []domain.HistoryEntry{entry}); err != nil {
		return nil, fmt.Errorf("record history: %w", err)
	}
	return story, nil
}

// ---------------------------------------------------------------------
// Shared proposal helpers.
// ---------------------------------------------------------------------

// expandTools returns the two tool definitions every expand-* command
// uses. The submit_proposal description is entity-specific; the
// schemas are identical across all four entities.
func expandTools(submitDescription string) []driven.Tool {
	return []driven.Tool{
		{
			Name:        driven.ToolAskQuestion,
			Description: askQuestionDescription,
			InputSchema: askQuestionSchema,
		},
		{
			Name:        driven.ToolSubmitProposal,
			Description: submitDescription,
			InputSchema: expandDescriptionSchema,
		},
	}
}

// validateExpandPayload enforces the submit_proposal schema for every
// expand-* command: a single required non-empty "description" string.
func validateExpandPayload(payload map[string]any) error {
	if payload == nil {
		return ErrExpandProposalDescription
	}
	raw, ok := payload["description"]
	if !ok {
		return ErrExpandProposalDescription
	}
	s, ok := raw.(string)
	if !ok {
		return fmt.Errorf("%w: description must be a string", ErrExpandProposalDescription)
	}
	if strings.TrimSpace(s) == "" {
		return fmt.Errorf("%w: description must not be empty", ErrExpandProposalDescription)
	}
	for k := range payload {
		if k != "description" {
			return fmt.Errorf("proposal has unexpected key %q", k)
		}
	}
	return nil
}

// extractDescription is the apply-side counterpart to validate.
func extractDescription(payload map[string]any) (string, error) {
	raw, ok := payload["description"]
	if !ok {
		return "", ErrExpandProposalDescription
	}
	s, ok := raw.(string)
	if !ok {
		return "", fmt.Errorf("%w: description must be a string", ErrExpandProposalDescription)
	}
	return strings.TrimSpace(s), nil
}

// renderExpandProposal turns the raw payload into a user-facing
// Proposal. For editor flows the JSON is what loads into the buffer;
// the Body is what the CLI prints to stdout. Shared across all four
// expand commands because the proposal shape is identical.
func renderExpandProposal(payload map[string]any) driven.Proposal {
	desc, _ := payload["description"].(string)
	body := strings.TrimSpace(desc)
	rawJSON := `{"description": ` + quoteJSONString(body) + "}\n"
	return driven.Proposal{
		Format: driven.ProposalSingle,
		Single: &driven.ProposalItem{
			Title: "Drafted description",
			Body:  body,
			JSON:  rawJSON,
		},
		RawJSON: rawJSON,
	}
}

// quoteJSONString encodes s as a JSON string literal (with surrounding
// double quotes).
func quoteJSONString(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '\\':
			b.WriteString(`\\`)
		case '"':
			b.WriteString(`\"`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			if r < 0x20 {
				fmt.Fprintf(&b, `\u%04x`, r)
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('"')
	return b.String()
}

