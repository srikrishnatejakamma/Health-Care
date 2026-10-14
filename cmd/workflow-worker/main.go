package main

import (
	"context"
	"errors"
	"healthcare/internal/application"
	"healthcare/internal/bootstrap"
	"healthcare/internal/framework/configuration"
	"healthcare/internal/framework/workflow"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"
	_ "time/tzdata"
)

func run() error {
	if err := configuration.LoadDotEnv(".env"); err != nil {
		return err
	}
	c, err := configuration.Load()
	if err != nil {
		return err
	}
	if c.Store != "postgres" {
		return errors.New("a standalone worker requires STORE=postgres; memory mode must use the embedded API worker")
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	startup, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	store, err := bootstrap.Store(startup, c)
	if err != nil {
		return err
	}
	defer store.Close()
	runner := workflow.New(application.New(store), c.Workers, c.MemoryMB)
	runner.Run(ctx)
	return nil
}
func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	if err := run(); err != nil {
		slog.Error("worker failed", "error", err)
		os.Exit(1)
	}
}
