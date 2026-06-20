package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/alecthomas/kong"
	"github.com/sacloud/sakumock"
	"github.com/sacloud/sakumock/apprundedicated"
	"github.com/sacloud/sakumock/core"
)

func main() {
	ctx, stop := core.NotifyContext(context.Background())
	defer stop()

	var cli struct {
		apprundedicated.Command
		Version kong.VersionFlag `help:"Show version" short:"v"`
	}
	kong.Parse(&cli, kong.Vars{"version": sakumock.Version})

	if err := cli.Command.Run(ctx); err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}
}
