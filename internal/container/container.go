// Package container provides the NASIJ dependency injection container.
//
// Container is constructed once in main and threaded into all CLI commands.
// It wires together: Config → Logger → PluginRegistry → WorkspaceManager → Terminal.
//
// Storage (per-workspace SQLite) is NOT part of the global container —
// it is opened on demand when a command needs to read or write scan data.
package container

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"go.uber.org/zap"

	"github.com/nasij/nasij/internal/config"
	"github.com/nasij/nasij/internal/logger"
	"github.com/nasij/nasij/internal/plugin"
	"github.com/nasij/nasij/internal/ui"
	"github.com/nasij/nasij/internal/workspace"
)

// Container holds all application-level dependencies.
// Fields are exported so CLI commands can access them directly.
type Container struct {
	Config    *config.Config
	Logger    *zap.Logger
	Plugins   *plugin.Registry
	Workspace *workspace.Manager
	UI        *ui.Terminal

	// pluginLoader is kept for doctor/diagnostics access
	pluginLoader *plugin.Loader
}

// New builds and wires the application container.
//
//   - ctx is used for any context-aware initialisation steps.
//   - cfgPath is the path to the config file; an empty string uses the
//     default location (~/.nasij/config.yaml).
//
// New is intentionally fail-fast: if any required component cannot be
// initialised, an error is returned immediately with a descriptive message.
func New(ctx context.Context, cfgPath string) (*Container, error) {
	// 1. Configuration
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return nil, fmt.Errorf("container init: config: %w", err)
	}

	// 2. Structured logger
	log, err := logger.New(cfg.LogLevel, cfg.LogFormat)
	if err != nil {
		return nil, fmt.Errorf("container init: logger: %w", err)
	}

	log.Debug("configuration loaded",
		zap.String("workspace_root", cfg.WorkspaceRoot),
		zap.String("log_level", cfg.LogLevel),
		zap.String("log_format", cfg.LogFormat),
		zap.String("plugin_dir", cfg.Plugin.Dir),
	)

	// 3. Plugin registry
	registry := plugin.NewRegistry()

	// 4. Plugin loader — scans plugin dir; actual .so loading in Phase 3
	loader := plugin.NewLoader(cfg.Plugin.Dir, registry, log)
	if cfg.Plugin.Enabled {
		if _, err := loader.Load(ctx); err != nil {
			// Non-fatal: log and continue
			log.Warn("plugin scan encountered errors; some plugins may be unavailable",
				zap.Error(err),
			)
		}
	} else {
		log.Debug("plugin system disabled via config")
	}

	// 5. Workspace manager (creates root dir if absent)
	wsManager, err := workspace.NewManager(cfg.WorkspaceRoot, log)
	if err != nil {
		return nil, fmt.Errorf("container init: workspace manager: %w", err)
	}

	// 6. Terminal UI
	term := ui.Default()

	log.Debug("container ready",
		zap.Int("plugins_registered", registry.Count()),
		zap.String("workspace_root", cfg.WorkspaceRoot),
	)

	return &Container{
		Config:       cfg,
		Logger:       log,
		Plugins:      registry,
		Workspace:    wsManager,
		UI:           term,
		pluginLoader: loader,
	}, nil
}

// Close releases resources held by the container.
// Always call this (e.g. via defer) in main.
func (c *Container) Close() {
	if c.Logger != nil {
		_ = c.Logger.Sync()
	}
}

// PluginLoader returns the plugin loader for diagnostics.
func (c *Container) PluginLoader() *plugin.Loader {
	return c.pluginLoader
}

// NasijDir returns the ~/.nasij directory for the current user.
func NasijDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("nasij dir: home: %w", err)
	}
	return filepath.Join(home, ".nasij"), nil
}
