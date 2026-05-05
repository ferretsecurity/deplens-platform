package main

import (
	"log/slog"
	"os"

	"github.com/ferretsecurity/deplens-platform/internal/app"
	"github.com/ferretsecurity/deplens-platform/internal/config"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	cfg, err := config.Load()
	if err != nil {
		logger.Error("load config", "error", err)
		os.Exit(1)
	}

	_ = app.New(cfg)
	logger.Info("deplens-platform configured", "mode", cfg.Mode, "http_address", cfg.HTTPAddress)
}
