package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	cloveradapter "github.com/c64-io/daedalus/internal/adapter/driven/clover"
	"github.com/c64-io/daedalus/internal/adapter/driving/cli"
	"github.com/c64-io/daedalus/internal/core/service"
)

func main() {
	// Composition root: wire driven adapters → core service → driving adapter.
	repo := cloveradapter.NewWorkspaceRepository()
	svc := service.NewWorkspaceService(repo)
	root := cli.NewRootCmd(svc)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := root.ExecuteContext(ctx); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
