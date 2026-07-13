package prio

import (
	"fmt"
	"strings"
)

type Priority int

const (
	PriorityNone     Priority = 0
	PriorityLow      Priority = 1
	PriorityMedium   Priority = 2
	PriorityHigh     Priority = 3
	PriorityCritical Priority = 4
)

func (p Priority) String() string {
	switch p {
	case PriorityCritical:
		return "CRITICAL"
	case PriorityHigh:
		return "HIGH"
	case PriorityMedium:
		return "MEDIUM"
	case PriorityLow:
		return "LOW"
	default:
		return "NONE"
	}
}

type ScoringFactor string

const (
	FactorAuth       ScoringFactor = "authentication"
	FactorAdmin      ScoringFactor = "admin_routes"
	FactorUpload     ScoringFactor = "uploads"
	FactorGraphQL    ScoringFactor = "graphql"
	FactorRedirect   ScoringFactor = "redirects"
	FactorDangerSink ScoringFactor = "dangerous_sinks"
	FactorNewEP      ScoringFactor = "new_endpoints"
	FactorSecrets    ScoringFactor = "secrets"
	FactorCVE        ScoringFactor = "cve"
	FactorExposure   ScoringFactor = "exposure"
)

var AllFactors = []ScoringFactor{
	FactorAuth, FactorAdmin, FactorUpload, FactorGraphQL,
	FactorRedirect, FactorDangerSink, FactorNewEP,
	FactorSecrets, FactorCVE, FactorExposure,
}

type ScoredItem struct {
	ID        string                   `json:"id"`
	Label     string                   `json:"label"`
	ItemType  string                   `json:"item_type"`
	Score     float64                  `json:"score"`
	Priority  Priority                 `json:"priority"`
	Breakdown map[ScoringFactor]float64 `json:"breakdown"`
	Summary   string                   `json:"summary"`
	Source    string                   `json:"source"`
}

type Result struct {
	Items []ScoredItem `json:"items"`
	Count map[Priority]int `json:"count"`
}

type Rule func(item Scorable) map[ScoringFactor]float64

type Scorable struct {
	ID         string
	Label      string
	ItemType   string
	URL        string
	Method     string
	Name       string
	SecretType string
	Provider   string
	Severity   int
	Data       map[string]any
}

func NewResult() *Result {
	return &Result{
		Items: make([]ScoredItem, 0),
		Count: make(map[Priority]int),
	}
}

func (r *Result) AddItem(item ScoredItem) {
	r.Items = append(r.Items, item)
	r.Count[item.Priority]++
}

func (r *Result) ByPriority(p Priority) []ScoredItem {
	var result []ScoredItem
	for _, item := range r.Items {
		if item.Priority == p {
			result = append(result, item)
		}
	}
	return result
}

func (r *Result) TopN(n int) []ScoredItem {
	if n > len(r.Items) {
		n = len(r.Items)
	}
	return r.Items[:n]
}

func scoreToPriority(score float64) Priority {
	switch {
	case score >= 5.0:
		return PriorityCritical
	case score >= 3.0:
		return PriorityHigh
	case score >= 1.5:
		return PriorityMedium
	case score >= 0.5:
		return PriorityLow
	default:
		return PriorityNone
	}
}

func (s ScoredItem) String() string {
	return fmt.Sprintf("[%s] %.1f %s — %s", s.Priority, s.Score, s.ItemType, s.Label)
}

func ContainsAny(s string, terms ...string) bool {
	s = strings.ToLower(s)
	for _, t := range terms {
		if strings.Contains(s, strings.ToLower(t)) {
			return true
		}
	}
	return false
}

func IsAuthEndpoint(url, method, endpointType string) bool {
	u := strings.ToLower(url)
	m := strings.ToLower(method)
	if endpointType != "" {
		return true
	}
	if ContainsAny(u, "login", "signin", "signup", "register", "auth", "oauth",
		"token", "authorize", "callback", "logout", "refresh", "revoke") {
		return true
	}
	if m == "post" && (strings.HasSuffix(u, "/login") || strings.HasSuffix(u, "/token")) {
		return true
	}
	return false
}

func IsAdminRoute(url string) bool {
	return ContainsAny(url, "/admin", "/dashboard", "/manage", "/panel",
		"/backend", "/admin/", "wp-admin", "/api/admin")
}

func IsUploadEndpoint(url string) bool {
	return ContainsAny(url, "/upload", "/file", "/media", "/attachment",
		"/image", "/document", "/import", "/export")
}

func IsGraphQLEndpoint(url string) bool {
	return ContainsAny(url, "/graphql", "graphql?", "/gql", "/api/graphql")
}

func IsRedirectEndpoint(url string) bool {
	return ContainsAny(url, "/redirect", "/callback", "/return", "/next",
		"/goto", "/forward", "/proxy", "/logout") ||
		strings.Contains(strings.ToLower(url), "redirect_uri") ||
		strings.Contains(strings.ToLower(url), "?return") ||
		strings.Contains(strings.ToLower(url), "?next") ||
		strings.Contains(strings.ToLower(url), "?dest=") ||
		strings.Contains(strings.ToLower(url), "?url=")
}

func IsDangerousSecret(secretType string) bool {
	dangerous := []string{"aws", "gcp", "azure", "private_key", "password",
		"jwt", "oauth", "token", "connection_string"}
	for _, d := range dangerous {
		if strings.EqualFold(secretType, d) {
			return true
		}
	}
	return false
}

func IsNewEndpoint(url string, knownEndpoints []string) bool {
	for _, k := range knownEndpoints {
		if strings.EqualFold(url, k) {
			return false
		}
	}
	return true
}
