package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"

	"github.com/alecthomas/kong"
	"github.com/sacloud/sakumock/secretmanager"
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
		secretmanager.Config
		Routes  bool             `help:"List supported HTTP routes and exit"`
		Version kong.VersionFlag `help:"Show version" short:"v"`
	}
	kong.Parse(&cli, kong.Vars{"version": secretmanager.Version})
	cfg := cli.Config

	if cli.Routes {
		handler := secretmanager.NewHandler(secretmanager.Config{})
		defer handler.Close()
		return handler.PrintRoutes(os.Stdout)
	}

	level := slog.LevelInfo
	if cfg.Debug {
		level = slog.LevelDebug
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})))

	handler := secretmanager.NewHandler(cfg)
	defer handler.Close()

	srv := &http.Server{
		Addr:    cfg.Addr,
		Handler: handler,
	}

	go func() {
		<-ctx.Done()
		srv.Shutdown(context.Background())
	}()

	slog.Info("sakumock-secretmanager starting",
		"version", secretmanager.Version,
		"addr", cfg.Addr,
		"latency", cfg.Latency,
		"debug", cfg.Debug,
	)
	slog.Info("to use with sakura-secrets-cli",
		"SAKURA_API_ROOT_URL", "http://"+cfg.Addr,
		"SAKURA_ACCESS_TOKEN", "dummy",
		"SAKURA_ACCESS_TOKEN_SECRET", "dummy",
	)
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		return err
	}
	return nil
}
