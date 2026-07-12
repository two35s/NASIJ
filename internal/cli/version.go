package cli

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"github.com/nasij/nasij/internal/container"
	"github.com/nasij/nasij/internal/ui"
	"github.com/nasij/nasij/pkg/version"
)

// newVersionCmd returns the `nasij version` command.
// Prints a formatted version table without displaying the full banner.
func newVersionCmd(c *container.Container) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print NASIJ version information",
		Long:  "Prints version, build metadata, Go runtime, and platform information.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runVersion(c)
		},
	}
}

func runVersion(c *container.Container) error {
	term := c.UI

	// Title block
	title := lipgloss.NewStyle().
		Foreground(ui.ColorPrimary).
		Bold(true).
		Render("  NASIJ")
	subtitle := ui.StyleMuted.Render(" — Intelligent JavaScript Reconnaissance Framework")
	fmt.Println(title + subtitle)
	term.Blank()

	// Version table
	term.Table([][2]string{
		{"Version", ui.StyleAccent.Render(version.Version)},
		{"Git Commit", version.GitCommit},
		{"Build Date", version.BuildDate},
		{"Go Runtime", version.GoVersion()},
		{"Platform", version.Platform()},
	})

	term.Blank()
	return nil
}
