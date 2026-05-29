package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/alecthomas/kong"
	"github.com/sacloud/sakumock/core"
	"github.com/sacloud/sakumock/simplemq"
)

func main() {
	ctx, stop := core.NotifyContext(context.Background())
	defer stop()

	var cli struct {
		simplemq.Command
		Version kong.VersionFlag `help:"Show version" short:"v"`
	}
	kong.Parse(&cli, kong.Vars{"version": simplemq.Version})

	if err := cli.Command.Run(ctx); err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}
}
