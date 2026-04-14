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
// ExpandX method walks that entity's parent chain into a DialogContext,
// runs the interview-first AI dialog via runDialog, and on acceptance
// updates the entity's Description field.
type ExpandService struct {
	fs        driven.FileSystem
	wsRepo    driven.WorkspaceRepository
	ideaRepo  driven.IdeaRepository
	epicRepo  driven.EpicRepository
	featRepo  driven.FeatureRepository
	storyRepo driven.StoryRepository
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
// dependencies. Every dependency is a port; the service holds no
// concrete types.
func NewExpandService(
	fs driven.FileSystem,
	wsRepo driven.WorkspaceRepository,
	ideaRepo driven.IdeaRepository,
	epicRepo driven.EpicRepository,
	featRepo driven.FeatureRepository,
	storyRepo driven.StoryRepository,
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
	dc, err := s.buildIdeaContext(ctx, ws.DBDir, *idea)
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
	dc, err := s.buildEpicContext(ctx, ws.DBDir, *epic)
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
	dc, err := s.buildFeatureContext(ctx, ws.DBDir, *feat)
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
	dc, err := s.buildStoryContext(ctx, ws.DBDir, *story)
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
	dlg := &aiDialog{
		command:          p.command,
		target:           p.target,
		model:            model,
		system:           p.systemPrompt,
		contextBlock:     domain.FormatDialogContext(p.dc),
		tools:            expandTools(p.submitDescription),
		maxTokensPerTurn: defaultExpandMaxTokens,
		maxInterviews:    defaultExpandMaxInterviews,
		malformedBudget:  defaultExpandMalformedBudget,
		backoffBudget:    defaultExpandBackoffBudget,
		backoffBase:      defaultExpandBackoffBase,

		validate:       validateExpandPayload,
		renderProposal: renderExpandProposal,
		apply:          p.apply,
	}
	return runDialog(ctx, dlg, s.llm, s.inter, s.status, s.clock)
}

// ---------------------------------------------------------------------
// Per-entity context builders.
// ---------------------------------------------------------------------

// buildIdeaContext has no ancestors — just the workspace description,
// links, and refs touching the Idea.
func (s *ExpandService) buildIdeaContext(ctx context.Context, dbDir string, idea domain.Idea) (domain.DialogContext, error) {
	var dc domain.DialogContext
	if err := s.attachCommonContext(ctx, dbDir, idea.ID, &dc); err != nil {
		return dc, err
	}
	dc.Target = ideaToContextEntity(idea)
	return dc, nil
}

// buildEpicContext walks Epic → Idea.
func (s *ExpandService) buildEpicContext(ctx context.Context, dbDir string, epic domain.Epic) (domain.DialogContext, error) {
	var dc domain.DialogContext
	if err := s.attachCommonContext(ctx, dbDir, epic.ID, &dc); err != nil {
		return dc, err
	}
	idea, err := s.ideaRepo.GetIdea(ctx, dbDir, epic.IdeaID)
	if err != nil {
		return dc, fmt.Errorf("get parent idea %s: %w", epic.IdeaID, err)
	}
	dc.Ancestors = []domain.DialogContextEntity{ideaToContextEntity(*idea)}
	dc.Target = epicToContextEntity(epic)
	return dc, nil
}

// buildFeatureContext walks Feature → Epic → Idea.
func (s *ExpandService) buildFeatureContext(ctx context.Context, dbDir string, feat domain.Feature) (domain.DialogContext, error) {
	var dc domain.DialogContext
	if err := s.attachCommonContext(ctx, dbDir, feat.ID, &dc); err != nil {
		return dc, err
	}
	epic, err := s.epicRepo.GetEpic(ctx, dbDir, feat.EpicID)
	if err != nil {
		return dc, fmt.Errorf("get parent epic %s: %w", feat.EpicID, err)
	}
	idea, err := s.ideaRepo.GetIdea(ctx, dbDir, epic.IdeaID)
	if err != nil {
		return dc, fmt.Errorf("get parent idea %s: %w", epic.IdeaID, err)
	}
	dc.Ancestors = []domain.DialogContextEntity{
		ideaToContextEntity(*idea),
		epicToContextEntity(*epic),
	}
	dc.Target = featureToContextEntity(feat)
	return dc, nil
}

// buildStoryContext walks Story → Feature → Epic → Idea.
func (s *ExpandService) buildStoryContext(ctx context.Context, dbDir string, story domain.Story) (domain.DialogContext, error) {
	var dc domain.DialogContext
	if err := s.attachCommonContext(ctx, dbDir, story.ID, &dc); err != nil {
		return dc, err
	}
	feat, err := s.featRepo.GetFeature(ctx, dbDir, story.FeatureID)
	if err != nil {
		return dc, fmt.Errorf("get parent feature %s: %w", story.FeatureID, err)
	}
	epic, err := s.epicRepo.GetEpic(ctx, dbDir, feat.EpicID)
	if err != nil {
		return dc, fmt.Errorf("get parent epic %s: %w", feat.EpicID, err)
	}
	idea, err := s.ideaRepo.GetIdea(ctx, dbDir, epic.IdeaID)
	if err != nil {
		return dc, fmt.Errorf("get parent idea %s: %w", epic.IdeaID, err)
	}
	dc.Ancestors = []domain.DialogContextEntity{
		ideaToContextEntity(*idea),
		epicToContextEntity(*epic),
		featureToContextEntity(*feat),
	}
	dc.Target = storyToContextEntity(story)
	return dc, nil
}

// attachCommonContext populates the workspace description, links, and
// refs — the slice of context every entity shares regardless of depth.
func (s *ExpandService) attachCommonContext(ctx context.Context, dbDir string, selfID string, dc *domain.DialogContext) error {
	if desc, err := s.wsRepo.ReadProjectDescription(ctx, dbDir); err == nil {
		dc.Workspace = desc
	} else if !errors.Is(err, driven.ErrProjectDescriptionNotFound) {
		return fmt.Errorf("read project description: %w", err)
	}

	links, err := s.linkRepo.ListLinksByEntity(ctx, dbDir, selfID)
	if err != nil {
		return fmt.Errorf("list links: %w", err)
	}
	for _, l := range links {
		dcl, err := s.resolveDialogLink(ctx, dbDir, selfID, l)
		if err != nil {
			return err
		}
		dc.Links = append(dc.Links, dcl)
	}

	refs, err := s.refRepo.ListRefsByEntity(ctx, dbDir, selfID)
	if err != nil {
		return fmt.Errorf("list refs: %w", err)
	}
	for _, r := range refs {
		dc.Refs = append(dc.Refs, domain.DialogContextRef{URL: r.URL, Label: r.Label})
	}
	return nil
}

// resolveDialogLink turns a stored Link into a DialogContextLink with
// the "other" endpoint resolved to its title and the relation label
// direction-normalized relative to selfID.
func (s *ExpandService) resolveDialogLink(ctx context.Context, dbDir string, selfID string, l domain.Link) (domain.DialogContextLink, error) {
	var other string
	var relation string
	switch {
	case l.FromID == selfID:
		other = l.ToID
		relation = string(l.Kind)
	case l.ToID == selfID:
		other = l.FromID
		relation = l.Kind.InverseLabel()
	default:
		other = l.ToID
		relation = string(l.Kind)
	}
	title, err := s.resolver.ResolveEntity(ctx, dbDir, other)
	if err != nil && !errors.Is(err, driven.ErrEntityNotFound) {
		return domain.DialogContextLink{}, fmt.Errorf("resolve link endpoint %s: %w", other, err)
	}
	return domain.DialogContextLink{
		Relation:   relation,
		OtherID:    other,
		OtherTitle: title,
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

// ---------------------------------------------------------------------
// Typed → DialogContextEntity converters.
// ---------------------------------------------------------------------

func ideaToContextEntity(i domain.Idea) domain.DialogContextEntity {
	return domain.DialogContextEntity{
		Kind:        "Idea",
		ID:          i.ID,
		Title:       i.Title,
		Status:      string(i.Status),
		Description: i.Description,
	}
}

func epicToContextEntity(e domain.Epic) domain.DialogContextEntity {
	return domain.DialogContextEntity{
		Kind:        "Epic",
		ID:          e.ID,
		Title:       e.Title,
		Status:      string(e.Status),
		Description: e.Description,
		Priority:    string(e.Priority),
		Size:        sizeString(e.Size),
	}
}

func featureToContextEntity(f domain.Feature) domain.DialogContextEntity {
	return domain.DialogContextEntity{
		Kind:        "Feature",
		ID:          f.ID,
		Title:       f.Title,
		Status:      string(f.Status),
		Description: f.Description,
		Priority:    string(f.Priority),
		Size:        sizeString(f.Size),
	}
}

func storyToContextEntity(s domain.Story) domain.DialogContextEntity {
	return domain.DialogContextEntity{
		Kind:        "Story",
		ID:          s.ID,
		Title:       s.Title,
		Status:      string(s.Status),
		Description: s.Description,
		Priority:    string(s.Priority),
		Size:        sizeString(s.Size),
	}
}

// sizeString renders a Size as a Fibonacci string, or "" for unset.
func sizeString(sz domain.Size) string {
	if sz == 0 {
		return ""
	}
	return fmt.Sprintf("%d", int(sz))
}
