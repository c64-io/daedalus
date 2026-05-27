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

	// AI-adjacent driven adapters. The Anthropic client defers its
	// ANTHROPIC_API_KEY check until the first Chat() call, so it's
	// safe to build eagerly here — data-only commands never trip the
	// missing-key path.
	llm := anthropicadapter.New()
	inter := cliinteraction.New(editor)
	status := ttystatus.New()

	wsSvc := service.NewWorkspaceService(fs, wsRepo)
	ideaSvc := service.NewIdeaService(fs, ideaRepo, historyRepo)
	epicSvc := service.NewEpicService(fs, epicRepo, ideaRepo, historyRepo)
	featureSvc := service.NewFeatureService(fs, featureRepo, epicRepo, historyRepo)
	storySvc := service.NewStoryService(fs, storyRepo, featureRepo, historyRepo)
	specSvc := service.NewSpecService(fs, specRepo, storyRepo, historyRepo)
	scenarioSvc := service.NewScenarioService(fs, scenarioRepo, specRepo, historyRepo)
	linkSvc := service.NewLinkService(fs, linkRepo, refRepo, entityResolver, historyRepo)
	expandSvc := service.NewExpandService(
		fs, wsRepo, ideaRepo, epicRepo, featureRepo, storyRepo,
		specRepo, scenarioRepo,
		linkRepo, refRepo, entityResolver, historyRepo,
		llm, inter, status, clock,
	)
	suggestSvc := service.NewSuggestService(
		fs, wsRepo, ideaRepo, epicRepo, featureRepo, storyRepo, specRepo,
		scenarioRepo,
		linkRepo, refRepo, entityResolver,
		epicSvc, featureSvc, storySvc, specSvc, scenarioSvc,
		llm, inter, status, clock,
	)
	exportSvc := service.NewExportService(
		fs, storyRepo, specRepo, scenarioRepo, featureRepo, epicRepo,
	)

	root := cli.NewRootCmd(
		wsSvc,       // driving.Workspace
		ideaSvc,     // driving.Idea
		epicSvc,     // driving.Epic
		featureSvc,  // driving.Feature
		storySvc,    // driving.Story
		specSvc,     // driving.Spec
		scenarioSvc, // driving.Scenario
		linkSvc,     // driving.Link
		linkSvc,     // driving.Ref (same concrete svc)
		expandSvc,   // driving.Expander
		suggestSvc,  // driving.Suggester
		exportSvc,   // driving.Export
		editor,
	)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := root.ExecuteContext(ctx); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
