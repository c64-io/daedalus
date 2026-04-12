package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	anthropicadapter "github.com/c64-io/daedalus/internal/adapter/driven/anthropic"
	"github.com/c64-io/daedalus/internal/adapter/driven/cliinteraction"
	"github.com/c64-io/daedalus/internal/adapter/driven/clockexec"
	cloveradapter "github.com/c64-io/daedalus/internal/adapter/driven/clover"
	"github.com/c64-io/daedalus/internal/adapter/driven/editorexec"
	"github.com/c64-io/daedalus/internal/adapter/driven/osfs"
	"github.com/c64-io/daedalus/internal/adapter/driven/ttystatus"
	"github.com/c64-io/daedalus/internal/adapter/driving/cli"
	"github.com/c64-io/daedalus/internal/core/domain"
	"github.com/c64-io/daedalus/internal/core/port/driven"
	"github.com/c64-io/daedalus/internal/core/port/driving"
	"github.com/c64-io/daedalus/internal/core/service"
)

func main() {
	// Composition root: wire driven adapters → core services → driving adapter.
	fs := osfs.New()
	wsRepo := cloveradapter.NewWorkspaceRepository()
	ideaRepo := cloveradapter.NewIdeaRepository()

	epicRepo := cloveradapter.NewEpicRepository()
	featureRepo := cloveradapter.NewFeatureRepository()
	storyRepo := cloveradapter.NewStoryRepository()
	specRepo := cloveradapter.NewSpecRepository()
	scenarioRepo := cloveradapter.NewScenarioRepository()
	linkRepo := cloveradapter.NewLinkRepository()
	refRepo := cloveradapter.NewRefRepository()
	entityResolver := cloveradapter.NewEntityResolver()
	historyRepo := cloveradapter.NewHistoryRepository()

	editor := editorexec.New()
	clock := clockexec.New()

	wsSvc := service.NewWorkspaceService(fs, wsRepo)
	ideaSvc := service.NewIdeaService(fs, ideaRepo, historyRepo)
	epicSvc := service.NewEpicService(fs, epicRepo, ideaRepo, historyRepo)
	featureSvc := service.NewFeatureService(fs, featureRepo, epicRepo, historyRepo)
	storySvc := service.NewStoryService(fs, storyRepo, featureRepo, historyRepo)
	specSvc := service.NewSpecService(fs, specRepo, storyRepo, historyRepo)
	scenarioSvc := service.NewScenarioService(fs, scenarioRepo, specRepo, historyRepo)
	linkSvc := service.NewLinkService(fs, linkRepo, refRepo, entityResolver, historyRepo)

	// AI-assist services are wired lazily: we only need the Anthropic
	// client if the user runs an AI command, and the SDK's constructor
	// hits ANTHROPIC_API_KEY at construction time. Deferring the build
	// until the command runs keeps `d7 idea new` working on a machine
	// that hasn't set the env var yet.
	storyExpander := newDeferredStoryExpander(fs, wsRepo, ideaRepo, epicRepo, featureRepo, storyRepo, linkRepo, refRepo, entityResolver, historyRepo, editor, clock)

	root := cli.NewRootCmd(wsSvc, wsSvc, wsSvc, wsSvc, ideaSvc, ideaSvc, ideaSvc, epicSvc, epicSvc, epicSvc, featureSvc, featureSvc, featureSvc, storySvc, storySvc, storySvc, specSvc, specSvc, specSvc, scenarioSvc, scenarioSvc, scenarioSvc, linkSvc, linkSvc, linkSvc, linkSvc, linkSvc, linkSvc, storyExpander, editor)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := root.ExecuteContext(ctx); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

// deferredStoryExpander builds the AI-backed StoryExpandService on the
// first call to ExpandStory, so a missing ANTHROPIC_API_KEY only bites
// users who actually try to run an AI command.
type deferredStoryExpander struct {
	fs             driven.FileSystem
	wsRepo         driven.WorkspaceRepository
	ideaRepo       driven.IdeaRepository
	epicRepo       driven.EpicRepository
	featRepo       driven.FeatureRepository
	storyRepo      driven.StoryRepository
	linkRepo       driven.LinkRepository
	refRepo        driven.RefRepository
	entityResolver driven.EntityResolver
	history        driven.HistoryRepository
	editor         driven.Editor
	clock          driven.Clock
}

func newDeferredStoryExpander(
	fs driven.FileSystem,
	wsRepo driven.WorkspaceRepository,
	ideaRepo driven.IdeaRepository,
	epicRepo driven.EpicRepository,
	featRepo driven.FeatureRepository,
	storyRepo driven.StoryRepository,
	linkRepo driven.LinkRepository,
	refRepo driven.RefRepository,
	entityResolver driven.EntityResolver,
	history driven.HistoryRepository,
	editor driven.Editor,
	clock driven.Clock,
) *deferredStoryExpander {
	return &deferredStoryExpander{
		fs:             fs,
		wsRepo:         wsRepo,
		ideaRepo:       ideaRepo,
		epicRepo:       epicRepo,
		featRepo:       featRepo,
		storyRepo:      storyRepo,
		linkRepo:       linkRepo,
		refRepo:        refRepo,
		entityResolver: entityResolver,
		history:        history,
		editor:         editor,
		clock:          clock,
	}
}

// Compile-time assertion that deferredStoryExpander satisfies the
// driving port.
var _ driving.StoryExpander = (*deferredStoryExpander)(nil)

func (d *deferredStoryExpander) ExpandStory(ctx context.Context, req driving.ExpandStoryRequest) (*domain.Story, error) {
	llm, err := anthropicadapter.NewClient()
	if err != nil {
		return nil, err
	}
	inter := cliinteraction.New(d.editor)
	status := ttystatus.New()

	svc := service.NewStoryExpandService(
		d.fs, d.wsRepo, d.ideaRepo, d.epicRepo, d.featRepo, d.storyRepo,
		d.linkRepo, d.refRepo, d.entityResolver, d.history,
		llm, inter, status, d.clock,
	)
	return svc.ExpandStory(ctx, req)
}
