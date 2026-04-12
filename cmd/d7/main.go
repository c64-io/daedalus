package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	cloveradapter "github.com/c64-io/daedalus/internal/adapter/driven/clover"
	"github.com/c64-io/daedalus/internal/adapter/driven/editorexec"
	"github.com/c64-io/daedalus/internal/adapter/driven/osfs"
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

	wsSvc := service.NewWorkspaceService(fs, wsRepo)
	ideaSvc := service.NewIdeaService(fs, ideaRepo, historyRepo)
	epicSvc := service.NewEpicService(fs, epicRepo, ideaRepo, historyRepo)
	featureSvc := service.NewFeatureService(fs, featureRepo, epicRepo, historyRepo)
	storySvc := service.NewStoryService(fs, storyRepo, featureRepo, historyRepo)
	specSvc := service.NewSpecService(fs, specRepo, storyRepo, historyRepo)
	scenarioSvc := service.NewScenarioService(fs, scenarioRepo, specRepo, historyRepo)
	linkSvc := service.NewLinkService(fs, linkRepo, refRepo, entityResolver, historyRepo)

	root := cli.NewRootCmd(wsSvc, wsSvc, wsSvc, wsSvc, ideaSvc, ideaSvc, ideaSvc, epicSvc, epicSvc, epicSvc, featureSvc, featureSvc, featureSvc, storySvc, storySvc, storySvc, specSvc, specSvc, specSvc, scenarioSvc, scenarioSvc, scenarioSvc, linkSvc, linkSvc, linkSvc, linkSvc, linkSvc, linkSvc, editor)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := root.ExecuteContext(ctx); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
