package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	_ "github.com/G-A-S-UNIVERSE/airing-go/internal/olt/generic"
	_ "github.com/G-A-S-UNIVERSE/airing-go/internal/olt/zte"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	slog.Info("airing-go starting", "version", "0.1.0-dev")
	<-ctx.Done()
	slog.Info("airing-go shutting down")
}
