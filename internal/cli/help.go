package cli

import (
	"fmt"
	"io"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"github.com/nasij/nasij/internal/container"
	"github.com/nasij/nasij/internal/ui"
	"github.com/nasij/nasij/pkg/version"
)

func helpFunc(c *container.Container) func(*cobra.Command, []string) {
	return func(cmd *cobra.Command, args []string) {
		w := c.UI.Writer()
		isRoot := !cmd.HasParent()

		if isRoot {
			renderRootHelp(w, cmd)
		} else {
			renderSubHelp(w, cmd)
		}
	}
}

func renderRootHelp(w io.Writer, cmd *cobra.Command) {
	ui.PrintBanner()

	fmt.Fprintln(w, "  "+ui.StyleHeader.Render("USAGE"))
	fmt.Fprintln(w, "    "+ui.StyleWhite.Render("nasij <command> [flags]"))
	fmt.Fprintln(w)

	fmt.Fprintln(w, "  "+ui.StyleHeader.Render("COMMANDS"))
	cmds := []struct {
		name, desc string
	}{
		{"scan", "Run a reconnaissance scan against a target"},
		{"crawl", "Crawl a target application to discover assets"},
		{"workspace", "Manage reconnaissance workspaces"},
		{"config", "View and modify NASIJ configuration"},
		{"plugins", "List and inspect loaded plugins"},
		{"report", "Generate and view scan reports"},
		{"doctor", "Verify system health and dependencies"},
		{"init", "Initialise NASIJ directory and configuration"},
		{"version", "Print build and runtime version info"},
	}
	for _, c := range cmds {
		name := ui.StyleAccent.Render(padRight(c.name, 14))
		desc := ui.StyleMuted.Render(c.desc)
		fmt.Fprintln(w, "    "+name+" "+desc)
	}
	fmt.Fprintln(w)

	fmt.Fprintln(w, "  "+ui.StyleHeader.Render("FLAGS"))
	ui.PrintFlagUsages(w, cmd.PersistentFlags(), "    ")
	fmt.Fprintln(w)

	fmt.Fprintln(w, "  "+ui.StyleHeader.Render("EXAMPLES"))
	for _, ex := range []string{
		"  "+ui.StyleAccent.Render("$ nasij init"),
		"  "+ui.StyleAccent.Render("$ nasij doctor"),
		"  "+ui.StyleAccent.Render("$ nasij workspace create --name my-project --target https://example.com"),
		"  "+ui.StyleAccent.Render("$ nasij scan --target https://example.com --scope scope.yaml"),
	} {
		fmt.Fprintln(w, ex)
	}
	fmt.Fprintln(w)

	versionLine := lipgloss.JoinHorizontal(lipgloss.Left,
		ui.StyleMuted.Render("  nasij "+version.Version),
		ui.StyleMuted.Render("  ·  "),
		ui.StyleMuted.Render(version.Platform()),
		ui.StyleMuted.Render("  ·  "),
		ui.StyleMuted.Render(version.GoVersion()),
	)
	fmt.Fprintln(w, versionLine)
	fmt.Fprintln(w)
}

func renderSubHelp(w io.Writer, cmd *cobra.Command) {
	fmt.Fprintln(w)
	fmt.Fprintln(w, "  "+ui.StyleHeader.Render("USAGE"))
	fmt.Fprintln(w, "    "+ui.StyleWhite.Render(cmd.UseLine()))
	fmt.Fprintln(w)

	if cmd.HasAvailableSubCommands() {
		fmt.Fprintln(w, "  "+ui.StyleHeader.Render("COMMANDS"))
		longest := 0
		for _, c := range cmd.Commands() {
			if len(c.Name()) > longest {
				longest = len(c.Name())
			}
		}
		for _, c := range cmd.Commands() {
			name := ui.StyleAccent.Render(padRight(c.Name(), longest+2))
			desc := ui.StyleMuted.Render(c.Short)
			fmt.Fprintln(w, "    "+name+desc)
		}
		fmt.Fprintln(w)
	}

	if cmd.HasAvailableLocalFlags() {
		fmt.Fprintln(w, "  "+ui.StyleHeader.Render("FLAGS"))
		ui.PrintFlagUsages(w, cmd.LocalFlags(), "    ")
		fmt.Fprintln(w)
	}

	if cmd.HasParent() {
		extra := ui.StyleMuted.Render("Run \"" + cmd.Root().Name() + " " + cmd.Name() + " --help\" for more info.")
		fmt.Fprintln(w, "  "+extra)
		fmt.Fprintln(w)
	}
}


