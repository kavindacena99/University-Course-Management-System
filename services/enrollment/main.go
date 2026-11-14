package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	enrollment "enrollment/internal"
	"enrollment/internal/config"
)

func main() {
	if err := run(); err != nil {
		slog.New(slog.NewJSONHandler(os.Stderr, nil)).Error(
			"enrollment_service_failed", "error", err,
		)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load configuration: %w", err)
	}

	logger := enrollment.NewLogger(cfg.LogLevel)
	app, err := enrollment.NewApp(cfg, logger)
	if err != nil {
		return fmt.Errorf("initialise application: %w", err)
	}
	defer app.Close()

	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()

	return app.Run(ctx)
}
