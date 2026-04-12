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

// Sentinel errors specific to expand-story.
var (
	ErrExpandStoryIDRequired     = errors.New("story ID is required")
	ErrExpandProposalDescription = errors.New("proposal is missing a description string")
)

// Defaults shared by every AI command service. These are tuned for a
// solo-founder interview flow: many questions are expected, and
// network hiccups should retry a couple of times before giving up.
const (
	defaultExpandModel           = "claude-haiku-4-5-20251001"
	defaultExpandMaxInterviews   = 10
	defaultExpandMalformedBudget = 2
	defaultExpandBackoffBudget   = 3
	defaultExpandBackoffBase     = 2 * time.Second
	defaultExpandMaxTokens       = 4096
)

// Compile-time assertion.
var _ driving.StoryExpander = (*StoryExpandService)(nil)

// StoryExpandService wires the expand-story use case. It gathers the
// full parent chain for the target Story (idea → epic → feature),
// cross-links, refs, and workspace description into a DialogContext,
// runs the interview-first AI dialog via runDialog, and on acceptance
// updates the Story's Description.
type StoryExpandService struct {
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

// NewStoryExpandService wires the expand-story service with all of
// its driven dependencies. Every dependency is a port; the service
// holds no concrete types.
func NewStoryExpandService(
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
) *StoryExpandService {
	return &StoryExpandService{
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

// ExpandStory runs the interactive dialog to draft a Story's
// Description. Returns the updated Story on acceptance,
// ErrAIDialogAborted if the founder aborted (no DB writes), or a
// wrapped error otherwise.
func (s *StoryExpandService) ExpandStory(ctx context.Context, req driving.ExpandStoryRequest) (*domain.Story, error) {
	if req.StoryID == "" {
		return nil, ErrExpandStoryIDRequired
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

	model := req.Model
	if model == "" {
		model = defaultExpandModel
	}

	dlg := &aiDialog{
		command:          "expand story",
		target:           story.ID,
		model:            model,
		system:           expandStorySystemPrompt,
		contextBlock:     domain.FormatDialogContext(dc),
		tools:            expandStoryTools(),
		maxTokensPerTurn: defaultExpandMaxTokens,
		maxInterviews:    defaultExpandMaxInterviews,
		malformedBudget:  defaultExpandMalformedBudget,
		backoffBudget:    defaultExpandBackoffBudget,
		backoffBase:      defaultExpandBackoffBase,

		validate:       validateExpandStoryPayload,
		renderProposal: renderExpandStoryProposal,
		apply: func(ctx context.Context, payload map[string]any) (any, error) {
			return s.applyExpandStory(ctx, ws.DBDir, story.ID, payload)
		},
	}

	result, err := runDialog(ctx, dlg, s.llm, s.inter, s.status, s.clock)
	if err != nil {
		return nil, err
	}
	updated, ok := result.(*domain.Story)
	if !ok || updated == nil {
		return nil, fmt.Errorf("expand story: unexpected apply result %T", result)
	}
	return updated, nil
}

// buildStoryContext walks from the Story up to the Idea root, gathers
// cross-links and external refs touching the Story, and pulls the
// workspace description, returning a DialogContext ready for
// FormatDialogContext.
func (s *StoryExpandService) buildStoryContext(ctx context.Context, dbDir string, story domain.Story) (domain.DialogContext, error) {
	var dc domain.DialogContext

	// Workspace description is best-effort; a missing one renders as
	// "(no project description yet)" by FormatDialogContext.
	if desc, err := s.wsRepo.ReadProjectDescription(ctx, dbDir); err == nil {
		dc.Workspace = desc
	} else if !errors.Is(err, driven.ErrProjectDescriptionNotFound) {
		return dc, fmt.Errorf("read project description: %w", err)
	}

	// Ancestors: Idea → Epic → Feature.
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

	// Links touching this story.
	links, err := s.linkRepo.ListLinksByEntity(ctx, dbDir, story.ID)
	if err != nil {
		return dc, fmt.Errorf("list links: %w", err)
	}
	for _, l := range links {
		dcl, err := s.resolveDialogLink(ctx, dbDir, story.ID, l)
		if err != nil {
			return dc, err
		}
		dc.Links = append(dc.Links, dcl)
	}

	// External refs.
	refs, err := s.refRepo.ListRefsByEntity(ctx, dbDir, story.ID)
	if err != nil {
		return dc, fmt.Errorf("list refs: %w", err)
	}
	for _, r := range refs {
		dc.Refs = append(dc.Refs, domain.DialogContextRef{URL: r.URL, Label: r.Label})
	}

	return dc, nil
}

// resolveDialogLink turns a stored Link into a DialogContextLink with
// the "other" endpoint resolved to its title and the relation label
// direction-normalized relative to this story.
func (s *StoryExpandService) resolveDialogLink(ctx context.Context, dbDir string, selfID string, l domain.Link) (domain.DialogContextLink, error) {
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
		// Shouldn't happen — the repo returns links touching selfID.
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

// applyExpandStory persists the accepted description via SetStory
// semantics (history entry included).
func (s *StoryExpandService) applyExpandStory(ctx context.Context, dbDir string, storyID string, payload map[string]any) (any, error) {
	desc, err := extractDescription(payload)
	if err != nil {
		return nil, err
	}

	story, err := s.storyRepo.GetStory(ctx, dbDir, storyID)
	if err != nil {
		return nil, fmt.Errorf("get story for apply: %w", err)
	}
	if desc == story.Description {
		return story, nil
	}

	now := s.clock.Now()
	entry := domain.HistoryEntry{
		EntityID:  storyID,
		Field:     "description",
		OldValue:  story.Description,
		NewValue:  desc,
		Timestamp: now,
	}
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
// Shared helpers (small and focused; widened when more AI services
// land in later commits).
// ---------------------------------------------------------------------

// expandStoryTools returns the tool definitions for the expand-story
// command. Every AI command in v1 uses exactly these two tools.
func expandStoryTools() []driven.Tool {
	return []driven.Tool{
		{
			Name:        driven.ToolAskQuestion,
			Description: askQuestionDescription,
			InputSchema: askQuestionSchema,
		},
		{
			Name:        driven.ToolSubmitProposal,
			Description: expandStorySubmitDescription,
			InputSchema: expandStorySchema,
		},
	}
}

// validateExpandStoryPayload enforces the submit_proposal schema for
// expand_story: a single required "description" string, non-empty
// after trim. Extra keys are rejected to keep the model honest.
func validateExpandStoryPayload(payload map[string]any) error {
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

// extractDescription is the apply-side counterpart to validate — it
// assumes the payload has already been validated and pulls the string
// out defensively.
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

// renderExpandStoryProposal turns the raw payload into a user-facing
// Proposal. For editor flows the JSON is what loads into the buffer;
// the Body is what the CLI prints to stdout.
func renderExpandStoryProposal(payload map[string]any) driven.Proposal {
	desc, _ := payload["description"].(string)
	body := strings.TrimSpace(desc)

	// The raw JSON buffer is what the editor-based Interaction loads
	// if the founder picks "edit" at review time. Keep it minimal: one
	// field, same name as the schema.
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

// quoteJSONString encodes s as a JSON string literal (including the
// surrounding double quotes). Extracted so it can be reused by the
// proposal renderers for other commands without pulling in a helper
// package.
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
// Typed → DialogContextEntity converters
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
