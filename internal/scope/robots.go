package scope

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// RobotsTxt holds parsed robots.txt rules.
type RobotsTxt struct {
	DisallowedPaths []string
	AllowedPaths    []string
	CrawlDelay      time.Duration
	Sitemaps        []string
}

// FetchRobotsTxt fetches and parses /robots.txt from the given base URL.
func FetchRobotsTxt(ctx context.Context, baseURL string, client *http.Client) (*RobotsTxt, string, error) {
	if client == nil {
		client = http.DefaultClient
	}

	u, err := url.Parse(baseURL)
	if err != nil {
		return nil, "", fmt.Errorf("robots fetch: %w", err)
	}
	u.Path = "/robots.txt"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, "", fmt.Errorf("robots request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("robots fetch %s: %w", u.String(), err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("robots read: %w", err)
	}

	rt := ParseRobotsTxt(string(body))
	return rt, string(body), nil
}

// ParseRobotsTxt parses a robots.txt body string.
// It extracts rules for the default agent (*) and returns the disallowed paths.
func ParseRobotsTxt(body string) *RobotsTxt {
	rt := &RobotsTxt{}

	inDefaultAgent := false
	lines := strings.Split(body, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)

		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		lower := strings.ToLower(line)

		if strings.HasPrefix(lower, "user-agent:") {
			agent := strings.TrimSpace(line[11:])
			inDefaultAgent = agent == "*"
			continue
		}

		if !inDefaultAgent {
			continue
		}

		switch {
		case strings.HasPrefix(lower, "disallow:"):
			path := strings.TrimSpace(line[9:])
			if path != "" {
				rt.DisallowedPaths = append(rt.DisallowedPaths, path)
			}

		case strings.HasPrefix(lower, "allow:"):
			path := strings.TrimSpace(line[6:])
			if path != "" {
				rt.AllowedPaths = append(rt.AllowedPaths, path)
			}

		case strings.HasPrefix(lower, "crawl-delay:"):
			var secs float64
			if _, err := fmt.Sscanf(line[12:], "%f", &secs); err == nil {
				rt.CrawlDelay = time.Duration(secs * float64(time.Second))
			}

		case strings.HasPrefix(lower, "sitemap:"):
			url := strings.TrimSpace(line[8:])
			if url != "" {
				rt.Sitemaps = append(rt.Sitemaps, url)
			}
		}
	}

	return rt
}

// IsAllowed checks if a path is allowed by the robots.txt rules.
// A path is allowed unless it matches a Disallow rule that is not overridden
// by a more specific Allow rule.
func (r *RobotsTxt) IsAllowed(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return true // be permissive on parse failure
	}
	path := u.Path
	if path == "" {
		path = "/"
	}

	// Check Allow rules first (most specific wins)
	bestAllowLen := 0
	for _, allow := range r.AllowedPaths {
		if strings.HasPrefix(path, allow) && len(allow) > bestAllowLen {
			bestAllowLen = len(allow)
		}
	}

	// Check Disallow rules
	bestDisallowLen := 0
	for _, disallow := range r.DisallowedPaths {
		if strings.HasPrefix(path, disallow) && len(disallow) > bestDisallowLen {
			bestDisallowLen = len(disallow)
		}
	}

	// Most specific rule wins
	if bestAllowLen > bestDisallowLen {
		return true
	}
	if bestDisallowLen > 0 {
		return false
	}
	return true
}
