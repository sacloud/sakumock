package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/alecthomas/kong"
	"github.com/sacloud/sakumock/core"
	"github.com/sacloud/sakumock/kms"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), signals()...)
	defer stop()
	if err := run(ctx); err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	var cli struct {
		kms.Config
		Routes  bool             `help:"List supported HTTP routes and exit"`
		Version kong.VersionFlag `help:"Show version" short:"v"`
	}
	kong.Parse(&cli, kong.Vars{"version": kms.Version})
	cfg := cli.Config

	if cli.Routes {
		handler := kms.NewHandler(kms.Config{})
		defer handler.Close()
		return core.PrintRoutes(os.Stdout, handler.Routes())
	}

	level := slog.LevelInfo
	if cfg.Debug {
		level = slog.LevelDebug
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})))

	handler := kms.NewHandler(cfg)
	defer handler.Close()

	srv := &http.Server{
		Addr:    cfg.Addr,
		Handler: handler,
	}

	go func() {
		<-ctx.Done()
		srv.Shutdown(context.Background())
	}()

	slog.Info("sakumock-kms starting",
		"version", kms.Version,
		"addr", cfg.Addr,
		"latency", cfg.Latency,
		"rate_limit", rateLimitHint(cfg.RateLimit, cfg.RateLimitWindow),
		"debug", cfg.Debug,
	)
	slog.Info("to use with kms-api-go SDK",
		"SAKURA_ENDPOINTS_KMS", "http://"+cfg.Addr,
		"SAKURA_ACCESS_TOKEN", "dummy",
		"SAKURA_ACCESS_TOKEN_SECRET", "dummy",
	)
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		return err
	}
	return nil
}

func rateLimitHint(events float64, window time.Duration) string {
	if events <= 0 {
		return "(disabled)"
	}
	return fmt.Sprintf("%g per %s", events, window)
}
