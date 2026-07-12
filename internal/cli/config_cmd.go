package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/nasij/nasij/internal/container"
	"github.com/nasij/nasij/internal/ui"
)

func newConfigCmd(c *container.Container) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "View and modify NASIJ configuration",
		Long: `Display, read, or update NASIJ configuration values.
Configuration is loaded from ~/.nasij/config.yaml and can be
overridden at runtime with NASIJ_* environment variables.

Available keys:
  log_level       Log severity threshold (debug|info|warn|error)
  log_format      Output format (pretty|json)
  workspace_root  Directory for workspace data
  db.max_open_conns  SQLite max open connections
  db.max_idle_conns  SQLite max idle connections
  plugin.dir      Plugin directory path
  plugin.enabled  Enable plugin system`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	cmd.SetHelpFunc(helpFunc(c))

	cmd.AddCommand(
		newConfigViewCmd(c),
		newConfigGetCmd(c),
	)

	return cmd
}

func newConfigViewCmd(c *container.Container) *cobra.Command {
	return &cobra.Command{
		Use:   "view",
		Short: "Display the full current configuration",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runConfigView(c)
		},
	}
}

func newConfigGetCmd(c *container.Container) *cobra.Command {
	return &cobra.Command{
		Use:   "get <key>",
		Short: "Get a specific configuration value",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runConfigGet(c, args[0])
		},
	}
}

func runConfigView(c *container.Container) error {
	ui.PrintBanner()
	term := c.UI

	term.Header("Configuration")
	term.Divider()
	term.Blank()

	cfg := c.Config
	term.Table([][2]string{
		{"log_level", ui.StyleAccent.Render(cfg.LogLevel)},
		{"log_format", ui.StyleMuted.Render(cfg.LogFormat)},
		{"workspace_root", ui.StyleWhite.Render(cfg.WorkspaceRoot)},
	})
	term.Blank()
	term.Subheader("  Database")
	term.Table([][2]string{
		{"max_open_conns", ui.StyleAccent.Render(fmt.Sprintf("%d", cfg.DB.MaxOpenConns))},
		{"max_idle_conns", ui.StyleAccent.Render(fmt.Sprintf("%d", cfg.DB.MaxIdleConns))},
	})
	term.Blank()
	term.Subheader("  Plugins")
	term.Table([][2]string{
		{"dir", ui.StyleWhite.Render(cfg.Plugin.Dir)},
		{"enabled", ui.StyleMuted.Render(fmt.Sprintf("%v", cfg.Plugin.Enabled))},
	})
	term.Blank()
	term.Divider()
	term.Blank()

	cfgPath, _ := container.NasijDir()
	cfgFile := filepath.Join(cfgPath, "config.yaml")
	if _, err := os.Stat(cfgFile); err == nil {
		term.Info(ui.StyleMuted.Render("Config file: " + cfgFile))
	} else {
		term.Info(ui.StyleMuted.Render("No config file found — using defaults (run nasij init)"))
	}
	term.Blank()

	return nil
}

func runConfigGet(c *container.Container, key string) error {
	cfg := c.Config
	var val string
	switch key {
	case "log_level":
		val = cfg.LogLevel
	case "log_format":
		val = cfg.LogFormat
	case "workspace_root":
		val = cfg.WorkspaceRoot
	case "db.max_open_conns":
		val = fmt.Sprintf("%d", cfg.DB.MaxOpenConns)
	case "db.max_idle_conns":
		val = fmt.Sprintf("%d", cfg.DB.MaxIdleConns)
	case "plugin.dir":
		val = cfg.Plugin.Dir
	case "plugin.enabled":
		val = fmt.Sprintf("%v", cfg.Plugin.Enabled)
	default:
		return fmt.Errorf("unknown config key: %s", key)
	}
	c.UI.Table([][2]string{{key, ui.StyleAccent.Render(val)}})
	return nil
}
