package authmapper

import (
	"fmt"
	"strings"
)

const mermaidHeader = "sequenceDiagram\n    participant Browser\n    participant Server\n    participant IdP as Identity Provider\n"

func GenerateDiagram(m *Mapping) string {
	if len(m.Flows) == 0 && len(m.JWTs) == 0 && len(m.Cookies) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString(mermaidHeader)

	seenEndpoints := make(map[string]bool)
	stepCount := 0

	for _, flow := range m.Flows {
		b.WriteString(fmt.Sprintf("    Note over Browser,Server: === %s ===\n", escapeLabel(flow.Name)))
		for _, s := range flow.Steps {
			stepCount++
			switch s.Action {
			case "request":
				if s.Method == "" {
					s.Method = "POST"
				}
				label := s.Detail
				if label == "" {
					label = s.URL
				}
				b.WriteString(fmt.Sprintf("    Browser->>Server: %s %s\n", s.Method, escapeLabel(truncateURL(label))))
				seenEndpoints[s.URL] = true

			case "response":
				b.WriteString(fmt.Sprintf("    Server-->>Browser: %s\n", escapeLabel(s.Detail)))

			case "redirect":
				b.WriteString(fmt.Sprintf("    Server-->>Browser: Redirect to\n"))
				b.WriteString(fmt.Sprintf("    Browser->>Server: %s\n", escapeLabel(truncateURL(s.URL))))
				if s.Detail != "" {
					b.WriteString(fmt.Sprintf("    Note over Browser: %s\n", escapeLabel(s.Detail)))
				}

			case "script":
				b.WriteString(fmt.Sprintf("    Note over Browser: %s\n", escapeLabel(s.Detail)))

			default:
				b.WriteString(fmt.Sprintf("    Note over Browser,Server: %s\n", escapeLabel(s.Detail)))
			}
		}
	}

	if len(m.JWTs) > 0 {
		b.WriteString("    Note over Browser,Server: --- Token Summary ---\n")
		for _, j := range m.JWTs {
			label := fmt.Sprintf("%s token (%s)", j.TokenType, j.Algorithm)
			if j.Subject != "" {
				label += fmt.Sprintf(" sub=%s", j.Subject)
			}
			b.WriteString(fmt.Sprintf("    Note over Browser: %s\n", escapeLabel(label)))
			b.WriteString(fmt.Sprintf("    Note over Browser: Source: %s - %s\n", j.Source, escapeLabel(j.Location)))
		}
	}

	if len(m.Cookies) > 0 {
		b.WriteString("    Note over Browser,Server: --- Auth Cookies ---\n")
		for _, c := range m.Cookies {
			label := fmt.Sprintf("🍪 %s (%s)", c.Name, c.TokenType)
			if c.Secure {
				label += " 🔒"
			}
			if c.HttpOnly {
				label += " 👁️"
			}
			b.WriteString(fmt.Sprintf("    Note over Browser: %s\n", escapeLabel(label)))
		}
	}

	return b.String()
}

func escapeLabel(s string) string {
	s = strings.ReplaceAll(s, "\"", "'")
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) > 80 {
		s = s[:77] + "..."
	}
	return s
}

func truncateURL(u string) string {
	if len(u) <= 80 {
		return u
	}
	if idx := strings.Index(u, "?"); idx > 0 && idx < 80 {
		return u[:idx+1] + "..."
	}
	return u[:77] + "..."
}
