package app

import (
	"context"
	"embed"
	"io/fs"
	"net/http"
	"runtime"

	"github.com/adamada1725/de_loader/internal/components"
	"github.com/adamada1725/de_loader/internal/config"
	"github.com/adamada1725/de_loader/internal/jobs"
	"github.com/adamada1725/de_loader/internal/logging"
	"github.com/adamada1725/de_loader/internal/update"
)

type Options struct {
	Version, OS, Arch string
	WebFS             embed.FS
}
type App struct {
	cfg               *config.Store
	components        *components.Manager
	jobs              *jobs.Manager
	updates           *update.Checker
	log               *logging.Logger
	web               fs.FS
	version, os, arch string
}

func New(o Options) (*App, error) {
	cfg, err := config.Open()
	if err != nil {
		return nil, err
	}
	logger, err := logging.New(cfg.DataDir())
	if err != nil {
		return nil, err
	}
	logger.Event("startup version=%s os=%s arch=%s", o.Version, o.OS, o.Arch)
	cm := components.New(cfg.DataDir(), o.OS, o.Arch, logger)
	return &App{cfg: cfg, components: cm, jobs: jobs.New(cm, cfg, logger), updates: update.New(cfg, o.Version), log: logger, web: o.WebFS, version: o.Version, os: o.OS, arch: o.Arch}, nil
}
func (a *App) Startup(ctx context.Context) {
	if err := a.components.Ensure(ctx); err != nil {
		a.log.Error("component startup installation failed: %v", err)
	}
	a.updates.CheckIfDue(ctx)
}
func (a *App) Close()                { a.jobs.Close(); a.log.Event("application shutdown"); _ = a.log.Close() }
func (a *App) Handler() http.Handler { return routes(a) }
func platform() string               { return runtime.GOOS }
