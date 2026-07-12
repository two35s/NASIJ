package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/nasij/nasij/internal/container"
	"github.com/nasij/nasij/internal/ui"
)

// newInitCmd returns the `nasij init` command.
func newInitCmd(c *container.Container) *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Initialise NASIJ configuration directory",
		Long: `Creates the ~/.nasij directory tree and writes a default config.yaml.

If config.yaml already exists it is NOT overwritten — your existing
settings are preserved. Run this command once after installation.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInit(c)
		},
	}
}

func runInit(c *container.Container) error {
	ui.PrintBanner()

	term := c.UI
	term.Header("Initialising NASIJ")
	term.Divider()
	term.Blank()

	nasijDir, err := container.NasijDir()
	if err != nil {
		return fmt.Errorf("init: determine nasij dir: %w", err)
	}

	// 1. Create ~/.nasij
	if err := ensureDir(nasijDir); err != nil {
		term.StatusRow(false, "~/.nasij", err.Error())
		return err
	}
	term.StatusRow(true, "~/.nasij", nasijDir)

	// 2. Create ~/.nasij/workspaces
	wsDir := filepath.Join(nasijDir, "workspaces")
	if err := ensureDir(wsDir); err != nil {
		term.StatusRow(false, "~/.nasij/workspaces", err.Error())
		return err
	}
	term.StatusRow(true, "~/.nasij/workspaces", wsDir)

	// 3. Create ~/.nasij/plugins
	pluginDir := filepath.Join(nasijDir, "plugins")
	if err := ensureDir(pluginDir); err != nil {
		term.StatusRow(false, "~/.nasij/plugins", err.Error())
		return err
	}
	term.StatusRow(true, "~/.nasij/plugins", pluginDir)

	// 4. Write config.yaml (only if absent)
	cfgPath := filepath.Join(nasijDir, "config.yaml")
	cfgWritten, err := writeDefaultConfig(cfgPath)
	if err != nil {
		term.StatusRow(false, "config.yaml", err.Error())
		return err
	}
	if cfgWritten {
		term.StatusRow(true, "config.yaml", cfgPath+" (created)")
	} else {
		term.StatusRow(true, "config.yaml", cfgPath+" (already exists — not overwritten)")
	}

	term.Blank()
	term.Divider()
	term.Blank()
	term.Success("NASIJ is ready.")
	term.Blank()
	term.Info("Next steps:")
	term.Info("  1. Run `nasij doctor` to verify your setup.")
	term.Info("  2. Run `nasij workspace create --name <label> --target <url>` to start a recon workspace.")
	term.Blank()

	return nil
}

// ensureDir creates dir with mode 0750 if it does not exist.
func ensureDir(dir string) error {
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("mkdir %q: %w", dir, err)
	}
	return nil
}

// writeDefaultConfig writes a default config.yaml to path if it does not exist.
// Returns (true, nil) if the file was written, (false, nil) if it already existed.
func writeDefaultConfig(path string) (bool, error) {
	if _, err := os.Stat(path); err == nil {
		// Already exists — do not overwrite
		return false, nil
	}

	const defaultConfig = `# NASIJ Configuration
# See configs/default.yaml for all available options and documentation.

log_level: info
log_format: pretty
workspace_root: ~/.nasij/workspaces

db:
  max_open_conns: 1
  max_idle_conns: 1

plugin:
  dir: ~/.nasij/plugins
  enabled: true
`
	if err := os.WriteFile(path, []byte(defaultConfig), 0o640); err != nil {
		return false, fmt.Errorf("write %q: %w", path, err)
	}
	return true, nil
}
