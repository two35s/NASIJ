package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/nasij/nasij/internal/authmapper"
	"github.com/nasij/nasij/internal/container"
	"github.com/nasij/nasij/internal/depintel"
	"github.com/nasij/nasij/internal/prio"
	"github.com/nasij/nasij/internal/runtime"
	"github.com/nasij/nasij/internal/secrets"
)

func newRankCmd(c *container.Container) *cobra.Command {
	var (
		target    string
		output    string
		format    string
		minPrio   string
		noCVE     bool
		noSecrets bool
		topN      int
	)

	cmd := &cobra.Command{
		Use:   "rank",
		Short: "Prioritize findings by risk scoring",
		Long: `Score and rank all findings across 10 risk factors:
  authentication, admin_routes, uploads, graphql, redirects,
  dangerous_sinks, new_endpoints, secrets, cve, exposure

Ingests data from all subsystems (runtime, authmapper, secrets,
depintel, framework) and produces a unified priority-ranked report.`,
		Example: `  nasij rank --target https://example.com
  nasij rank --target https://example.com --min-prio high
  nasij rank --target https://example.com --top 10
  nasij rank --target https://example.com --format json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRank(c, target, output, format, minPrio, noCVE, noSecrets, topN)
		},
	}

	cmd.Flags().StringVarP(&target, "target", "t", "", "Target URL to scan")
	cmd.Flags().StringVarP(&output, "output", "o", "", "Output file path")
	cmd.Flags().StringVarP(&format, "format", "", "text", "Output format: text, json")
	cmd.Flags().StringVarP(&minPrio, "min-prio", "p", "low", "Minimum priority: none, low, medium, high, critical")
	cmd.Flags().BoolVar(&noCVE, "no-cve", false, "Skip CVE lookup")
	cmd.Flags().BoolVar(&noSecrets, "no-secrets", false, "Skip secrets scanning")
	cmd.Flags().IntVar(&topN, "top", 0, "Show only top N results")

	return cmd
}

func runRank(c *container.Container, target, output, format, minPrio string, noCVE, noSecrets bool, topN int) error {
	if target == "" {
		return fmt.Errorf("--target is required")
	}

	c.UI.Info(fmt.Sprintf("Running prioritization engine on %s...", target))
	scorer := prio.NewScorer()

	c.UI.Info("  Crawling with Playwright...")
	collector, err := runtime.New()
	if err != nil {
		return fmt.Errorf("runtime init: %w", err)
	}
	defer collector.Close()

	rr, err := collector.CollectURL(target)
	if err != nil {
		return fmt.Errorf("collect: %w", err)
	}

	c.UI.Info("  Scoring endpoints...")
	rtResult := scorer.ScoreRuntime(rr)

	c.UI.Info("  Scoring authentication...")
	am := authmapper.MapFromResult(rr)
	authResult := scorer.ScoreAuth(am)

	c.UI.Info("  Scoring secrets...")
	var secResult *prio.Result
	if !noSecrets {
		sc := secrets.NewScanner()
		sr := sc.ScanRuntime(rr)
		secResult = scorer.ScoreSecrets(sr)
	}

	c.UI.Info("  Scoring vulnerabilities...")
	var depResult *prio.Result
	if !noCVE {
		ds := depintel.NewScanner()
		dr, err := ds.ScanDir(".")
		if err == nil {
			depResult = scorer.ScoreDeps(dr)
		}
	}

	combined := prio.NewResult()
	for _, item := range rtResult.Items {
		combined.AddItem(item)
	}
	for _, item := range authResult.Items {
		combined.AddItem(item)
	}
	if secResult != nil {
		for _, item := range secResult.Items {
			combined.AddItem(item)
		}
	}
	if depResult != nil {
		for _, item := range depResult.Items {
			combined.AddItem(item)
		}
	}

	minP := parseRankPriority(minPrio)
	filtered := prio.NewResult()
	for _, item := range combined.Items {
		if item.Priority >= minP {
			filtered.AddItem(item)
		}
	}

	if topN > 0 && topN < len(filtered.Items) {
		filtered.Items = filtered.Items[:topN]
	}

	switch format {
	case "json":
		return outputRankJSON(filtered, output)
	default:
		return outputRankText(filtered, output)
	}
}

func outputRankText(r *prio.Result, path string) error {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("Prioritization Results: %d items\n", len(r.Items)))

	byPrio := []prio.Priority{
		prio.PriorityCritical, prio.PriorityHigh, prio.PriorityMedium,
		prio.PriorityLow, prio.PriorityNone,
	}
	for _, p := range byPrio {
		if count := r.Count[p]; count > 0 {
			b.WriteString(fmt.Sprintf("  %s: %d\n", p, count))
		}
	}

	b.WriteString("\n--- RANKED FINDINGS ---\n")
	for i, item := range r.Items {
		b.WriteString(fmt.Sprintf("\n[%d] [%s] Score: %.1f — %s\n", i+1, item.Priority, item.Score, item.Label))
		b.WriteString(fmt.Sprintf("     Type: %s | ID: %s\n", item.ItemType, item.ID))
		if item.Summary != "" {
			b.WriteString(fmt.Sprintf("     Factors: %s\n", item.Summary))
		}
		breakdownParts := make([]string, 0)
		for factor, score := range item.Breakdown {
			breakdownParts = append(breakdownParts, fmt.Sprintf("%s=%.1f", factor, score))
		}
		b.WriteString(fmt.Sprintf("     Breakdown: [%s]\n", strings.Join(breakdownParts, ", ")))
	}

	out := b.String()
	if path != "" {
		return os.WriteFile(path, []byte(out), 0644)
	}
	fmt.Print(out)
	return nil
}

func outputRankJSON(r *prio.Result, path string) error {
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	if path != "" {
		return os.WriteFile(path, data, 0644)
	}
	fmt.Println(string(data))
	return nil
}

func parseRankPriority(s string) prio.Priority {
	switch strings.ToLower(s) {
	case "critical":
		return prio.PriorityCritical
	case "high":
		return prio.PriorityHigh
	case "medium":
		return prio.PriorityMedium
	case "low":
		return prio.PriorityLow
	case "none":
		return prio.PriorityNone
	default:
		return prio.PriorityLow
	}
}
