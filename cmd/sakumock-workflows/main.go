package main

import (
	"context"

	"github.com/alecthomas/kong"
	"github.com/sacloud/sakumock"
	"github.com/sacloud/sakumock/core"
	"github.com/sacloud/sakumock/workflows"
)

func main() {
	ctx, stop := core.NotifyContext(context.Background())
	defer stop()

	var cmd workflows.Command
	kong.Parse(&cmd,
		kong.Name("sakumock-workflows"),
		kong.Description("Local mock server for SAKURA Cloud Workflows API."),
		kong.UsageOnError(),
		kong.Vars{"version": sakumock.Version},
	)
	if err := cmd.Run(ctx); err != nil {
		panic(err)
	}
}
