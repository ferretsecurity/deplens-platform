package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"

	"github.com/ferretsecurity/deplens-platform/internal/app"
	"github.com/ferretsecurity/deplens-platform/internal/config"
	"github.com/ferretsecurity/deplens-platform/internal/store"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	cfg, err := config.Load()
	if err != nil {
		logger.Error("load config", "error", err)
		os.Exit(1)
	}

	application, err := app.New(cfg)
	if err != nil {
		logger.Error("build app", "error", err)
		os.Exit(1)
	}
	defer application.DB.Close()

	if cfg.MigrateOnStart {
		if err := store.Migrate(cfg.DatabaseURL); err != nil {
			logger.Error("run migrations", "error", err)
			os.Exit(1)
		}
	}

	bootstrapInput := store.BootstrapInput{
		OwnerEmail:     cfg.BootstrapOwnerEmail,
		OwnerPassword:  cfg.BootstrapOwnerPassword,
		BootstrapToken: cfg.BootstrapAPIToken,
	}
	if _, err := store.BootstrapDefaultTenant(context.Background(), application.DB, bootstrapInput); err != nil {
		logger.Error("bootstrap default tenant", "error", err)
		os.Exit(1)
	}

	logger.Info("http server starting", "http_address", cfg.HTTPAddress, "app_base_url", cfg.AppBaseURL)
	if err := http.ListenAndServe(cfg.HTTPAddress, application.Handler); err != nil {
		logger.Error("http server stopped", "error", err)
		os.Exit(1)
	}
}
