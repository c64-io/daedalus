package main

import (
	"context"

	anthropicadapter "github.com/c64-io/daedalus/internal/adapter/driven/anthropic"
	"github.com/c64-io/daedalus/internal/adapter/driven/cliinteraction"
	"github.com/c64-io/daedalus/internal/adapter/driven/ttystatus"
	"github.com/c64-io/daedalus/internal/core/domain"
	"github.com/c64-io/daedalus/internal/core/port/driven"
	"github.com/c64-io/daedalus/internal/core/port/driving"
	"github.com/c64-io/daedalus/internal/core/service"
)

// deferredExpander lazily builds a service.ExpandService on the first
// AI command call. Deferring is important: the Anthropic SDK's
// constructor reads ANTHROPIC_API_KEY at construction time, so
// building the client eagerly at process start would break
// `d7 idea new` (and every other data-only command) for anyone who
// has not set the env var yet.
//
// One instance satisfies all four driving expander ports, so the
// composition root wires it once and passes it to cli.NewRootCmd as
// a single driving.Expander value.
type deferredExpander struct {
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

	editor driven.Editor
	clock  driven.Clock
}

func newDeferredExpander(
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
	editor driven.Editor,
	clock driven.Clock,
) *deferredExpander {
	return &deferredExpander{
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
		editor:    editor,
		clock:     clock,
	}
}

// Compile-time assertions — the shim satisfies every expander port
// plus the combined Expander bundle.
var (
	_ driving.IdeaExpander    = (*deferredExpander)(nil)
	_ driving.EpicExpander    = (*deferredExpander)(nil)
	_ driving.FeatureExpander = (*deferredExpander)(nil)
	_ driving.StoryExpander   = (*deferredExpander)(nil)
	_ driving.Expander        = (*deferredExpander)(nil)
)

// build constructs a fresh ExpandService, wiring the Anthropic client
// and the per-invocation I/O adapters (interaction, status). Called
// at the top of each Expand* method. The only significant cost is
// anthropicadapter.NewClient, which re-reads ANTHROPIC_API_KEY — that
// is fine for a CLI process.
func (d *deferredExpander) build() (*service.ExpandService, error) {
	llm, err := anthropicadapter.NewClient()
	if err != nil {
		return nil, err
	}
	inter := cliinteraction.New(d.editor)
	status := ttystatus.New()
	return service.NewExpandService(
		d.fs, d.wsRepo, d.ideaRepo, d.epicRepo, d.featRepo, d.storyRepo,
		d.linkRepo, d.refRepo, d.resolver, d.history,
		llm, inter, status, d.clock,
	), nil
}

func (d *deferredExpander) ExpandIdea(ctx context.Context, req driving.ExpandIdeaRequest) (*domain.Idea, error) {
	svc, err := d.build()
	if err != nil {
		return nil, err
	}
	return svc.ExpandIdea(ctx, req)
}

func (d *deferredExpander) ExpandEpic(ctx context.Context, req driving.ExpandEpicRequest) (*domain.Epic, error) {
	svc, err := d.build()
	if err != nil {
		return nil, err
	}
	return svc.ExpandEpic(ctx, req)
}

func (d *deferredExpander) ExpandFeature(ctx context.Context, req driving.ExpandFeatureRequest) (*domain.Feature, error) {
	svc, err := d.build()
	if err != nil {
		return nil, err
	}
	return svc.ExpandFeature(ctx, req)
}

func (d *deferredExpander) ExpandStory(ctx context.Context, req driving.ExpandStoryRequest) (*domain.Story, error) {
	svc, err := d.build()
	if err != nil {
		return nil, err
	}
	return svc.ExpandStory(ctx, req)
}
