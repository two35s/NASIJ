package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/nasij/nasij/internal/authmapper"
	"github.com/nasij/nasij/internal/container"
	"github.com/nasij/nasij/internal/runtime"
)

func newAuthCmd(c *container.Container) *cobra.Command {
	var (
		target string
		output string
		format string
	)

	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Map authentication mechanisms of a target",
		Long: `Visit a target URL with Playwright and map all authentication mechanisms:
JWT tokens, OAuth flows, cookies, login/logout/refresh endpoints,
tokens in localStorage/sessionStorage/IndexedDB, and service workers.

Outputs a Mermaid sequence diagram of the auth flows.`,
		Example: `  nasij auth --target https://example.com
  nasij auth --target https://example.com --format json
  nasij auth --target https://example.com --output auth-diagram.mmd`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAuth(c, target, output, format)
		},
	}

	cmd.Flags().StringVarP(&target, "target", "t", "", "Target URL to analyze (required)")
	cmd.Flags().StringVarP(&output, "output", "o", "", "Output file path")
	cmd.Flags().StringVarP(&format, "format", "f", "text", "Output format: text, json, diagram")

	cmd.MarkFlagRequired("target")

	return cmd
}

func runAuth(c *container.Container, target, output, format string) error {
	c.UI.Info(fmt.Sprintf("Launching browser to collect runtime data from %s...", target))

	collector, err := runtime.New()
	if err != nil {
		return fmt.Errorf("runtime init: %w", err)
	}
	defer collector.Close()

	result, err := collector.CollectURL(target)
	if err != nil {
		return fmt.Errorf("collect: %w", err)
	}

	c.UI.Info(fmt.Sprintf("Collected %d requests, %d cookies, %d storage entries",
		len(result.Requests), len(result.Cookies),
		len(result.LocalStorage)+len(result.SessionStorage)))

	m := authmapper.MapFromResult(result)

	c.UI.Info(fmt.Sprintf("Found %d JWT tokens, %d OAuth flows, %d auth endpoints, %d flows",
		len(m.JWTs), len(m.OAuthFlows), len(m.Endpoints), len(m.Flows)))

	switch format {
	case "json":
		return outputAuthJSON(m, output)
	case "diagram":
		return outputAuthDiagram(m, output)
	default:
		return outputAuthText(m, output)
	}
}

func outputAuthText(m *authmapper.Mapping, path string) error {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("Auth Mapping for: %s\n\n", m.Source))

	if len(m.JWTs) > 0 {
		b.WriteString("=== JWT Tokens ===\n")
		for i, j := range m.JWTs {
			b.WriteString(fmt.Sprintf("  %d. Type: %s, Algorithm: %s\n", i+1, j.TokenType, j.Algorithm))
			b.WriteString(fmt.Sprintf("     Source: %s (%s)\n", j.Source, j.Location))
			if j.Subject != "" {
				b.WriteString(fmt.Sprintf("     Subject: %s\n", j.Subject))
			}
			if j.Issuer != "" {
				b.WriteString(fmt.Sprintf("     Issuer: %s\n", j.Issuer))
			}
			if len(j.Scopes) > 0 {
				b.WriteString(fmt.Sprintf("     Scopes: %s\n", strings.Join(j.Scopes, ", ")))
			}
			if j.ExpiresAt != nil {
				b.WriteString(fmt.Sprintf("     Expires: %s\n", j.ExpiresAt.Format(time.DateTime)))
			}
		}
		b.WriteString("\n")
	}

	if len(m.OAuthFlows) > 0 {
		b.WriteString("=== OAuth Flows ===\n")
		for i, of := range m.OAuthFlows {
			b.WriteString(fmt.Sprintf("  %d. Type: %s\n", i+1, of.Type))
			if of.AuthEndpoint != "" {
				b.WriteString(fmt.Sprintf("     Auth Endpoint: %s\n", of.AuthEndpoint))
			}
			if of.TokenEndpoint != "" {
				b.WriteString(fmt.Sprintf("     Token Endpoint: %s\n", of.TokenEndpoint))
			}
			if of.ClientID != "" {
				b.WriteString(fmt.Sprintf("     Client ID: %s\n", of.ClientID))
			}
			if of.PKCE {
				b.WriteString("     PKCE: true\n")
			}
			if of.OIDC {
				b.WriteString("     OIDC: true\n")
			}
		}
		b.WriteString("\n")
	}

	if len(m.Cookies) > 0 {
		b.WriteString("=== Auth Cookies ===\n")
		for _, c := range m.Cookies {
			tag := c.TokenType
			if c.Secure {
				tag += " [Secure]"
			}
			if c.HttpOnly {
				tag += " [HttpOnly]"
			}
			if c.Suspicious {
				tag += " [SUSPICIOUS]"
			}
			b.WriteString(fmt.Sprintf("  %s (%s)\n", c.Name, tag))
		}
		b.WriteString("\n")
	}

	if len(m.Endpoints) > 0 {
		b.WriteString("=== Auth Endpoints ===\n")
		for _, ep := range m.Endpoints {
			b.WriteString(fmt.Sprintf("  %s %s (%s)\n", ep.Method, ep.URL, ep.EndpointType))
		}
		b.WriteString("\n")
	}

	if len(m.StorageTokens) > 0 {
		b.WriteString("=== Storage Tokens ===\n")
		for _, st := range m.StorageTokens {
			b.WriteString(fmt.Sprintf("  [%s] %s = %s (type: %s)\n", st.Source, st.Key, st.Value[:min(len(st.Value), 40)], st.TokenType))
		}
		b.WriteString("\n")
	}

	if len(m.Flows) > 0 {
		b.WriteString("=== Auth Flows ===\n")
		for _, f := range m.Flows {
			b.WriteString(fmt.Sprintf("  Flow: %s\n", f.Name))
			for _, step := range f.Steps {
				b.WriteString(fmt.Sprintf("    %d. [%s] %s\n", step.Order, step.Action, step.Detail))
			}
		}
		b.WriteString("\n")
	}

	if m.Diagram != "" {
		b.WriteString("=== Mermaid Diagram ===\n")
		b.WriteString(m.Diagram)
		b.WriteString("\n")
	}

	out := b.String()
	if path != "" {
		return os.WriteFile(path, []byte(out), 0644)
	}
	fmt.Print(out)
	return nil
}

func outputAuthJSON(m *authmapper.Mapping, path string) error {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	if path != "" {
		return os.WriteFile(path, data, 0644)
	}
	fmt.Println(string(data))
	return nil
}

func outputAuthDiagram(m *authmapper.Mapping, path string) error {
	diagram := m.Diagram
	if diagram == "" {
		return fmt.Errorf("no auth flows detected to diagram")
	}
	if path != "" {
		return os.WriteFile(path, []byte(diagram), 0644)
	}
	fmt.Println(diagram)
	return nil
}
