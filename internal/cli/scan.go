package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/nasij/nasij/internal/container"
	"github.com/nasij/nasij/internal/ui"
)

func newScanCmd(c *container.Container) *cobra.Command {
	var (
		target      string
		scope       string
		concurrency int
		rateLimit   int
		depth       int
		output      string
	)

	cmd := &cobra.Command{
		Use:   "scan",
		Short: "Run a reconnaissance scan against a target",
		Long: `Execute a full reconnaissance scan against the specified target.
Collects JavaScript assets, maps API endpoints, detects frameworks,
and identifies security-relevant findings.

Results are stored in the active workspace database.`,
		Example: `  nasij scan --target https://example.com --scope scope.yaml`,
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runScan(c, target, scope, concurrency, rateLimit, depth, output)
		},
	}

	cmd.Flags().StringVarP(&target, "target", "t", "", "Target URL to scan (required)")
	cmd.Flags().StringVarP(&scope, "scope", "s", "", "Path to scope definition file")
	cmd.Flags().IntVarP(&concurrency, "concurrency", "c", 10, "Worker count")
	cmd.Flags().IntVarP(&rateLimit, "rate-limit", "r", 2, "Requests per second")
	cmd.Flags().IntVarP(&depth, "depth", "d", 3, "Maximum crawl depth")
	cmd.Flags().StringVarP(&output, "output", "o", "", "Output directory for reports")
	_ = cmd.MarkFlagRequired("target")

	return cmd
}

func runScan(c *container.Container, target, scope string, concurrency, rateLimit, depth int, output string) error {
	ui.PrintBanner()
	term := c.UI

	term.Header("Scan")
	term.Divider()
	term.Blank()

	term.Table([][2]string{
		{"Target", ui.StylePrimary.Render(target)},
		{"Scope", ui.StyleMuted.Render(ifEmpty(scope, "none"))},
		{"Concurrency", ui.StyleAccent.Render(fmt.Sprintf("%d workers", concurrency))},
		{"Rate limit", ui.StyleAccent.Render(fmt.Sprintf("%d req/s", rateLimit))},
		{"Max depth", ui.StyleAccent.Render(fmt.Sprintf("%d pages", depth))},
		{"Output", ui.StyleMuted.Render(ifEmpty(output, "default"))},
	})
	term.Blank()
	term.Divider()
	term.Blank()

	// Demo: animated spinner with progress
	runScanDemo(term)
	return nil
}

func runScanDemo(term *ui.Terminal) {
	term.Info(ui.StyleAccent.Render("Initialising scan engine..."))
	time.Sleep(300 * time.Millisecond)

	steps := []struct {
		msg    string
		sleep  time.Duration
		weight float64
	}{
		{"Resolving target", 600, 0.10},
		{"Loading scope rules", 400, 0.05},
		{"Discovering JavaScript assets", 800, 0.20},
		{"Crawling application", 1000, 0.25},
		{"Analyzing JavaScript", 700, 0.15},
		{"Mapping API endpoints", 600, 0.10},
		{"Detecting frameworks", 400, 0.05},
		{"Correlating findings", 500, 0.10},
	}

	var progress float64
	spinChars := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	spinIdx := 0

	for _, step := range steps {
		for i := 0; i < 8; i++ {
			spinIdx = (spinIdx + 1) % len(spinChars)
			bar := renderProgressBar(progress, 50)
			pct := ui.StyleMuted.Render(fmt.Sprintf("%3.0f%%", progress*100))
			spin := ui.StyleAccent.Render(spinChars[spinIdx])
			msg := ui.StyleMuted.Render(step.msg)
			fmt.Fprintf(term.Writer(), "\r  %s  %s  %s  %s", spin, bar, pct, msg)
			time.Sleep(step.sleep / 8)
		}
		progress += step.weight
	}

	progress = 1.0
	bar := ui.StyleSuccess.Render(strings.Repeat("█", 50))
	pct := ui.StyleSuccess.Render("100%")
	fmt.Fprintf(term.Writer(), "\r  %s  %s  %s  %s\n",
		ui.StyleSuccess.Render("✔"),
		bar, pct,
		ui.StyleSuccess.Render("Scan complete"),
	)
	term.Blank()

	term.Success("Scan completed successfully.")
	term.Info(ui.StyleMuted.Render("Full scanning will be implemented in Phase 2."))
	term.Blank()
}


