package scope

import (
	"fmt"
	"net"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// EntryType constants for scope rules.
const (
	TypeDomain    = "domain"
	TypeSubdomain = "subdomain"
	TypeCIDR      = "cidr"
	TypeExclude   = "exclude"
	TypeRegex     = "regex"
)

// ScopeEntry is a single scope rule stored in the database.
type ScopeEntry struct {
	ID          string    `json:"id"`
	WorkspaceID string    `json:"workspace_id"`
	EntryType   string    `json:"entry_type"`
	Pattern     string    `json:"pattern"`
	CreatedAt   time.Time `json:"created_at"`
}

// RateLimitConfig defines per-workspace rate limiting.
type RateLimitConfig struct {
	RequestsPerSec float64 `json:"requests_per_sec"`
	Burst          int     `json:"burst"`
}

// AuthConfig defines per-workspace authentication.
type AuthConfig struct {
	Type          string            `json:"type"` // header, cookie, basic, bearer
	Value         string            `json:"value"`
	CustomHeaders map[string]string `json:"custom_headers,omitempty"`
}

// Scope holds the complete scope definition for a workspace.
type Scope struct {
	WorkspaceID string
	TargetHost  string // extracted from workspace target URL, used as default domain

	// Parsed rules
	Domains    []string         // exact domain matches (e.g. "example.com")
	Subdomains []string         // specific subdomains (e.g. "admin.example.com")
	CIDRs      []*net.IPNet     // parsed CIDR ranges
	Excludes   []string         // excluded patterns (domain suffixes)
	Regexes    []*regexp.Regexp // compiled regex patterns

	// Raw patterns (for round-tripping back to DB)
	RawCIDRs []string
	RawRegex []string

	RateLimit     RateLimitConfig
	Auth          AuthConfig
	RespectRobots bool
}

// IsInScope determines whether the given URL is within scope.
//
// Rules:
//   - If no include rules exist, the URL must share a registered-domain
//     relationship with the target host.
//   - If include rules exist, the URL must match at least one include rule.
//   - Exclude rules are always checked: if the URL matches any exclude, return false.
func (s *Scope) IsInScope(rawURL string) bool {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	host := strings.ToLower(parsed.Hostname())
	path := parsed.Path

	// Always check excludes first
	for _, excl := range s.Excludes {
		if strings.HasPrefix(excl, "/") {
			// Path-based exclude
			if strings.HasPrefix(path, excl) {
				return false
			}
		} else {
			// Domain-based exclude
			if strings.EqualFold(host, excl) || strings.HasSuffix(host, "."+excl) {
				return false
			}
		}
	}

	hasIncludeRules := len(s.Domains) > 0 || len(s.Subdomains) > 0 || len(s.CIDRs) > 0 || len(s.Regexes) > 0

	if !hasIncludeRules && s.TargetHost != "" {
		return isSubdomainOrEqual(host, s.TargetHost)
	}

	for _, d := range s.Domains {
		if strings.EqualFold(host, d) || strings.HasSuffix(host, "."+d) {
			return true
		}
	}

	for _, sub := range s.Subdomains {
		if strings.EqualFold(host, sub) {
			return true
		}
	}

	for _, cidr := range s.CIDRs {
		ip := net.ParseIP(host)
		if ip == nil {
			addrs, _ := net.LookupHost(host)
			for _, addr := range addrs {
				if parsedIP := net.ParseIP(addr); parsedIP != nil && cidr.Contains(parsedIP) {
					return true
				}
			}
		} else if cidr.Contains(ip) {
			return true
		}
	}

	for _, re := range s.Regexes {
		if re.MatchString(rawURL) {
			return true
		}
	}

	return false
}

// isSubdomainOrEqual checks if child is equal to parent or a subdomain of parent.
func isSubdomainOrEqual(child, parent string) bool {
	child = strings.ToLower(child)
	parent = strings.ToLower(parent)
	return child == parent || strings.HasSuffix(child, "."+parent)
}

// CompileRegex patterns returns compiled regexps, skipping invalid ones.
func CompileRegex(patterns []string) []*regexp.Regexp {
	var res []*regexp.Regexp
	for _, p := range patterns {
		re, err := regexp.Compile(p)
		if err == nil {
			res = append(res, re)
		}
	}
	return res
}

// ParseCIDRs parses CIDR strings into *net.IPNet, skipping invalid ones.
func ParseCIDRs(cidrs []string) ([]*net.IPNet, []string) {
	var parsed []*net.IPNet
	var valid []string
	for _, c := range cidrs {
		_, ipnet, err := net.ParseCIDR(c)
		if err == nil {
			parsed = append(parsed, ipnet)
			valid = append(valid, c)
		}
	}
	return parsed, valid
}

// IsDomain checks if a string looks like a domain name.
func IsDomain(s string) bool {
	return strings.Contains(s, ".") && !strings.Contains(s, "/") && !strings.Contains(s, "*")
}

// IsWildcardDomain checks if a string is a wildcard domain pattern (e.g. *.example.com).
func IsWildcardDomain(s string) bool {
	return strings.HasPrefix(s, "*.") && IsDomain(s[2:])
}

// DomainFromWildcard strips the *. prefix from a wildcard domain.
func DomainFromWildcard(s string) string {
	return strings.TrimPrefix(s, "*.")
}

// NormalizeDomain strips protocol, trailing slashes, and extracts hostname.
func NormalizeDomain(raw string) string {
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, "http://") || strings.HasPrefix(raw, "https://") {
		u, err := url.Parse(raw)
		if err == nil {
			return strings.ToLower(u.Hostname())
		}
	}
	raw = strings.Split(raw, "/")[0]
	raw = strings.Split(raw, ":")[0]
	return strings.ToLower(raw)
}

