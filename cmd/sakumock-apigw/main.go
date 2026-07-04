package main

import (
	"context"

	"github.com/alecthomas/kong"
	"github.com/sacloud/sakumock"
	"github.com/sacloud/sakumock/apigw"
	"github.com/sacloud/sakumock/core"
)

func main() {
	ctx, stop := core.NotifyContext(context.Background())
	defer stop()

	var cmd apigw.Command
	kong.Parse(&cmd,
		kong.Name("sakumock-apigw"),
		kong.Description("Local mock server for SAKURA Cloud API Gateway API."),
		kong.UsageOnError(),
		kong.Vars{"version": sakumock.Version},
	)
	if err := cmd.Run(ctx); err != nil {
		panic(err)
	}
}
