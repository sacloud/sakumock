package simplemq

import (
	"context"
	"log/slog"
	"os"

	"github.com/sacloud/sakumock"
	"github.com/sacloud/sakumock/core"
)

// Command is the CLI command for the SimpleMQ mock server. It embeds Config so
// the same struct works both as a standalone binary (flat flags) and as a
// subcommand of the unified sakumock binary.
type Command struct {
	Config
	TLS    core.TLSFiles `embed:"" prefix:"tls-" envprefix:"SIMPLEMQ_TLS_"`
	Routes bool          `help:"List supported HTTP routes and exit"`
}

// Run starts the SimpleMQ mock server and serves until ctx is canceled.
func (c *Command) Run(ctx context.Context) error {
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

	h, err := NewHandler(c.Config)
	if err != nil {
		return err
	}
	defer h.Close()

	slog.Info("sakumock-simplemq starting",
		"version", sakumock.Version,
		"addr", c.Addr,
		"api_key", apiKeyHint(c.APIKey),
		"visibility_timeout", c.VisibilityTimeout,
		"message_expire", c.MessageExpire,
		"database", databaseHint(c.Database),
		"latency", c.Latency,
		"rate_limit", core.RateLimitHint(c.RateLimit, c.RateLimitWindow, " per queue"),
		"debug", c.Debug,
	)
	slog.Info("to use with sacloud-sdk-go or simplemq-cli",
		core.LogArgs(core.WithTLSScheme(append(c.ClientEnv(), core.DummyCredentialEnv()...), c.TLS.Enabled()))...)
	return core.Serve(ctx, c.Addr, h, c.TLS)
}

func apiKeyHint(key string) string {
	if key == "" {
		return "(any non-empty value accepted)"
	}
	return "(configured, use the key you specified)"
}

func databaseHint(path string) string {
	if path == "" {
		return "(in-memory)"
	}
	return path
}
