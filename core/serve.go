package core

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"time"
)

// SetupLogger installs the default logger, at Debug level when debug is set.
func SetupLogger(debug bool) {
	level := slog.LevelInfo
	if debug {
		level = slog.LevelDebug
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})))
}

// NotifyContext returns a context that is canceled when the process receives a
// shutdown signal (SIGINT, and SIGTERM on non-Windows platforms). The returned
// stop function releases the signal handler and should be deferred.
func NotifyContext(parent context.Context) (context.Context, context.CancelFunc) {
	return signal.NotifyContext(parent, shutdownSignals()...)
}

// Serve runs h on addr until ctx is canceled, then shuts the server down
// gracefully. It returns nil on a clean shutdown.
func Serve(ctx context.Context, addr string, h http.Handler, tls TLSFiles) error {
	srv := &http.Server{
		Addr:    addr,
		Handler: h,
	}
	go func() {
		<-ctx.Done()
		srv.Shutdown(context.Background())
	}()
	var err error
	if tls.Enabled() {
		err = srv.ListenAndServeTLS(tls.CertFile, tls.KeyFile)
	} else {
		err = srv.ListenAndServe()
	}
	if err != http.ErrServerClosed {
		return err
	}
	return nil
}

// RateLimitHint describes a rate-limit setting for startup logs. suffix is
// appended after the window (e.g. " per queue").
func RateLimitHint(events float64, window time.Duration, suffix string) string {
	if events <= 0 {
		return "(disabled)"
	}
	return fmt.Sprintf("%g per %s%s", events, window, suffix)
}
