package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/nasij/nasij/internal/container"
	"github.com/nasij/nasij/internal/runtime"
	"github.com/nasij/nasij/internal/secrets"
)

func newSecretsCmd(c *container.Container) *cobra.Command {
	var (
		target     string
		filePath   string
		output     string
		format     string
		minSev     string
	)

	cmd := &cobra.Command{
		Use:   "secrets",
		Short: "Scan for secrets, API keys, and tokens",
		Long: `Scan a target URL (via Playwright) or a local file for exposed secrets:
API keys, AWS/Azure/GCP credentials, JWT tokens, OAuth secrets,
private keys, connection strings, passwords, and high-entropy strings.

Detects 25+ providers with severity-based classification.`,
		Example: `  nasij secrets --target https://example.com
  nasij secrets --target https://example.com --min-severity high
  nasij secrets scan path/to/file.env
  nasij secrets --target https://example.com --format json --output findings.json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSecrets(c, target, filePath, output, format, minSev)
		},
	}

	cmd.Flags().StringVarP(&target, "target", "t", "", "Target URL to scan with Playwright")
	cmd.Flags().StringVarP(&filePath, "file", "f", "", "Local file path to scan")
	cmd.Flags().StringVarP(&output, "output", "o", "", "Output file path")
	cmd.Flags().StringVarP(&format, "format", "", "text", "Output format: text, json")
	cmd.Flags().StringVarP(&minSev, "min-severity", "s", "info", "Minimum severity: info, low, medium, high, critical")

	return cmd
}

func runSecrets(c *container.Container, target, filePath, output, format, minSev string) error {
	sev := parseSeverity(minSev)
	opts := []secrets.Option{secrets.WithMinSeverity(sev)}

	var result *secrets.ScanResult

	if target != "" {
		c.UI.Info(fmt.Sprintf("Launching browser to scan %s for secrets...", target))
		collector, err := runtime.New()
		if err != nil {
			return fmt.Errorf("runtime init: %w", err)
		}
		defer collector.Close()

		runtimeResult, err := collector.CollectURL(target)
		if err != nil {
			return fmt.Errorf("collect: %w", err)
		}

		scanner := secrets.NewScanner(opts...)
		result = scanner.ScanRuntime(runtimeResult)
	} else if filePath != "" {
		c.UI.Info(fmt.Sprintf("Scanning file %s...", filePath))
		scanner := secrets.NewScanner(opts...)
		var err error
		result, err = scanner.ScanFile(filePath)
		if err != nil {
			return fmt.Errorf("scan file: %w", err)
		}
	} else {
		return fmt.Errorf("either --target or --file is required")
	}

	switch format {
	case "json":
		return outputSecretsJSON(result, output)
	default:
		return outputSecretsText(result, output)
	}
}

func outputSecretsText(r *secrets.ScanResult, path string) error {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("Secrets Scan: %s\n", r.Target))
	bySev := r.CountBySeverity()
	b.WriteString(fmt.Sprintf("Total findings: %d (critical: %d, high: %d, medium: %d, low: %d, info: %d)\n\n",
		len(r.Findings), bySev[secrets.SeverityCritical], bySev[secrets.SeverityHigh],
		bySev[secrets.SeverityMedium], bySev[secrets.SeverityLow], bySev[secrets.SeverityInfo]))

	for i, f := range r.Findings {
		sev := f.Severity.String()
		var sevTag string
		switch f.Severity {
		case secrets.SeverityCritical:
			sevTag = "CRITICAL"
		case secrets.SeverityHigh:
			sevTag = "HIGH"
		case secrets.SeverityMedium:
			sevTag = "MEDIUM"
		case secrets.SeverityLow:
			sevTag = "LOW"
		default:
			sevTag = "INFO"
		}

		b.WriteString(fmt.Sprintf("[%d] [%s] %s — %s\n", i+1, sevTag, f.Provider, f.Key))
		b.WriteString(fmt.Sprintf("     Match: %s\n", f.Match))
		if f.Value != "" && f.Value != f.Match {
			b.WriteString(fmt.Sprintf("     Value: %s\n", f.Value))
		}
		b.WriteString(fmt.Sprintf("     Source: %s\n", f.Source))
		b.WriteString(fmt.Sprintf("     Severity: %s (entropy: %.2f)\n", sev, f.Entropy))
		if f.Context != "" {
			b.WriteString(fmt.Sprintf("     Context: %s\n", f.Context))
		}
		b.WriteString("\n")
	}

	out := b.String()
	if path != "" {
		return os.WriteFile(path, []byte(out), 0644)
	}
	fmt.Print(out)
	return nil
}

func outputSecretsJSON(r *secrets.ScanResult, path string) error {
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

func parseSeverity(s string) secrets.Severity {
	switch strings.ToLower(s) {
	case "critical":
		return secrets.SeverityCritical
	case "high":
		return secrets.SeverityHigh
	case "medium":
		return secrets.SeverityMedium
	case "low":
		return secrets.SeverityLow
	case "info":
		return secrets.SeverityInfo
	default:
		return secrets.SeverityInfo
	}
}
