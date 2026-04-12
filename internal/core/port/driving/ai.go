package driving

import (
	"context"

	"github.com/c64-io/daedalus/internal/core/domain"
)

// ExpandStoryRequest asks the AI to help the founder flesh out a
// Story's Description field through an interactive dialog. The Story
// must already exist; ExpandStory never creates one. On acceptance,
// the Story's Description is replaced with the approved text.
type ExpandStoryRequest struct {
	RootDir string // workspace root; empty = cwd
	StoryID string // e.g. "STORY-001", required
	Model   string // optional override; empty = service default
}

// StoryExpander is the driving port for running the "expand story"
// AI dialog. Implementations drive the dialog loop (ask_question,
// submit_proposal, review, apply) and return the updated Story
// on acceptance.
//
// Abort is a clean exit and is signaled with driven.ErrAIDialogAborted;
// callers in the CLI layer translate that into a non-error exit
// message.
type StoryExpander interface {
	ExpandStory(ctx context.Context, req ExpandStoryRequest) (*domain.Story, error)
}
