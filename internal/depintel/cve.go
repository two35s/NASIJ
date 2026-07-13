package depintel

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const osvBaseURL = "https://api.osv.dev/v1"

type osvQuery struct {
	Package struct {
		Name      string `json:"name"`
		Ecosystem string `json:"ecosystem"`
	} `json:"package"`
	Version string `json:"version"`
}

type osvQueryBatch struct {
	Queries []osvQuery `json:"queries"`
}

type osvResponse struct {
	Results []osvQueryResult `json:"results"`
}

type osvQueryResult struct {
	Vulns []osvVuln `json:"vulns,omitempty"`
}

type osvVuln struct {
	ID      string   `json:"id"`
	Aliases []string `json:"aliases,omitempty"`
	Summary string   `json:"summary,omitempty"`
	Severity []struct {
		Type  string `json:"type"`
		Score string `json:"score"`
	} `json:"severity,omitempty"`
	DatabaseSpecific struct {
		Severity string `json:"severity,omitempty"`
	} `json:"database_specific,omitempty"`
	Affected []struct {
		Package struct {
			Name      string `json:"name"`
			Ecosystem string `json:"ecosystem"`
		} `json:"package"`
		Ranges []struct {
			Type  string `json:"type"`
			Events []struct {
				Introduced string `json:"introduced,omitempty"`
				Fixed      string `json:"fixed,omitempty"`
				LastAffected string `json:"last_affected,omitempty"`
			} `json:"events"`
		} `json:"ranges"`
		Versions []string `json:"versions,omitempty"`
	} `json:"affected,omitempty"`
}

type OSVClient struct {
	client  *http.Client
	baseURL string
}

func NewOSVClient() *OSVClient {
	return &OSVClient{
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
		baseURL: osvBaseURL,
	}
}

func mapEcosystem(eco Ecosystem) string {
	return string(eco)
}

func (c *OSVClient) Query(pkg Package) ([]Vulnerability, error) {
	results, err := c.QueryBatch([]Package{pkg})
	if err != nil {
		return nil, err
	}
	return results[pkg], nil
}

func (c *OSVClient) QueryBatch(pkgs []Package) (map[Package][]Vulnerability, error) {
	result := make(map[Package][]Vulnerability)

	if len(pkgs) == 0 {
		return result, nil
	}

	batch := osvQueryBatch{}
	for _, pkg := range pkgs {
		if pkg.Version == "" {
			continue
		}
		q := osvQuery{}
		q.Package.Name = pkg.Name
		q.Package.Ecosystem = mapEcosystem(pkg.Ecosystem)
		q.Version = pkg.Version
		batch.Queries = append(batch.Queries, q)
	}

	if len(batch.Queries) == 0 {
		return result, nil
	}

	body, err := json.Marshal(batch)
	if err != nil {
		return nil, fmt.Errorf("marshal query: %w", err)
	}

	resp, err := c.client.Post(c.baseURL+"/querybatch", "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("osv query: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("osv returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var osvResp osvResponse
	if err := json.Unmarshal(respBody, &osvResp); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	for i, qr := range osvResp.Results {
		if i >= len(pkgs) {
			break
		}
		pkg := pkgs[i]
		var vulns []Vulnerability
		for _, v := range qr.Vulns {
			vuln := Vulnerability{
				ID:              v.ID,
				Aliases:         v.Aliases,
				Summary:         v.Summary,
				AffectedPackage: pkg.Name,
				AffectedVersion: pkg.Version,
				Ecosystem:       pkg.Ecosystem,
				Source:          "OSV",
			}

			vuln.Severity = parseOSVSeverity(v)

			for _, a := range v.Affected {
				for _, r := range a.Ranges {
					for _, e := range r.Events {
						if e.Fixed != "" {
							vuln.FixedVersion = e.Fixed
						}
					}
				}
			}

			vulns = append(vulns, vuln)
		}
		result[pkgs[i]] = vulns
	}

	return result, nil
}

func parseOSVSeverity(v osvVuln) Severity {
	if v.DatabaseSpecific.Severity != "" {
		switch strings.ToLower(v.DatabaseSpecific.Severity) {
		case "critical":
			return SeverityCritical
		case "high":
			return SeverityHigh
		case "medium":
			return SeverityMedium
		case "low":
			return SeverityLow
		}
	}
	for _, s := range v.Severity {
		if s.Type == "CVSS_V3" || s.Type == "CVSS_V2" {
			score := 0.0
			fmt.Sscanf(s.Score, "%f", &score)
			return cvssToSeverity(score)
		}
	}
	return SeverityUnknown
}

func cvssToSeverity(score float64) Severity {
	switch {
	case score >= 9.0:
		return SeverityCritical
	case score >= 7.0:
		return SeverityHigh
	case score >= 4.0:
		return SeverityMedium
	case score > 0:
		return SeverityLow
	default:
		return SeverityUnknown
	}
}

func osvSeverityString(s Severity) string {
	switch s {
	case SeverityCritical:
		return "CRITICAL"
	case SeverityHigh:
		return "HIGH"
	case SeverityMedium:
		return "MEDIUM"
	case SeverityLow:
		return "LOW"
	default:
		return "UNKNOWN"
	}
}

var severityMap = map[string]Severity{
	"CRITICAL": SeverityCritical,
	"HIGH":     SeverityHigh,
	"MEDIUM":   SeverityMedium,
	"LOW":      SeverityLow,
}

func SeverityFromString(s string) Severity {
	if sev, ok := severityMap[strings.ToUpper(strings.TrimSpace(s))]; ok {
		return sev
	}
	return SeverityUnknown
}
