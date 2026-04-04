package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"

	"github.com/alecthomas/kong"
	"github.com/sacloud/sakumock/simplemq"
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
	var cfg simplemq.Config
	kong.Parse(&cfg)

	level := slog.LevelInfo
	if cfg.Debug {
		level = slog.LevelDebug
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})))

	handler, err := simplemq.NewHandler(cfg)
	if err != nil {
		return err
	}
	defer handler.Close()

	srv := &http.Server{
		Addr:    cfg.Addr,
		Handler: handler,
	}

	go func() {
		<-ctx.Done()
		srv.Shutdown(context.Background())
	}()

	slog.Info("sakumock-simplemq starting",
		"addr", cfg.Addr,
		"api_key", apiKeyHint(cfg.APIKey),
		"visibility_timeout", cfg.VisibilityTimeout,
		"message_expire", cfg.MessageExpire,
		"database", databaseHint(cfg.Database),
		"latency", cfg.Latency,
		"debug", cfg.Debug,
	)
	slog.Info("to use with simplemq-api-go SDK or simplemq-cli",
		"SAKURA_ENDPOINTS_SIMPLE_MQ_MESSAGE", "http://"+cfg.Addr,
		"SAKURA_ACCESS_TOKEN", "dummy",
		"SAKURA_ACCESS_TOKEN_SECRET", "dummy",
	)
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		return err
	}
	return nil
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
