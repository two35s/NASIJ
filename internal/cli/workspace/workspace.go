package workspace

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/nasij/nasij/internal/container"
	"github.com/nasij/nasij/internal/storage"
	"github.com/nasij/nasij/internal/ui"
)

func padRight(s string, n int) string {
	if len(s) >= n {
		return s
	}
	return s + strings.Repeat(" ", n-len(s))
}

func NewWorkspaceCmd(c *container.Container) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "workspace",
		Short: "Manage reconnaissance workspaces",
		Long: `Workspaces are isolated units of reconnaissance state.
Each workspace has its own SQLite database, downloaded assets, reports, and audit logs.`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	cmd.SetHelpFunc(customHelpFunc(c))

	cmd.AddCommand(newCreateCmd(c))
	cmd.AddCommand(newListCmd(c))
	cmd.AddCommand(newResumeCmd(c))
	cmd.AddCommand(newDeleteCmd(c))
	cmd.AddCommand(newInfoCmd(c))

	return cmd
}

func customHelpFunc(c *container.Container) func(*cobra.Command, []string) {
	return func(cmd *cobra.Command, args []string) {
		w := c.UI.Writer()

		fmt.Fprintln(w)
		fmt.Fprintln(w, "  "+ui.StyleHeader.Render("USAGE"))
		fmt.Fprintln(w, "    "+ui.StyleWhite.Render(cmd.UseLine()))
		fmt.Fprintln(w)

		if cmd.HasAvailableSubCommands() {
			fmt.Fprintln(w, "  "+ui.StyleHeader.Render("COMMANDS"))
			cmds := []struct{ name, desc string }{
				{"create", "Create a new reconnaissance workspace"},
				{"list", "List all workspaces"},
				{"resume", "Resume and verify a workspace"},
				{"delete", "Permanently delete a workspace"},
				{"info", "Show detailed workspace information"},
			}
			for _, c := range cmds {
				n := ui.StyleAccent.Render(padRight(c.name, 12))
				d := ui.StyleMuted.Render(c.desc)
				fmt.Fprintln(w, "    "+n+d)
			}
			fmt.Fprintln(w)
		}

		if cmd.HasAvailableLocalFlags() {
			fmt.Fprintln(w, "  "+ui.StyleHeader.Render("FLAGS"))
			ui.PrintFlagUsages(w, cmd.LocalFlags(), "    ")
			fmt.Fprintln(w)
		}

		extra := ui.StyleMuted.Render(`Run "nasij workspace --help" for more info.`)
		fmt.Fprintln(w, "  "+extra)
		fmt.Fprintln(w)
	}
}

func newCreateCmd(c *container.Container) *cobra.Command {
	var (
		name   string
		target string
	)

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new reconnaissance workspace",
		Long: `Provisions a new workspace directory and initialises its SQLite database.

Example:
  nasij workspace create --name my-recon --target https://example.com`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCreate(cmd.Context(), c, name, target)
		},
	}

	cmd.Flags().StringVarP(&name, "name", "n", "", "Human-readable workspace label (required)")
	cmd.Flags().StringVarP(&target, "target", "t", "", "Primary target URL (required)")
	_ = cmd.MarkFlagRequired("name")
	_ = cmd.MarkFlagRequired("target")

	return cmd
}

func newListCmd(c *container.Container) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all workspaces",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runList(cmd.Context(), c)
		},
	}
}

func runList(ctx context.Context, c *container.Container) error {
	_ = ctx
	term := c.UI

	workspaces, err := c.Workspace.List()
	if err != nil {
		return fmt.Errorf("workspace list: %w", err)
	}

	if len(workspaces) == 0 {
		term.Info("No workspaces found. Run `nasij workspace create` to get started.")
		term.Blank()
		return nil
	}

	term.Header(fmt.Sprintf("Workspaces (%d)", len(workspaces)))
	term.Divider()
	term.Blank()

	for _, ws := range workspaces {
		term.Subheader("  " + ws.Name)
		term.Table([][2]string{
			{"ID", ws.ID},
			{"Target", ws.Target},
			{"Created", ws.CreatedAt.Format("2006-01-02 15:04:05 UTC")},
			{"Scans", fmt.Sprintf("%d", ws.ScanCount)},
		})
		term.Blank()
	}

	return nil
}

func newResumeCmd(c *container.Container) *cobra.Command {
	return &cobra.Command{
		Use:   "resume <id>",
		Short: "Resume and verify a workspace",
		Long: `Verify a workspace's directory structure and content integrity,
then set its status back to idle for scanning.

Recreates any missing subdirectories on the next scan. Reports integrity
warnings if workspace metadata has been externally modified.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runResume(c, args[0])
		},
	}
}

func runResume(c *container.Container, id string) error {
	ui.PrintBanner()
	term := c.UI

	term.Header("Resuming Workspace")
	term.Divider()
	term.Blank()

	ws, err := c.Workspace.Resume(id)
	if err != nil {
		term.Warning("Integrity issues: " + err.Error())
		term.Blank()
		term.Info("Run `nasij workspace info " + id[:8] + "` to inspect workspace state.")
		term.Blank()
		return nil
	}

	term.StatusRow(true, "Workspace", ws.Name)
	term.StatusRow(true, "Status", ws.Status)
	term.StatusRow(true, "Target", ws.Target)
	term.Blank()
	term.Divider()
	term.Blank()

	term.Success("Workspace verified and ready.")
	term.Blank()
	term.Info("Run `nasij scan` to start a scan on this workspace.")
	term.Blank()

	return nil
}

func newDeleteCmd(c *container.Container) *cobra.Command {
	return &cobra.Command{
		Use:   "delete <id>",
		Short: "Permanently delete a workspace",
		Long: `Removes the workspace directory, SQLite database, and all associated data.
This action cannot be undone.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDelete(c, args[0])
		},
	}
}

