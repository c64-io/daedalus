package driving

import (
	"context"

	"github.com/c64-io/daedalus/internal/core/domain"
)

// ExpandIdeaRequest asks the AI to help the founder flesh out an
// Idea's Description through an interactive dialog. The Idea must
// already exist; ExpandIdea never creates one. On acceptance, the
// Idea's Description is replaced with the approved text.
type ExpandIdeaRequest struct {
	RootDir string // workspace root; empty = cwd
	IdeaID  string // e.g. "IDEA-001", required
	Model   string // optional override; empty = service default
}

// ExpandEpicRequest asks the AI to help the founder flesh out an
// Epic's Description through an interactive dialog. The Epic must
// already exist; ExpandEpic never creates one. On acceptance, the
// Epic's Description is replaced with the approved text.
type ExpandEpicRequest struct {
	RootDir string
	EpicID  string // e.g. "EPIC-001", required
	Model   string
}

// ExpandFeatureRequest asks the AI to help the founder flesh out a
// Feature's Description through an interactive dialog. The Feature
// must already exist; ExpandFeature never creates one. On acceptance,
// the Feature's Description is replaced with the approved text.
type ExpandFeatureRequest struct {
	RootDir   string
	FeatureID string // e.g. "FEAT-001", required
	Model     string
}

// ExpandStoryRequest asks the AI to help the founder flesh out a
// Story's Description field through an interactive dialog. The Story
// must already exist; ExpandStory never creates one. On acceptance,
// the Story's Description is replaced with the approved text.
type ExpandStoryRequest struct {
	RootDir string
	StoryID string // e.g. "STORY-001", required
	Model   string
}

// IdeaExpander drives the "expand idea" AI dialog.
type IdeaExpander interface {
	ExpandIdea(ctx context.Context, req ExpandIdeaRequest) (*domain.Idea, error)
}

// EpicExpander drives the "expand epic" AI dialog.
type EpicExpander interface {
	ExpandEpic(ctx context.Context, req ExpandEpicRequest) (*domain.Epic, error)
}

// FeatureExpander drives the "expand feature" AI dialog.
type FeatureExpander interface {
	ExpandFeature(ctx context.Context, req ExpandFeatureRequest) (*domain.Feature, error)
}

// StoryExpander drives the "expand story" AI dialog.
//
// Abort is a clean exit signaled via service.ErrAIDialogAborted;
// callers in the CLI layer translate that into a non-error exit
// message. The same contract applies to the other three expanders.
type StoryExpander interface {
	ExpandStory(ctx context.Context, req ExpandStoryRequest) (*domain.Story, error)
}

// Expander is the combined AI-expand surface. It exists solely to let
// the root-command wiring accept a single bundled parameter instead
// of four parallel ones. Individual CLI subcommand builders still
// accept the narrow port they actually need, so the contracts at
// each subcommand call site remain minimal.
type Expander interface {
	IdeaExpander
	EpicExpander
	FeatureExpander
	StoryExpander
}

// ---------------------------------------------------------------------
// d7 suggest — bulk drafting of child entities under an existing parent.
// ---------------------------------------------------------------------

// SuggestEpicsRequest asks the AI to propose several Epics under a
// parent Idea. The Idea must already exist and be at least `refined`
// (the same gate the CreateEpic use case enforces). On acceptance, the
// approved items are created as new Epics; rejected items vanish.
type SuggestEpicsRequest struct {
	RootDir string // workspace root; empty = cwd
	IdeaID  string // parent, required
	Model   string // optional override; empty = service default
}

// SuggestFeaturesRequest asks the AI to propose several Features under
// a parent Epic.
type SuggestFeaturesRequest struct {
	RootDir string
	EpicID  string // parent, required
	Model   string
}

// SuggestStoriesRequest asks the AI to propose several Stories under a
// parent Feature.
type SuggestStoriesRequest struct {
	RootDir   string
	FeatureID string // parent, required
	Model     string
}

// SuggestSpecsRequest asks the AI to propose several Specs under a
// parent Story.
type SuggestSpecsRequest struct {
	RootDir string
	StoryID string // parent, required
	Model   string
}

// SuggestScenariosRequest asks the AI to propose several Scenarios
// under a parent Spec. Scenarios are special-cased because their
// payload carries Given/When/Then steps rather than a description.
type SuggestScenariosRequest struct {
	RootDir string
	SpecID  string // parent, required
	Model   string
}

// EpicsSuggester drives the "suggest epics" AI dialog.
type EpicsSuggester interface {
	SuggestEpics(ctx context.Context, req SuggestEpicsRequest) ([]domain.Epic, error)
}

// FeaturesSuggester drives the "suggest features" AI dialog.
type FeaturesSuggester interface {
	SuggestFeatures(ctx context.Context, req SuggestFeaturesRequest) ([]domain.Feature, error)
}

// StoriesSuggester drives the "suggest stories" AI dialog.
type StoriesSuggester interface {
	SuggestStories(ctx context.Context, req SuggestStoriesRequest) ([]domain.Story, error)
}

// SpecsSuggester drives the "suggest specs" AI dialog.
type SpecsSuggester interface {
	SuggestSpecs(ctx context.Context, req SuggestSpecsRequest) ([]domain.Spec, error)
}

// ScenariosSuggester drives the "suggest scenarios" AI dialog.
type ScenariosSuggester interface {
	SuggestScenarios(ctx context.Context, req SuggestScenariosRequest) ([]domain.Scenario, error)
}

// Suggester is the combined AI-suggest surface, bundled exactly like
// Expander so the root-command wiring takes one parameter. Individual
// CLI subcommand builders still narrow down to the verb they implement.
type Suggester interface {
	EpicsSuggester
	FeaturesSuggester
	StoriesSuggester
	SpecsSuggester
	ScenariosSuggester
}
