package cli

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/spf13/cobra"

	"github.com/nasij/nasij/internal/container"
	"github.com/nasij/nasij/internal/ui"
	"github.com/nasij/nasij/pkg/version"

	_ "modernc.org/sqlite" // ensure driver is registered for doctor check
)

// newDoctorCmd returns the `nasij doctor` command.
func newDoctorCmd(c *container.Container) *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Run system health checks",
		Long: `Verifies that NASIJ's runtime dependencies and configuration are working correctly.
Exits with code 1 if any required check fails.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDoctor(cmd.Context(), c)
		},
	}
}

func runDoctor(ctx context.Context, c *container.Container) error {
	ui.PrintBanner()

	term := c.UI
	term.Header("System Health Check")
	term.Divider()
	term.Blank()

	allOK := true
	check := func(ok bool, label, detail string) {
		term.StatusRow(ok, label, detail)
		if !ok {
			allOK = false
		}
	}

	// 1. Go runtime
	check(true, "Go runtime", runtime.Version())

	// 2. Version
	check(true, "NASIJ version", version.Version+" ("+version.GitCommit+")")

	// 3. Config file
	cfgPath, _ := container.NasijDir()
	cfgFile := filepath.Join(cfgPath, "config.yaml")
	cfgExists := fileExists(cfgFile)
	if cfgExists {
		check(true, "Config file", cfgFile)
	} else {
		check(true, "Config file", "not found — using defaults (run nasij init)")
	}

	// 4. Log settings
	check(true, "Log level", c.Config.LogLevel)
	check(true, "Log format", c.Config.LogFormat)

	// 5. Workspace root
	wsRoot := c.Config.WorkspaceRoot
	wsList, err := c.Workspace.List()
	wsDetail := wsRoot
	if err == nil {
		wsDetail = fmt.Sprintf("%s  (%d workspaces)", wsRoot, len(wsList))
	}
	check(err == nil, "Workspace root", wsDetail)

	// 6. SQLite driver
	sqliteOK, sqliteDetail := checkSQLite(ctx)
	check(sqliteOK, "SQLite driver", sqliteDetail)

	// 7. Plugin directory
	pluginDir := c.Config.Plugin.Dir
	pluginOK, pluginDetail := checkPluginDir(pluginDir)
	check(pluginOK, "Plugin directory", pluginDetail)

	// 8. Plugin registry
	pluginCount := c.Plugins.Count()
	check(true, "Plugins registered", fmt.Sprintf("%d", pluginCount))

	term.Blank()
	term.Divider()
	term.Blank()

	if allOK {
		term.Success("All checks passed — NASIJ is ready.")
	} else {
		term.Error("One or more checks failed. Resolve the issues above before scanning.")
		term.Blank()
		return fmt.Errorf("doctor: health checks failed")
	}

	term.Blank()
	return nil
}

// checkSQLite opens an in-memory SQLite database to verify the driver works.
func checkSQLite(ctx context.Context) (bool, string) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		return false, "driver open failed: " + err.Error()
	}
	defer db.Close()

	if err := db.PingContext(ctx); err != nil {
		return false, "ping failed: " + err.Error()
	}

	var v string
	if err := db.QueryRowContext(ctx, "SELECT sqlite_version()").Scan(&v); err != nil {
		return false, "version query failed: " + err.Error()
	}

	return true, "modernc/sqlite  SQLite " + v + "  (WAL)"
}

// checkPluginDir reports plugin directory status.
func checkPluginDir(dir string) (bool, string) {
	info, err := os.Stat(dir)
	if os.IsNotExist(err) {
		return true, dir + "  (does not exist — plugins not yet loaded)"
	}
	if err != nil {
		return false, dir + "  stat error: " + err.Error()
	}
	if !info.IsDir() {
		return false, dir + "  is not a directory"
	}
	// Count .so files
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false, dir + "  read error: " + err.Error()
	}
	count := 0
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".so" {
			count++
		}
	}
	return true, fmt.Sprintf("%s  (%d plugins)", dir, count)
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
