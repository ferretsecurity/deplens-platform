package app

import "github.com/ferretsecurity/deplens-platform/internal/config"

type App struct {
	Config config.Config
}

func New(cfg config.Config) *App {
	return &App{Config: cfg}
}
