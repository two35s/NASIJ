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
	"github.com/nasij/nasij/internal/framework"
	"github.com/nasij/nasij/internal/knowledge"
	"github.com/nasij/nasij/internal/runtime"
	"github.com/nasij/nasij/internal/secrets"
)

func newGraphCmd(c *container.Container) *cobra.Command {
	var (
		target       string
		output       string
		format       string
		search       string
		sourceOnly   string
		noCVE        bool
		noSecrets    bool
		noFrameworks bool
	)

	cmd := &cobra.Command{
		Use:   "graph",
		Short: "Build and query the knowledge graph",
		Long: `Build a unified knowledge graph connecting:
  Page → API → Auth → Cookie → Storage → Framework → Finding → Dependency → CVE

Ingests data from runtime (Playwright crawl), framework detection,
auth mapping, secrets scanning, and dependency intelligence.

Allows full-text search, type filtering, neighbor traversal,
path finding, and JSON/DOT export.`,
		Example: `  nasij graph --target https://example.com
  nasij graph --target https://example.com --format json
  nasij graph --target https://example.com --search "jwt"
  nasij graph --target https://example.com --format dot
  nasij graph --target https://example.com --no-cve --no-secrets`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGraph(c, target, output, format, search, sourceOnly, noCVE, noSecrets, noFrameworks)
		},
	}

	cmd.Flags().StringVarP(&target, "target", "t", "", "Target URL to scan")
	cmd.Flags().StringVarP(&output, "output", "o", "", "Output file path")
	cmd.Flags().StringVarP(&format, "format", "", "text", "Output format: text, json, dot")
	cmd.Flags().StringVarP(&search, "search", "s", "", "Search query (full-text across all nodes)")
	cmd.Flags().StringVarP(&sourceOnly, "source", "", "", "Filter by source type (page, api, finding, etc.)")
	cmd.Flags().BoolVar(&noCVE, "no-cve", false, "Skip CVE lookup (depintel)")
	cmd.Flags().BoolVar(&noSecrets, "no-secrets", false, "Skip secrets scanning")
	cmd.Flags().BoolVar(&noFrameworks, "no-framework", false, "Skip framework detection")

	return cmd
}

func runGraph(c *container.Container, target, output, format, search, sourceOnly string, noCVE, noSecrets, noFrameworks bool) error {
	if target == "" {
		return fmt.Errorf("--target is required")
	}

	c.UI.Info(fmt.Sprintf("Building knowledge graph for %s...", target))
	builder := knowledge.NewBuilder()

	if !noFrameworks {
		c.UI.Info("  Detecting frameworks...")
		fr := framework.DetectFromPaths(".", 1024*100)
		builder.AddFrameworkResult(target, fr)
	}

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
	builder.AddRuntimeResult(target, rr)

	c.UI.Info("  Mapping authentication...")
	am := authmapper.MapFromResult(rr)
	builder.AddAuthMapping(target, am)

	if !noSecrets {
		c.UI.Info("  Scanning for secrets...")
		sc := secrets.NewScanner()
		sr := sc.ScanRuntime(rr)
		builder.AddSecretsResult(sr)
	}

	if !noCVE {
		c.UI.Info("  Analyzing dependencies...")
		ds := depintel.NewScanner()
		dr, err := ds.ScanDir(".")
		if err == nil {
			builder.AddDepResult(dr)
		}
	}

	g := builder.Build()

	switch format {
	case "json":
		return outputGraphJSON(g, search, sourceOnly, output)
	case "dot":
		return outputGraphDOT(g, output)
	default:
		return outputGraphText(g, search, sourceOnly, output)
	}
}

func outputGraphText(g *knowledge.Graph, query, sourceFilter, path string) error {
	var b strings.Builder
	stats := g.Stats()
	b.WriteString(fmt.Sprintf("Knowledge Graph: %d nodes, %d edges\n", stats.TotalNodes, stats.TotalEdges))

	b.WriteString("Nodes by type:\n")
	for _, nt := range []knowledge.NodeType{
		knowledge.NodePage, knowledge.NodeAPIEndpoint, knowledge.NodeCookie,
		knowledge.NodeStorage, knowledge.NodeAuthFlow, knowledge.NodeAuthEndpoint,
		knowledge.NodeFramework, knowledge.NodeFinding, knowledge.NodeDependency,
		knowledge.NodeVulnerability, knowledge.NodeServiceWorker,
	} {
		if count := stats.NodesByType[nt]; count > 0 {
			b.WriteString(fmt.Sprintf("  %s: %d\n", nt, count))
		}
	}

	var nodes []*knowledge.Node
	if query != "" {
		results := g.SearchRanked(query)
		for _, r := range results {
			nodes = append(nodes, r.Node)
		}
	} else if sourceFilter != "" {
		nodes = g.NodesByType(knowledge.NodeType(sourceFilter))
	} else {
		nodes = g.AllNodes()
	}

	b.WriteString(fmt.Sprintf("\nNodes (%d shown):\n", len(nodes)))
	for _, n := range nodes {
		neighborCount := len(g.Neighbors(n.ID))
		b.WriteString(fmt.Sprintf("  [%s] %s (%d neighbors)\n", n.Type, n.Label, neighborCount))

		if query != "" {
			for k, v := range n.Properties {
				if str, ok := v.(string); ok && str != "" {
					if strings.Contains(strings.ToLower(str), strings.ToLower(query)) {
						b.WriteString(fmt.Sprintf("    %s: %s\n", k, str))
					}
				}
			}
		}
	}

	out := b.String()
	if path != "" {
		return os.WriteFile(path, []byte(out), 0644)
	}
	fmt.Print(out)
	return nil
}

func outputGraphJSON(g *knowledge.Graph, query, sourceFilter, path string) error {
	var nodes []*knowledge.Node
	if query != "" {
		results := g.SearchRanked(query)
		for _, r := range results {
			nodes = append(nodes, r.Node)
		}
	} else if sourceFilter != "" {
		nodes = g.NodesByType(knowledge.NodeType(sourceFilter))
	} else {
		nodes = g.AllNodes()
	}

	output := struct {
		Stats knowledge.GraphStats `json:"stats"`
		Nodes []*knowledge.Node    `json:"nodes"`
		Edges []*knowledge.Edge    `json:"edges,omitempty"`
	}{
		Stats: g.Stats(),
		Nodes: nodes,
	}

	if query == "" && sourceFilter == "" {
		output.Edges = g.AllEdges()
	}

	data, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return err
	}
	if path != "" {
		return os.WriteFile(path, data, 0644)
	}
	fmt.Println(string(data))
	return nil
}

func outputGraphDOT(g *knowledge.Graph, path string) error {
	dot := g.ToDOT()
	if path != "" {
		return os.WriteFile(path, []byte(dot), 0644)
	}
	fmt.Print(dot)
	return nil
}