func runDelete(c *container.Container, id string) error {
	term := c.UI

	if err := c.Workspace.Delete(id); err != nil {
		term.Error("Failed to delete workspace: " + err.Error())
		return fmt.Errorf("workspace delete: %w", err)
	}

	term.Success("Workspace deleted: " + id)
	term.Blank()
	return nil
}

func newInfoCmd(c *container.Container) *cobra.Command {
	return &cobra.Command{
		Use:   "info <id>",
		Short: "Show detailed workspace information",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInfo(c, args[0])
		},
	}
}

func runInfo(c *container.Container, id string) error {
	ui.PrintBanner()
	term := c.UI

	ws, err := c.Workspace.Open(id)
	if err != nil {
		term.Error("Workspace not found: " + id)
		return fmt.Errorf("workspace info: %w", err)
	}

	term.Header("Workspace: " + ws.Name)
	term.Divider()
	term.Blank()

	statusStyle := ui.StyleSuccess
	if ws.Status == "scanning" {
		statusStyle = ui.StylePrimary
	} else if ws.Status == "error" {
		statusStyle = ui.StyleDanger
	} else if ws.Status == "paused" {
		statusStyle = ui.StyleWarning
	}
	term.Table([][2]string{
		{"ID", ui.StyleAccent.Render(ws.ID)},
		{"Name", ui.StyleBold.Render(ws.Name)},
		{"Target", ui.StylePrimary.Render(ws.Target)},
		{"Status", statusStyle.Render(ws.Status)},
		{"Version", ui.StyleMuted.Render(fmt.Sprintf("v%d", ws.Version))},
		{"Created", ws.CreatedAt.Format("2006-01-02 15:04:05 UTC")},
		{"Modified", ws.ModifiedAt.Format("2006-01-02 15:04:05 UTC")},
		{"Scans", fmt.Sprintf("%d", ws.ScanCount)},
		{"Root", ws.Root},
	})

	term.Blank()
	term.Subheader("  Paths")
	term.Table([][2]string{
		{"Database", ws.DBPath()},
		{"Assets", ws.AssetsPath()},
		{"Reports", ws.ReportsPath()},
		{"Logs", ws.LogsPath()},
		{"Cache", ws.CachePath()},
		{"Screenshots", ws.ScreenshotsPath()},
		{"Snapshots", ws.SnapshotsPath()},
	})

	if ws.Hash != "" {
		term.Blank()
		term.Subheader("  Integrity")
		term.Table([][2]string{
			{"Hash", ui.StyleMuted.Render(ws.Hash[:16] + "...")},
		})
	}

	term.Blank()
	term.Divider()
	term.Blank()
	term.Info("Run `nasij scan --workspace " + ws.ID[:8] + "` to scan this workspace.")
	term.Blank()

	return nil
}

func runCreate(ctx context.Context, c *container.Container, name, target string) error {
	ui.PrintBanner()

	term := c.UI
	term.Header("Creating Workspace")
	term.Divider()
	term.Blank()

	ws, err := c.Workspace.Create(name, target)
	if err != nil {
		term.Error("Failed to create workspace: " + err.Error())
		return fmt.Errorf("workspace create: %w", err)
	}

	db, err := storage.Open(ctx, ws.DBPath())
	if err != nil {
		term.Error("Workspace directories created but database initialisation failed: " + err.Error())
		return fmt.Errorf("workspace create: db: %w", err)
	}
	_ = db.Close()

	term.StatusRow(true, "Workspace directory", ws.Root)
	term.StatusRow(true, "SQLite database", ws.DBPath()+" (migrations applied)")
	term.StatusRow(true, "Assets directory", ws.AssetsPath())
	term.StatusRow(true, "Reports directory", ws.ReportsPath())
	term.StatusRow(true, "Cache directory", ws.CachePath())
	term.StatusRow(true, "Screenshots directory", ws.ScreenshotsPath())
	term.StatusRow(true, "Snapshots directory", ws.SnapshotsPath())
	term.Blank()
	term.Divider()
	term.Blank()

	term.Success("Workspace created successfully.")
	term.Blank()

	term.Table([][2]string{
		{"ID", ui.StyleAccent.Render(ws.ID)},
		{"Name", ui.StyleBold.Render(ws.Name)},
		{"Target", ui.StylePrimary.Render(ws.Target)},
		{"Status", ui.StyleSuccess.Render(ws.Status)},
		{"Version", ui.StyleMuted.Render(fmt.Sprintf("v%d", ws.Version))},
		{"Created", ws.CreatedAt.Format("2006-01-02 15:04:05 UTC")},
		{"Root", ws.Root},
	})

	term.Blank()
	term.Info("Run `nasij workspace list` to see all workspaces.")
	term.Blank()

	return nil
}
