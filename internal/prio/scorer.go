package prio

import (
	"strings"

	"github.com/nasij/nasij/internal/authmapper"
	"github.com/nasij/nasij/internal/depintel"
	"github.com/nasij/nasij/internal/knowledge"
	"github.com/nasij/nasij/internal/runtime"
	"github.com/nasij/nasij/internal/secrets"
)

type Scorer struct {
	rules         []Rule
	knownEndpoint map[string]bool
}

func NewScorer() *Scorer {
	s := &Scorer{
		knownEndpoint: make(map[string]bool),
	}
	s.rules = []Rule{
		ruleAuth,
		ruleAdmin,
		ruleUpload,
		ruleGraphQL,
		ruleRedirect,
		ruleDangerSink,
		ruleSecrets,
		ruleCVE,
	}
	return s
}

func (s *Scorer) AddKnownEndpoint(url string) {
	s.knownEndpoint[url] = true
}

func (s *Scorer) SetKnownEndpoints(urls []string) {
	for _, u := range urls {
		s.knownEndpoint[u] = true
	}
}

func (s *Scorer) Score(items []Scorable) *Result {
	result := NewResult()

	for _, item := range items {
		breakdown := make(map[ScoringFactor]float64)
		var total float64

		for _, rule := range s.rules {
			scores := rule(item)
			for factor, score := range scores {
				if _, exists := breakdown[factor]; !exists {
					breakdown[factor] = score
					total += score
				}
			}
		}

		if s.knownEndpoint != nil && item.URL != "" {
			if !s.knownEndpoint[item.URL] {
				breakdown[FactorNewEP] = 0.5
				total += 0.5
			}
		}

		scored := ScoredItem{
			ID:        item.ID,
			Label:     item.Label,
			ItemType:  item.ItemType,
			Score:     total,
			Priority:  scoreToPriority(total),
			Breakdown: breakdown,
			Source:    item.Provider,
		}

		parts := make([]string, 0)
		for factor, score := range breakdown {
			if score > 0 {
				parts = append(parts, string(factor))
			}
		}
		if len(parts) > 0 {
			scored.Summary = strings.Join(parts, ", ")
		}

		result.AddItem(scored)
	}

	sortItems(result.Items)

	return result
}

func (s *Scorer) ScoreGraph(g *knowledge.Graph) *Result {
	items := make([]Scorable, 0)

	nodes := g.AllNodes()
	for _, n := range nodes {
		item := Scorable{
			ID:       n.ID,
			Label:    n.Label,
			ItemType: string(n.Type),
			Data:     n.Properties,
		}
		if u, ok := n.Properties["url"].(string); ok {
			item.URL = u
		}
		if m, ok := n.Properties["method"].(string); ok {
			item.Method = m
		}
		if name, ok := n.Properties["name"].(string); ok {
			item.Name = name
		}
		if st, ok := n.Properties["finding_type"].(string); ok {
			item.SecretType = st
		}
		if p, ok := n.Properties["provider"].(string); ok {
			item.Provider = p
		}

		items = append(items, item)
	}

	return s.Score(items)
}

func (s *Scorer) ScoreSecrets(sr *secrets.ScanResult) *Result {
	items := make([]Scorable, 0)
	for _, f := range sr.Findings {
		items = append(items, Scorable{
			ID:         string(f.SecretType) + ":" + f.Match,
			Label:      f.Key + ": " + f.Match,
			ItemType:   "secret",
			URL:        f.Source,
			SecretType: string(f.SecretType),
			Name:       f.Key,
			Provider:   f.Provider,
			Data: map[string]any{
				"severity": int(f.Severity),
				"entropy":  f.Entropy,
			},
		})
	}
	return s.Score(items)
}

func (s *Scorer) ScoreRuntime(rr *runtime.Result) *Result {
	items := make([]Scorable, 0)
	for _, req := range rr.Requests {
		items = append(items, Scorable{
			ID:       "api:" + req.Method + ":" + req.URL,
			Label:    req.Method + " " + req.URL,
			ItemType: "api_endpoint",
			URL:      req.URL,
			Method:   req.Method,
		})
	}
	for _, ws := range rr.WebSockets {
		items = append(items, Scorable{
			ID:       "ws:" + ws.URL,
			Label:    "WS " + ws.URL,
			ItemType: "websocket",
			URL:      ws.URL,
			Method:   "WS",
		})
	}
	return s.Score(items)
}

func (s *Scorer) ScoreAuth(am *authmapper.Mapping) *Result {
	items := make([]Scorable, 0)
	for _, ep := range am.Endpoints {
		items = append(items, Scorable{
			ID:       "auth:" + ep.EndpointType + ":" + ep.URL,
			Label:    ep.Method + " " + ep.URL + " (" + ep.EndpointType + ")",
			ItemType: "auth_endpoint",
			URL:      ep.URL,
			Method:   ep.Method,
			Name:     ep.EndpointType,
		})
	}
	for _, flow := range am.OAuthFlows {
		items = append(items, Scorable{
			ID:       "oauth:" + flow.Type + ":" + flow.AuthEndpoint,
			Label:    "OAuth " + flow.Type + " flow",
			ItemType: "auth_flow",
			URL:      flow.AuthEndpoint,
		})
	}
	for _, j := range am.JWTs {
		items = append(items, Scorable{
			ID:         "jwt:" + j.Token[:minInt(20, len(j.Token))],
			Label:      "JWT " + j.TokenType + " (" + j.Subject + ")",
			ItemType:   "jwt",
			SecretType: "jwt",
			Name:       j.TokenType,
		})
	}
	for _, c := range am.Cookies {
		items = append(items, Scorable{
			ID:       "cookie:" + c.Name + "@" + c.Domain,
			Label:    "Cookie " + c.Name + " @" + c.Domain,
			ItemType: "auth_cookie",
			URL:      c.Domain,
			Name:     c.Name,
		})
	}
	return s.Score(items)
}

