package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/nasij/nasij/internal/container"
	"github.com/nasij/nasij/internal/ui"
)

func newPluginsCmd(c *container.Container) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "plugins",
		Short: "List and inspect loaded plugins",
		Long: `Display registered plugins, their versions, kinds, and metadata.
NASIJ plugins extend functionality for analysis, reporting, and export.

The plugin system uses Go's plugin package (.so files) loaded from
the configured plugin directory.`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	cmd.SetHelpFunc(helpFunc(c))

	cmd.AddCommand(
		newPluginsListCmd(c),
		newPluginsInfoCmd(c),
	)

	return cmd
}

func newPluginsListCmd(c *container.Container) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all registered plugins",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPluginsList(c)
		},
	}
}

func newPluginsInfoCmd(c *container.Container) *cobra.Command {
	return &cobra.Command{
		Use:   "info <name>",
		Short: "Show details for a specific plugin",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPluginsInfo(c, args[0])
		},
	}
}

func runPluginsList(c *container.Container) error {
	ui.PrintBanner()
	term := c.UI

	term.Header(fmt.Sprintf("Plugins (%d)", c.Plugins.Count()))
	term.Divider()
	term.Blank()

	all := c.Plugins.List()
	if len(all) == 0 {
		term.Info("No plugins registered. Install plugins in " + c.Config.Plugin.Dir)
		term.Blank()
		term.Info(ui.StyleMuted.Render("Plugin loading from .so files will be implemented in Phase 3."))
		term.Blank()
		return nil
	}

	for _, p := range all {
		term.Subheader("  " + p.Name())
		term.Table([][2]string{
			{"Version", p.Version()},
			{"Kind", string(p.Kind())},
		})
		term.Blank()
	}

	return nil
}

func runPluginsInfo(c *container.Container, name string) error {
	term := c.UI

	p, ok := c.Plugins.Get(name)
	if !ok {
		term.Error(fmt.Sprintf("Plugin %q not found", name))
		return fmt.Errorf("plugin %q not registered", name)
	}

	ui.PrintBanner()
	term.Header(fmt.Sprintf("Plugin: %s", p.Name()))
	term.Divider()
	term.Blank()
	term.Table([][2]string{
		{"Name", ui.StyleAccent.Render(p.Name())},
		{"Version", p.Version()},
		{"Kind", ui.StylePrimary.Render(string(p.Kind()))},
	})
	term.Blank()
	term.Divider()
	term.Blank()
	term.Info(ui.StyleMuted.Render("Plugin details will be expanded in Phase 3."))
	term.Blank()

	return nil
}


