package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/G-A-S-UNIVERSE/airing-go/internal/akvorado"
	"github.com/G-A-S-UNIVERSE/airing-go/internal/enricher"
	"github.com/G-A-S-UNIVERSE/airing-go/internal/eventbus"
	"github.com/G-A-S-UNIVERSE/airing-go/internal/librenms"
	_ "github.com/G-A-S-UNIVERSE/airing-go/internal/olt/generic"
	_ "github.com/G-A-S-UNIVERSE/airing-go/internal/olt/zte"
	"github.com/G-A-S-UNIVERSE/airing-go/internal/rules"
)

func parseLogLevel(raw string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func main() {
	level := parseLogLevel(os.Getenv("AIRING_LOG_LEVEL"))
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
	slog.SetDefault(logger)

	listenAddr := os.Getenv("AIRING_LISTEN_ADDR")
	if listenAddr == "" {
		listenAddr = ":8090"
	}

	bus := eventbus.New(eventbus.DefaultBufferSize)
	lnms := librenms.New(librenms.Config{
		BaseURL: os.Getenv("LIBRENMS_URL"),
		Token:   os.Getenv("LIBRENMS_API_TOKEN"),
	})
	akv := akvorado.New(akvorado.Config{
		BaseURL: os.Getenv("AKVORADO_URL"),
	})
	enr := enricher.New(lnms, akv, 5*time.Second, logger)
	eng := rules.New()
	notifier := &slogNotifier{logger: logger}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	workerDone := make(chan struct{})
	go func() {
		defer close(workerDone)
		bus.Run(ctx, func(wCtx context.Context, ev eventbus.Event) {
			processEvent(wCtx, ev, enr, eng, notifier, logger)
		})
	}()

	srv := &http.Server{
		Addr:              listenAddr,
		Handler:           newMux(serverConfig{APIToken: os.Getenv("AIRING_API_TOKEN"), WebhookSecret: os.Getenv("ZABBIX_WEBHOOK_SECRET"), Bus: bus, Logger: logger}),
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		logger.Info("airing-go starting", "version", appVersion, "addr", listenAddr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("http server error", "error", err)
			cancel()
		}
	}()

	<-ctx.Done()
	logger.Info("airing-go shutting down")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("http server shutdown error", "error", err)
	}
	bus.Close()
	<-workerDone
	logger.Info("airing-go stopped")
}
