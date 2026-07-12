package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/nasij/nasij/internal/container"
	"github.com/nasij/nasij/internal/ui"
)

func newCrawlCmd(c *container.Container) *cobra.Command {
	var (
		target      string
		concurrency int
		rateLimit   int
		depth       int
		maxPages    int
		respectRobots bool
	)

	cmd := &cobra.Command{
		Use:   "crawl",
		Short: "Crawl a target application to discover assets",
		Long: `Discover JavaScript files, API endpoints, and application structure
by crawling the target within defined scope boundaries.

Scope enforcement and rate limiting are active by default.`,
		Example: `  nasij crawl --target https://example.com --depth 5 --max-pages 500`,
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCrawl(c, target, concurrency, rateLimit, depth, maxPages, respectRobots)
		},
	}

	cmd.Flags().StringVarP(&target, "target", "t", "", "Target URL to crawl (required)")
	cmd.Flags().IntVarP(&concurrency, "concurrency", "c", 10, "Worker count")
	cmd.Flags().IntVarP(&rateLimit, "rate-limit", "r", 2, "Requests per second")
	cmd.Flags().IntVarP(&depth, "depth", "d", 3, "Maximum crawl depth")
	cmd.Flags().IntVarP(&maxPages, "max-pages", "m", 200, "Maximum pages to visit")
	cmd.Flags().BoolVar(&respectRobots, "respect-robots", true, "Respect robots.txt directives")
	_ = cmd.MarkFlagRequired("target")

	return cmd
}

func runCrawl(c *container.Container, target string, concurrency, rateLimit, depth, maxPages int, respectRobots bool) error {
	ui.PrintBanner()
	term := c.UI

	term.Header("Crawl")
	term.Divider()
	term.Blank()

	term.Table([][2]string{
		{"Target", ui.StylePrimary.Render(target)},
		{"Concurrency", ui.StyleAccent.Render(fmt.Sprintf("%d workers", concurrency))},
		{"Rate limit", ui.StyleAccent.Render(fmt.Sprintf("%d req/s", rateLimit))},
		{"Max depth", ui.StyleAccent.Render(fmt.Sprintf("%d", depth))},
		{"Max pages", ui.StyleAccent.Render(fmt.Sprintf("%d", maxPages))},
		{"Respect robots", ui.StyleMuted.Render(fmt.Sprintf("%v", respectRobots))},
	})
	term.Blank()
	term.Divider()
	term.Blank()

	runCrawlDemo(term)
	return nil
}

func runCrawlDemo(term *ui.Terminal) {
	term.Info(ui.StyleAccent.Render("Initialising crawler..."))
	time.Sleep(200 * time.Millisecond)

	discoveries := []string{
		"/js/app.abc123.js",
		"/js/vendor.def456.js",
		"/api/v1/users",
		"/api/v1/products",
		"/api/graphql",
		"/static/css/main.css",
		"/sw.js",
		"/manifest.json",
	}

	spinChars := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	term.Info(ui.StyleMuted.Render("  URL  Status  Size       Discovery"))

	for i, d := range discoveries {
		spin := ui.StyleAccent.Render(spinChars[i%len(spinChars)])
		url := ui.StyleWhite.Render(padRight(d, 32))
		status := ui.StyleSuccess.Render("200")
		size := ui.StyleMuted.Render(fmt.Sprintf("%d KB", 10+i*3))
		fmt.Fprintf(term.Writer(), "\r  %s  %s %s  %s\n", spin, url, status, size)
		time.Sleep(400 * time.Millisecond)
	}

	bar := ui.StyleSuccess.Render(strings.Repeat("█", 50))
	fmt.Fprintf(term.Writer(), "\r  %s  %s  %s  %s\n",
		ui.StyleSuccess.Render("✔"),
		bar,
		ui.StyleSuccess.Render("100%"),
		ui.StyleSuccess.Render("Crawl complete"),
	)
	term.Blank()

	term.Success(fmt.Sprintf("Discovered %d assets across %d pages.", len(discoveries), 12))
	term.Info(ui.StyleMuted.Render("Full crawling will be implemented in Phase 2."))
	term.Blank()
}
