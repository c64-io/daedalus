package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	cloveradapter "github.com/c64-io/daedalus/internal/adapter/driven/clover"
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

	wsSvc := service.NewWorkspaceService(fs, wsRepo)
	ideaSvc := service.NewIdeaService(fs, ideaRepo)
	epicSvc := service.NewEpicService(fs, epicRepo, ideaRepo)

	root := cli.NewRootCmd(wsSvc, wsSvc, ideaSvc, ideaSvc, wsSvc, epicSvc, epicSvc)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := root.ExecuteContext(ctx); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
