package monitoringsuite

import (
	"context"
	"log/slog"
	"os"

	"github.com/sacloud/sakumock/core"
)

// Command is the CLI command for the monitoring-suite mock server. It embeds
// Config so the same struct works both as a standalone binary (flat flags) and
// as a subcommand of the unified sakumock binary.
type Command struct {
	Config
	Routes bool `help:"List supported HTTP routes and exit"`
}

// Run starts the monitoring-suite mock server and serves until ctx is canceled.
func (c *Command) Run(ctx context.Context) error {
	if c.Routes {
		h, err := NewHandler(Config{})
		if err != nil {
			return err
		}
		defer h.Close()
		return core.PrintRoutes(os.Stdout, h.Routes())
	}

	core.SetupLogger(c.Debug)

	h, err := NewHandler(c.Config)
	if err != nil {
		return err
	}
	defer h.Close()

	slog.Info("sakumock-monitoringsuite starting",
		"version", Version,
		"addr", c.Addr,
		"latency", c.Latency,
		"rate_limit", core.RateLimitHint(c.RateLimit, c.RateLimitWindow, ""),
		"debug", c.Debug,
	)
	slog.Info("to use with the monitoring-suite SDK",
		core.LogArgs(append(c.ClientEnv(), core.DummyCredentialEnv()...))...)
	return core.Serve(ctx, c.Addr, h)
}
