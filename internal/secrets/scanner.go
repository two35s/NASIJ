package secrets

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/nasij/nasij/internal/runtime"
)

type Scanner struct {
	MinSeverity Severity
	maxFindings int
}

type Option func(*Scanner)

func WithMinSeverity(s Severity) Option {
	return func(sc *Scanner) {
		sc.MinSeverity = s
	}
}

func WithMaxFindings(n int) Option {
	return func(sc *Scanner) {
		sc.maxFindings = n
	}
}

func NewScanner(opts ...Option) *Scanner {
	s := &Scanner{
		MinSeverity: SeverityInfo,
		maxFindings: 0,
	}
	for _, o := range opts {
		o(s)
	}
	return s
}

func (s *Scanner) ScanText(text string, source string) *ScanResult {
	result := &ScanResult{}
	findings := scanText(text, source)
	for _, f := range findings {
		if f.Severity < s.MinSeverity {
			continue
		}
		result.Findings = append(result.Findings, f)
		if s.maxFindings > 0 && len(result.Findings) >= s.maxFindings {
			break
		}
	}
	return result
}

func (s *Scanner) ScanRuntime(r *runtime.Result) *ScanResult {
	result := &ScanResult{Target: r.URL}

	for i := range r.Requests {
		req := &r.Requests[i]
		for key, val := range req.Headers {
			findings := scanText(key+": "+val, req.URL+" [header: "+key+"]")
			for _, f := range findings {
				if f.Severity >= s.MinSeverity {
					f.Source = req.URL + " [header: " + key + "]"
					result.Findings = append(result.Findings, f)
				}
			}
		}
		findings := scanText(req.URL, req.URL)
		for _, f := range findings {
			if f.Severity >= s.MinSeverity {
				result.Findings = append(result.Findings, f)
			}
		}
	}

	for i := range r.Cookies {
		c := &r.Cookies[i]
		findings := scanText(c.Name+"="+c.Value, "cookie: "+c.Name)
		for _, f := range findings {
			if f.Severity >= s.MinSeverity {
				f.Source = "cookie: " + c.Name
				result.Findings = append(result.Findings, f)
			}
		}
	}

	for i := range r.LocalStorage {
		ls := &r.LocalStorage[i]
		findings := scanText(ls.Key+"="+ls.Value, "localStorage: "+ls.Key)
		for _, f := range findings {
			if f.Severity >= s.MinSeverity {
				f.Source = "localStorage: " + ls.Key
				result.Findings = append(result.Findings, f)
			}
		}
	}

	for i := range r.SessionStorage {
		ss := &r.SessionStorage[i]
		findings := scanText(ss.Key+"="+ss.Value, "sessionStorage: "+ss.Key)
		for _, f := range findings {
			if f.Severity >= s.MinSeverity {
				f.Source = "sessionStorage: " + ss.Key
				result.Findings = append(result.Findings, f)
			}
		}
	}

	for i := range r.IndexedDB {
		db := &r.IndexedDB[i]
		for _, store := range db.Stores {
			for _, record := range store.Records {
				switch v := record.(type) {
				case string:
					findings := scanText(v, "IndexedDB/"+db.Database+"/"+store.Name)
					for _, f := range findings {
						if f.Severity >= s.MinSeverity {
							f.Source = "IndexedDB/" + db.Database + "/" + store.Name
							result.Findings = append(result.Findings, f)
						}
					}
				case map[string]any:
					for k, val := range v {
						if vs, ok := val.(string); ok {
							findings := scanText(k+"="+vs, "IndexedDB/"+db.Database+"/"+store.Name)
							for _, f := range findings {
								if f.Severity >= s.MinSeverity {
									f.Source = "IndexedDB/" + db.Database + "/" + store.Name + "/" + k
									result.Findings = append(result.Findings, f)
								}
							}
						}
					}
				}
			}
		}
	}

	for i := range r.WebSockets {
		ws := &r.WebSockets[i]
		findings := scanText(ws.URL, "websocket: "+ws.URL)
		for _, f := range findings {
			if f.Severity >= s.MinSeverity {
				result.Findings = append(result.Findings, f)
			}
		}
	}

	result.Findings = dedupFindings(result.Findings)

	return result
}

func (s *Scanner) ScanFile(path string) (*ScanResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	source := filepath.Base(path)
	result := s.ScanText(string(data), source)
	result.Target = path
	return result, nil
}

func isLikelyFalsePositive(match, context string) bool {
	lower := strings.ToLower(context)
	prefixes := []string{
		"your-", "your_", "xxxx", "CHANGE_ME",
		"REPLACE_ME", "INSERT_",
	}
	for _, fp := range prefixes {
		if strings.Contains(lower, fp) {
			return true
		}
	}
	if strings.Contains(strings.ToLower(match), "placeholder") {
		return true
	}
	if len(match) < 8 {
		return true
	}
	return false
}

func scanText(text, source string) []Finding {
	var findings []Finding
	seen := make(map[string]bool)

	for _, p := range patterns {
		matches := p.Pattern.FindAllStringSubmatch(text, -1)
		for _, m := range matches {
			fullMatch := m[0]
			dedupKey := string(p.Type) + "|" + string(p.Provider) + "|" + p.Key + "|" + fullMatch

			if seen[dedupKey] {
				continue
			}
			seen[dedupKey] = true

			if isLikelyFalsePositive(fullMatch, text) {
				continue
			}

			var secret string
			if len(m) > 2 && m[2] != "" {
				secret = m[2]
			} else if len(m) > 1 && m[1] != "" && m[1] != fullMatch {
				if len([]rune(m[1])) >= len([]rune(fullMatch))-10 {
					secret = m[1]
				} else {
					secret = fullMatch
				}
			} else {
				secret = fullMatch
			}

			entropy := shannonEntropy(secret)
			ctx := extractContext(text, fullMatch, 60)

			finding := Finding{
				SecretType: p.Type,
				Severity:   p.Severity,
				Provider:   p.Provider,
				Key:        p.Key,
				Match:      truncate(fullMatch, 80),
				Source:     source,
				Context:    ctx,
				Entropy:    entropy,
			}

			if len(secret) > 0 {
				finding.Value = truncate(secret, 200)
			}

			findings = append(findings, finding)
		}
	}

	findings = dedupFindings(findings)
	return findings
}

func extractContext(text, match string, radius int) string {
	idx := strings.Index(text, match)
	if idx < 0 {
		return ""
	}
	start := idx - radius
	if start < 0 {
		start = 0
	}
	end := idx + len(match) + radius
	if end > len(text) {
		end = len(text)
	}
	ctx := text[start:end]
	ctx = strings.ReplaceAll(ctx, "\n", " ")
	ctx = strings.ReplaceAll(ctx, "\r", "")
	return truncate(ctx, 200)
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

func dedupFindings(findings []Finding) []Finding {
	seen := make(map[string]bool)
	var out []Finding
	for _, f := range findings {
		key := string(f.SecretType) + "|" + f.Match + "|" + f.Source
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, f)
	}
	return out
}