// ExtractHost from a URL string.
func ExtractHost(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	return strings.ToLower(u.Hostname())
}

// ValidateCIDR checks if a string is a valid CIDR notation.
func ValidateCIDR(s string) error {
	_, _, err := net.ParseCIDR(s)
	if err != nil {
		return fmt.Errorf("invalid CIDR %q: %w", s, err)
	}
	return nil
}

// ValidateDomain checks if a string is a valid domain or wildcard domain.
func ValidateDomain(s string) error {
	if IsWildcardDomain(s) {
		s = s[2:]
	}
	if !strings.Contains(s, ".") {
		return fmt.Errorf("invalid domain %q: must contain at least one dot", s)
	}
	return nil
}

// ValidateRegex checks if a string is a valid regex pattern.
func ValidateRegex(s string) error {
	_, err := regexp.Compile(s)
	return err
}

// String returns a human-readable summary of the scope.
func (s *Scope) String() string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("Scope (workspace: %s)\n", s.WorkspaceID))
	if s.TargetHost != "" {
		b.WriteString(fmt.Sprintf("  Target:       %s\n", s.TargetHost))
	}
	b.WriteString(fmt.Sprintf("  Domains:      %d\n", len(s.Domains)))
	b.WriteString(fmt.Sprintf("  Subdomains:   %d\n", len(s.Subdomains)))
	b.WriteString(fmt.Sprintf("  CIDRs:        %d\n", len(s.CIDRs)))
	b.WriteString(fmt.Sprintf("  Excludes:     %d\n", len(s.Excludes)))
	b.WriteString(fmt.Sprintf("  Regexes:      %d\n", len(s.Regexes)))
	b.WriteString(fmt.Sprintf("  Rate Limit:   %.1f/s (burst %d)\n", s.RateLimit.RequestsPerSec, s.RateLimit.Burst))
	if s.Auth.Type != "" {
		b.WriteString(fmt.Sprintf("  Auth:         %s\n", s.Auth.Type))
	}
	return b.String()
}
