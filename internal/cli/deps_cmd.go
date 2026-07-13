package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/nasij/nasij/internal/container"
	"github.com/nasij/nasij/internal/depintel"
)

func newDepsCmd(c *container.Container) *cobra.Command {
	var (
		target   string
		output   string
		format   string
		minSev   string
		noCVE    bool
	)

	cmd := &cobra.Command{
		Use:   "deps",
		Short: "Analyze dependencies and check for CVEs",
		Long: `Scan project dependencies from common package manifests
(package.json, go.mod, requirements.txt, Gemfile, Cargo.toml,
composer.json, pom.xml) and cross-reference against the OSV.dev
database and local advisory DB for known vulnerabilities.

Detects packages across 7 ecosystems: npm, Go, PyPI, RubyGems,
crates.io, Packagist, and Maven.`,
		Example: `  nasij deps --path /path/to/project
  nasij deps --path /path/to/project --format json
  nasij deps --path /path/to/project --min-severity high
  nasij deps --path /path/to/project --no-cve`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDeps(c, target, output, format, minSev, noCVE)
		},
	}

	cmd.Flags().StringVarP(&target, "path", "p", "", "Project directory to scan")
	cmd.Flags().StringVarP(&output, "output", "o", "", "Output file path")
	cmd.Flags().StringVarP(&format, "format", "", "text", "Output format: text, json")
	cmd.Flags().StringVarP(&minSev, "min-severity", "s", "info", "Minimum CVE severity: info, low, medium, high, critical")
	cmd.Flags().BoolVar(&noCVE, "no-cve", false, "Skip online CVE lookup (OSV.dev)")

	return cmd
}

func runDeps(c *container.Container, target, output, format, minSev string, noCVE bool) error {
	if target == "" {
		return fmt.Errorf("--path is required (e.g. nasij deps --path /path/to/project)")
	}

	c.UI.Info(fmt.Sprintf("Scanning %s for dependencies...", target))

	opts := []depintel.Option{depintel.WithOSVLookup(!noCVE)}
	if sev := parseDepSeverity(minSev); sev != depintel.SeverityUnknown {
		opts = append(opts, depintel.WithMinSeverity(sev))
	}

	scanner := depintel.NewScanner(opts...)
	result, err := scanner.ScanDir(target)
	if err != nil {
		return fmt.Errorf("scan deps: %w", err)
	}

	switch format {
	case "json":
		return outputDepsJSON(result, output)
	default:
		return outputDepsText(result, output)
	}
}

func outputDepsText(r *depintel.DepResult, path string) error {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("Dependency Analysis: %s\n", r.Target))
	b.WriteString(fmt.Sprintf("Total packages: %d, Vulnerable: %d\n\n", r.PackageCount(), r.VulnerableCount()))

	b.WriteString("--- PACKAGES ---\n")
	byEco := make(map[depintel.Ecosystem][]depintel.Package)
	for _, p := range r.Packages {
		byEco[p.Ecosystem] = append(byEco[p.Ecosystem], p)
	}

	for eco, pkgs := range byEco {
		b.WriteString(fmt.Sprintf("\n[%s] %d packages:\n", eco, len(pkgs)))
		for _, p := range pkgs {
			b.WriteString(fmt.Sprintf("  %s @ %s\n", p.Name, p.Version))
		}
	}

	if len(r.Vulnerabilities) > 0 {
		b.WriteString("\n--- VULNERABILITIES ---\n")
		for i, v := range r.Vulnerabilities {
			var sevTag string
			switch v.Severity {
			case depintel.SeverityCritical:
				sevTag = "CRITICAL"
			case depintel.SeverityHigh:
				sevTag = "HIGH"
			case depintel.SeverityMedium:
				sevTag = "MEDIUM"
			case depintel.SeverityLow:
				sevTag = "LOW"
			default:
				sevTag = "UNKNOWN"
			}

			b.WriteString(fmt.Sprintf("\n[%d] [%s] %s\n", i+1, sevTag, v.ID))
			b.WriteString(fmt.Sprintf("     Package: %s @ %s\n", v.AffectedPackage, v.AffectedVersion))
			b.WriteString(fmt.Sprintf("     Summary: %s\n", v.Summary))
			if v.FixedVersion != "" {
				b.WriteString(fmt.Sprintf("     Fix: upgrade to %s\n", v.FixedVersion))
			}
			b.WriteString(fmt.Sprintf("     Source: %s\n", v.Source))
		}
	}

	if r.VulnerableCount() == 0 {
		b.WriteString("\nNo vulnerabilities found.\n")
	}

	out := b.String()
	if path != "" {
		return os.WriteFile(path, []byte(out), 0644)
	}
	fmt.Print(out)
	return nil
}

func outputDepsJSON(r *depintel.DepResult, path string) error {
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

func parseDepSeverity(s string) depintel.Severity {
	switch strings.ToLower(s) {
	case "critical":
		return depintel.SeverityCritical
	case "high":
		return depintel.SeverityHigh
	case "medium":
		return depintel.SeverityMedium
	case "low":
		return depintel.SeverityLow
	case "info":
		return depintel.SeverityUnknown
	default:
		return depintel.SeverityUnknown
	}
}
