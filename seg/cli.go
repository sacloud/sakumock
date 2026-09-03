package seg

import (
	"context"
	"io"
	"log/slog"
	"os"

	"github.com/sacloud/sakumock"
	"github.com/sacloud/sakumock/core"
)

// Command is the CLI command for the Service Endpoint Gateway mock server. It
// embeds Config so the same struct works both as a standalone binary (flat
// flags) and as a subcommand of the unified sakumock binary.
type Command struct {
	Config
	TLS    core.TLSFiles `embed:"" prefix:"tls-" envprefix:"SEG_TLS_"`
	Routes bool          `help:"List supported HTTP routes and exit"`
	Docs   bool          `help:"Print this service's documentation (README) and exit"`
}

// Run starts the mock server and serves until ctx is canceled, or prints the
// route table (--routes) or the documentation (--docs) and returns.
func (c *Command) Run(ctx context.Context) error {
	if c.Docs {
		_, err := io.WriteString(os.Stdout, Doc)
		return err
	}

	if c.Routes {
		h, err := NewHandler(Config{})
		if err != nil {
			return err
		}
		defer h.Close()
		return core.PrintRoutes(os.Stdout, h.Routes())
	}

	if err := c.TLS.Validate(); err != nil {
		return err
	}

	core.SetupLogger(c.Debug)

	shutdownTracing, err := core.SetupTracing(ctx)
	if err != nil {
		return err
	}
	defer shutdownTracing()

	h, err := NewHandler(c.Config)
	if err != nil {
		return err
	}
	defer h.Close()

	slog.Info("sakumock-seg starting",
		"version", sakumock.Version,
		"addr", c.Addr,
		"latency", c.Latency,
		"rate_limit", core.RateLimitHint(c.RateLimit, c.RateLimitWindow, ""),
		"fault", core.FaultHint(c.Fault),
		"debug", c.Debug,
	)
	slog.Info("to use with sacloud-sdk-go",
		core.LogArgs(core.WithTLSScheme(append(c.ClientEnv(), core.DummyCredentialEnv()...), c.TLS.Enabled()))...)
	return core.Serve(ctx, c.Addr, core.TraceHandler(c.Name(), h), c.TLS)
}
