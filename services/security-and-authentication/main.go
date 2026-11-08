package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	auth "security-and-authentication/internal"
	"security-and-authentication/internal/config"
)

func main() {
	if err := run(); err != nil {
		slog.New(slog.NewJSONHandler(os.Stderr, nil)).Error(
			"auth_service_failed", "error", err,
		)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load configuration: %w", err)
	}

	logger := auth.NewLogger(cfg.LogLevel)
	app, err := auth.NewApp(cfg, logger)
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