func (s *Scorer) ScoreDeps(dr *depintel.DepResult) *Result {
	items := make([]Scorable, 0)
	for _, v := range dr.Vulnerabilities {
		cveScore := cveSeverityToFloat(v.Severity)
		items = append(items, Scorable{
			ID:       "cve:" + v.ID,
			Label:    v.ID + " — " + v.Summary,
			ItemType: "vulnerability",
			Name:     v.AffectedPackage,
			Data: map[string]any{
				"severity": cveScore,
			},
		})
	}
	return s.Score(items)
}

// Rules

func ruleAuth(item Scorable) map[ScoringFactor]float64 {
	if item.ItemType == "auth_endpoint" || item.ItemType == "auth_flow" ||
		item.ItemType == "auth_cookie" || item.ItemType == "jwt" {
		return map[ScoringFactor]float64{FactorAuth: 1.0}
	}
	if IsAuthEndpoint(item.URL, item.Method, item.Name) {
		score := 0.5
		if item.Method == "POST" {
			score = 0.8
		}
		if strings.Contains(item.Label, "token") {
			score = 1.0
		}
		return map[ScoringFactor]float64{FactorAuth: score}
	}
	if item.SecretType == "jwt" || item.SecretType == "oauth" {
		return map[ScoringFactor]float64{FactorAuth: 1.0}
	}
	return nil
}

func ruleAdmin(item Scorable) map[ScoringFactor]float64 {
	if IsAdminRoute(item.URL) {
		return map[ScoringFactor]float64{FactorAdmin: 1.0}
	}
	if item.ItemType == "auth_endpoint" && strings.Contains(item.Label, "admin") {
		return map[ScoringFactor]float64{FactorAdmin: 0.8}
	}
	return nil
}

func ruleUpload(item Scorable) map[ScoringFactor]float64 {
	if IsUploadEndpoint(item.URL) {
		return map[ScoringFactor]float64{FactorUpload: 1.0}
	}
	if item.ItemType == "api_endpoint" && strings.Contains(item.Label, "upload") {
		return map[ScoringFactor]float64{FactorUpload: 0.8}
	}
	return nil
}

func ruleGraphQL(item Scorable) map[ScoringFactor]float64 {
	if IsGraphQLEndpoint(item.URL) {
		return map[ScoringFactor]float64{FactorGraphQL: 1.0}
	}
	if strings.Contains(item.Label, "graphql") {
		return map[ScoringFactor]float64{FactorGraphQL: 0.8}
	}
	return nil
}

func ruleRedirect(item Scorable) map[ScoringFactor]float64 {
	if IsRedirectEndpoint(item.URL) {
		return map[ScoringFactor]float64{FactorRedirect: 1.0}
	}
	if item.ItemType == "auth_endpoint" && item.Name == "callback" {
		return map[ScoringFactor]float64{FactorRedirect: 0.8}
	}
	return nil
}

func ruleDangerSink(item Scorable) map[ScoringFactor]float64 {
	if IsDangerousSecret(item.SecretType) {
		return map[ScoringFactor]float64{FactorDangerSink: 1.0}
	}
	if item.ItemType == "vulnerability" {
		if sev, ok := item.Data["severity"]; ok {
			if sevFloat, ok := sev.(float64); ok && sevFloat >= 3.0 {
				return map[ScoringFactor]float64{FactorDangerSink: 0.8}
			}
		}
	}
	if item.Provider == "AWS" || item.Provider == "GitHub" || item.Provider == "Azure" {
		return map[ScoringFactor]float64{FactorDangerSink: 0.7}
	}
	return nil
}

func ruleSecrets(item Scorable) map[ScoringFactor]float64 {
	if item.ItemType == "secret" || item.ItemType == "finding" {
		score := 0.5
		if sev, ok := item.Data["severity"]; ok {
			if sevInt, ok := sev.(int); ok {
				score = float64(sevInt) / 4.0
			}
		}
		if item.Provider == "Critical" || item.SecretType == "private_key" {
			score = 1.0
		}
		return map[ScoringFactor]float64{FactorSecrets: score}
	}
	return nil
}

func ruleCVE(item Scorable) map[ScoringFactor]float64 {
	if item.ItemType == "vulnerability" {
		score := 0.5
		if sev, ok := item.Data["severity"]; ok {
			if sevFloat, ok := sev.(float64); ok {
				score = sevFloat / 4.0
			}
		}
		if score > 1.0 {
			score = 1.0
		}
		return map[ScoringFactor]float64{FactorCVE: score}
	}
	return nil
}

func cveSeverityToFloat(sev depintel.Severity) float64 {
	switch sev {
	case depintel.SeverityCritical:
		return 4.0
	case depintel.SeverityHigh:
		return 3.0
	case depintel.SeverityMedium:
		return 2.0
	case depintel.SeverityLow:
		return 1.0
	default:
		return 0.0
	}
}

func sortItems(items []ScoredItem) {
	for i := 0; i < len(items); i++ {
		for j := i + 1; j < len(items); j++ {
			if items[j].Score > items[i].Score {
				items[i], items[j] = items[j], items[i]
			}
		}
	}
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
