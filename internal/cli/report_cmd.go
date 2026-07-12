package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/nasij/nasij/internal/container"
	"github.com/nasij/nasij/internal/ui"
)

func newReportCmd(c *container.Container) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "report",
		Short: "Generate and view scan reports",
		Long: `List, view, and export scan reports from workspaces.
Reports include asset inventories, API maps, framework detection,
findings, and architecture diagrams.

Supported formats: HTML, Markdown, JSON.`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	cmd.SetHelpFunc(helpFunc(c))

	cmd.AddCommand(
		newReportListCmd(c),
		newReportViewCmd(c),
		newReportExportCmd(c),
	)

	return cmd
}

func newReportListCmd(c *container.Container) *cobra.Command {
	var workspaceID string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List available reports in a workspace",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runReportList(c, workspaceID)
		},
	}

	cmd.Flags().StringVarP(&workspaceID, "workspace", "w", "", "Workspace ID (required)")
	_ = cmd.MarkFlagRequired("workspace")

	return cmd
}

func newReportViewCmd(c *container.Container) *cobra.Command {
	return &cobra.Command{
		Use:   "view <id>",
		Short: "View a specific report",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runReportView(c, args[0])
		},
	}
}

func newReportExportCmd(c *container.Container) *cobra.Command {
	var format string

	cmd := &cobra.Command{
		Use:   "export <id>",
		Short: "Export a report in the specified format",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runReportExport(c, args[0], format)
		},
	}

	cmd.Flags().StringVarP(&format, "format", "f", "html", "Output format (html|markdown|json)")
	return cmd
}

func runReportList(c *container.Container, workspaceID string) error {
	ui.PrintBanner()
	term := c.UI

	ws, err := c.Workspace.Open(workspaceID)
	if err != nil {
		term.Error("Workspace not found: " + workspaceID)
		return fmt.Errorf("report list: %w", err)
	}

	term.Header(fmt.Sprintf("Reports — %s", ws.Name))
	term.Divider()
	term.Blank()
	term.Info(ui.StyleMuted.Render("No reports yet. Run a scan to generate reports."))
	term.Blank()
	term.Info(ui.StyleMuted.Render("Reports directory: " + ws.ReportsPath()))
	term.Blank()
	term.Info(ui.StyleMuted.Render("Report generation will be implemented in Phase 2."))
	term.Blank()

	return nil
}

func runReportView(c *container.Container, id string) error {
	ui.PrintBanner()
	term := c.UI
	term.Header("Report: " + id)
	term.Divider()
	term.Blank()
	term.Info(ui.StyleMuted.Render("Report viewing will be implemented in Phase 2."))
	term.Blank()
	return nil
}

func runReportExport(c *container.Container, id, format string) error {
	ui.PrintBanner()
	term := c.UI
	term.Header(fmt.Sprintf("Export: %s (%s)", id, format))
	term.Divider()
	term.Blank()
	term.Info(ui.StyleMuted.Render("Report export will be implemented in Phase 2."))
	term.Blank()
	return nil
}
