package prio

import (
	"testing"

	"github.com/nasij/nasij/internal/authmapper"
	"github.com/nasij/nasij/internal/depintel"
	"github.com/nasij/nasij/internal/knowledge"
	"github.com/nasij/nasij/internal/runtime"
	"github.com/nasij/nasij/internal/secrets"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPriorityString(t *testing.T) {
	assert.Equal(t, "CRITICAL", PriorityCritical.String())
	assert.Equal(t, "HIGH", PriorityHigh.String())
	assert.Equal(t, "MEDIUM", PriorityMedium.String())
	assert.Equal(t, "LOW", PriorityLow.String())
	assert.Equal(t, "NONE", PriorityNone.String())
}

func TestScoreToPriority(t *testing.T) {
	assert.Equal(t, PriorityCritical, scoreToPriority(5.5))
	assert.Equal(t, PriorityCritical, scoreToPriority(5.0))
	assert.Equal(t, PriorityHigh, scoreToPriority(3.0))
	assert.Equal(t, PriorityHigh, scoreToPriority(4.9))
	assert.Equal(t, PriorityMedium, scoreToPriority(1.5))
	assert.Equal(t, PriorityMedium, scoreToPriority(2.9))
	assert.Equal(t, PriorityLow, scoreToPriority(0.5))
	assert.Equal(t, PriorityNone, scoreToPriority(0.0))
}

func TestIsAuthEndpoint(t *testing.T) {
	assert.True(t, IsAuthEndpoint("https://example.com/login", "POST", ""))
	assert.True(t, IsAuthEndpoint("https://example.com/oauth/callback", "GET", ""))
	assert.True(t, IsAuthEndpoint("https://example.com/token", "POST", ""))
	assert.False(t, IsAuthEndpoint("https://example.com/api/users", "GET", ""))
	assert.True(t, IsAuthEndpoint("https://example.com/api/users", "GET", "login"))
}

func TestIsAdminRoute(t *testing.T) {
	assert.True(t, IsAdminRoute("https://example.com/admin"))
	assert.True(t, IsAdminRoute("https://example.com/admin/users"))
	assert.True(t, IsAdminRoute("https://example.com/wp-admin"))
	assert.True(t, IsAdminRoute("https://example.com/dashboard"))
	assert.False(t, IsAdminRoute("https://example.com/api/users"))
}

func TestIsUploadEndpoint(t *testing.T) {
	assert.True(t, IsUploadEndpoint("https://example.com/upload"))
	assert.True(t, IsUploadEndpoint("https://example.com/api/upload"))
	assert.True(t, IsUploadEndpoint("https://example.com/import"))
	assert.False(t, IsUploadEndpoint("https://example.com/api/users"))
}

func TestIsGraphQLEndpoint(t *testing.T) {
	assert.True(t, IsGraphQLEndpoint("https://example.com/graphql"))
	assert.True(t, IsGraphQLEndpoint("https://example.com/api/graphql"))
	assert.False(t, IsGraphQLEndpoint("https://example.com/api/users"))
}

func TestIsRedirectEndpoint(t *testing.T) {
	assert.True(t, IsRedirectEndpoint("https://example.com/redirect"))
	assert.True(t, IsRedirectEndpoint("https://example.com/callback"))
	assert.True(t, IsRedirectEndpoint("https://example.com?redirect_uri=http://evil.com"))
	assert.False(t, IsRedirectEndpoint("https://example.com/api/users"))
}

func TestIsDangerousSecret(t *testing.T) {
	assert.True(t, IsDangerousSecret("aws"))
	assert.True(t, IsDangerousSecret("private_key"))
	assert.True(t, IsDangerousSecret("jwt"))
	assert.False(t, IsDangerousSecret("api_key"))
}

func TestNewScorer(t *testing.T) {
	s := NewScorer()
	assert.NotNil(t, s)
	assert.Len(t, s.rules, 8)
}

func TestScorerScoreSecretItem(t *testing.T) {
	s := NewScorer()
	items := []Scorable{
		{
			ID:         "secret:aws:AKIA",
			Label:      "AWS Access Key: AKIAIOSFODNN7EXAMPLE",
			ItemType:   "secret",
			SecretType: "aws",
			Provider:   "AWS",
			Data:       map[string]any{"severity": 3},
		},
	}
	result := s.Score(items)
	require.Len(t, result.Items, 1)
	item := result.Items[0]
	assert.GreaterOrEqual(t, item.Score, 1.0)
	assert.Contains(t, item.Summary, "dangerous_sinks")
	assert.Contains(t, item.Summary, "secrets")
}

func TestScorerScoreAuthEndpoint(t *testing.T) {
	s := NewScorer()
	items := []Scorable{
		{
			ID:       "api:POST:/login",
			Label:    "POST /login",
			ItemType: "api_endpoint",
			URL:      "https://example.com/login",
			Method:   "POST",
		},
	}
	result := s.Score(items)
	require.Len(t, result.Items, 1)
	item := result.Items[0]
	assert.GreaterOrEqual(t, item.Score, 0.8)
	assert.Contains(t, item.Summary, "authentication")
}

func TestScorerScoreAdminRoute(t *testing.T) {
	s := NewScorer()
	items := []Scorable{
		{
			ID:       "api:GET:/admin/users",
			Label:    "GET /admin/users",
			ItemType: "api_endpoint",
			URL:      "https://example.com/admin/users",
		},
	}
	result := s.Score(items)
	require.Len(t, result.Items, 1)
	item := result.Items[0]
	assert.GreaterOrEqual(t, item.Score, 1.0)
	assert.Contains(t, item.Summary, "admin_routes")
}

func TestScorerScoreUpload(t *testing.T) {
	s := NewScorer()
	items := []Scorable{
		{
			ID:       "api:POST:/upload",
			Label:    "POST /upload",
			ItemType: "api_endpoint",
			URL:      "https://example.com/upload",
		},
	}
	result := s.Score(items)
	require.Len(t, result.Items, 1)
	assert.GreaterOrEqual(t, result.Items[0].Score, 1.0)
}

func TestScorerScoreGraphQL(t *testing.T) {
	s := NewScorer()
	items := []Scorable{
		{
			ID:       "api:POST:/graphql",
			Label:    "POST /graphql",
			ItemType: "api_endpoint",
			URL:      "https://example.com/graphql",
		},
	}
	result := s.Score(items)
	require.Len(t, result.Items, 1)
	assert.GreaterOrEqual(t, result.Items[0].Score, 1.0)
}

func TestScorerScoreRedirect(t *testing.T) {
	s := NewScorer()
	items := []Scorable{
		{
			ID:       "api:GET:/callback",
			Label:    "GET /callback",
			ItemType: "api_endpoint",
			URL:      "https://example.com/callback",
		},
	}
	result := s.Score(items)
	require.Len(t, result.Items, 1)
	assert.GreaterOrEqual(t, result.Items[0].Score, 1.0)
}

func TestScorerScoreJWT(t *testing.T) {
	s := NewScorer()
	items := []Scorable{
		{
			ID:         "jwt:eyJhbGci",
			Label:      "JWT access token (user123)",
			ItemType:   "jwt",
			SecretType: "jwt",
		},
	}
	result := s.Score(items)
	require.Len(t, result.Items, 1)
	// JWT items score: auth(1.0) + danger_sink(1.0) = 2.0+
	assert.GreaterOrEqual(t, result.Items[0].Score, 1.5)
	assert.Equal(t, "MEDIUM", result.Items[0].Priority.String())
}

func TestScorerScoreMultiFactor(t *testing.T) {
	s := NewScorer()
	items := []Scorable{
		{
			ID:       "api:POST:/admin/upload",
			Label:    "POST /admin/upload",
			ItemType: "api_endpoint",
			URL:      "https://example.com/admin/upload",
			Method:   "POST",
		},
	}
	result := s.Score(items)
	require.Len(t, result.Items, 1)
	item := result.Items[0]
	// admin_routes(1.0) + uploads(1.0) = 2.0+
	assert.GreaterOrEqual(t, item.Score, 2.0)
}

func TestScorerScoreEmpty(t *testing.T) {
	s := NewScorer()
	result := s.Score(nil)
	assert.Empty(t, result.Items)
}

func TestScorerByPriority(t *testing.T) {
	s := NewScorer()
	items := []Scorable{
		{ID: "low", Label: "low", ItemType: "page", URL: "https://example.com"},
		{ID: "high", Label: "JWT token", ItemType: "jwt", SecretType: "jwt"},
	}
	result := s.Score(items)
	medItems := result.ByPriority(PriorityMedium)
	assert.NotEmpty(t, medItems)
}

func TestScorerTopN(t *testing.T) {
	s := NewScorer()
	items := []Scorable{
		{ID: "1", Label: "item1", ItemType: "page", URL: "https://example.com"},
		{ID: "2", Label: "JWT token", ItemType: "jwt", SecretType: "jwt"},
		{ID: "3", Label: "POST /admin", ItemType: "api_endpoint", URL: "https://example.com/admin", Method: "POST"},
	}
	result := s.Score(items)
	top2 := result.TopN(2)
	assert.Len(t, top2, 2)
	assert.GreaterOrEqual(t, top2[0].Score, top2[1].Score)
}

func TestScorerScoreRuntime(t *testing.T) {
	s := NewScorer()
	rr := &runtime.Result{
		Requests: []runtime.RequestRecord{
			{URL: "https://example.com/login", Method: "POST"},
			{URL: "https://example.com/graphql", Method: "POST"},
			{URL: "https://example.com/api/users", Method: "GET"},
		},
		WebSockets: []runtime.WebSocketRecord{
			{URL: "wss://ws.example.com/socket"},
		},
	}
	result := s.ScoreRuntime(rr)
	require.Len(t, result.Items, 4)

	// Login should be auth-scored
	login := result.TopN(4)
	require.GreaterOrEqual(t, len(login), 3)
}

func TestScorerScoreSecrets(t *testing.T) {
	s := NewScorer()
	sr := &secrets.ScanResult{
		Findings: []secrets.Finding{
			{SecretType: secrets.TypeAWS, Provider: "AWS", Key: "AWS Key", Match: "AKIAIOSFODNN7EXAMPLE", Severity: secrets.SeverityHigh},
			{SecretType: secrets.TypeAPIKey, Provider: "Generic", Key: "API Key", Match: "sk_test_abc123", Severity: secrets.SeverityMedium},
		},
	}
	result := s.ScoreSecrets(sr)
	require.Len(t, result.Items, 2)
	// AWS item should have danger_sinks in breakdown
	awsHasDanger := false
	for _, item := range result.Items {
		if _, ok := item.Breakdown[FactorDangerSink]; ok {
			awsHasDanger = true
			break
		}
	}
	assert.True(t, awsHasDanger)
}

func TestScorerScoreAuth(t *testing.T) {
	s := NewScorer()
	am := &authmapper.Mapping{
		Endpoints: []authmapper.AuthEndpoint{
			{URL: "https://example.com/login", Method: "POST", EndpointType: "login"},
			{URL: "https://example.com/admin", Method: "GET", EndpointType: ""},
		},
		OAuthFlows: []authmapper.OAuthFlow{
			{Type: "authorization_code", AuthEndpoint: "https://example.com/authorize"},
		},
		JWTs: []authmapper.JWTInfo{
			{Token: "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjMifQ", TokenType: "access", Subject: "user1"},
		},
		Cookies: []authmapper.AuthCookie{
			{Name: "sessionid", Domain: "example.com", TokenType: "session"},
		},
	}
	result := s.ScoreAuth(am)
	assert.GreaterOrEqual(t, len(result.Items), 4)
}

func TestScorerScoreDeps(t *testing.T) {
	s := NewScorer()
	dr := &depintel.DepResult{
		Vulnerabilities: []depintel.Vulnerability{
			{
				ID: "CVE-2020-8203", AffectedPackage: "lodash", AffectedVersion: "4.17.19",
				Severity: depintel.SeverityHigh, Summary: "Prototype pollution",
			},
			{
				ID: "CVE-2024-0001", AffectedPackage: "express", AffectedVersion: "4.17.2",
				Severity: depintel.SeverityCritical, Summary: "RCE",
			},
		},
	}
	result := s.ScoreDeps(dr)
	require.Len(t, result.Items, 2)
	// Critical CVE should score higher
	assert.GreaterOrEqual(t, result.Items[0].Score, result.Items[1].Score)
}

func TestScorerScoreGraph(t *testing.T) {
	s := NewScorer()
	g := knowledge.New()
	g.AddNode(&knowledge.Node{
		ID: "api::POST::https://example.com/login", Type: knowledge.NodeAPIEndpoint,
		Label: "POST https://example.com/login",
		Properties: map[string]any{"url": "https://example.com/login", "method": "POST"},
	})
	g.AddNode(&knowledge.Node{
		ID: "api::GET::https://example.com/admin/users", Type: knowledge.NodeAPIEndpoint,
		Label: "GET https://example.com/admin/users",
		Properties: map[string]any{"url": "https://example.com/admin/users", "method": "GET"},
	})
	g.AddNode(&knowledge.Node{
		ID: "page::https://example.com", Type: knowledge.NodePage,
		Label: "https://example.com",
	})

	result := s.ScoreGraph(g)
	assert.GreaterOrEqual(t, len(result.Items), 3)

	// Login should be top priority
	top := result.TopN(1)
	require.NotEmpty(t, top)
}

func TestScorerNewEndpointDetection(t *testing.T) {
	s := NewScorer()
	s.AddKnownEndpoint("https://example.com/login")

	items := []Scorable{
		{
			ID:       "api:POST:/login",
			Label:    "POST /login",
			ItemType: "api_endpoint",
			URL:      "https://example.com/login",
		},
		{
			ID:       "api:POST:/unknown",
			Label:    "POST /unknown",
			ItemType: "api_endpoint",
			URL:      "https://example.com/unknown",
		},
	}
	result := s.Score(items)
	require.Len(t, result.Items, 2)
	// Unknown endpoint should have new_endpoints factor
	foundNew := false
	for _, item := range result.Items {
		if _, ok := item.Breakdown[FactorNewEP]; ok {
			foundNew = true
			break
		}
	}
	assert.True(t, foundNew)
}

func TestContainsAny(t *testing.T) {
	assert.True(t, ContainsAny("hello world", "world"))
	assert.False(t, ContainsAny("hello world", "xyz"))
	assert.True(t, ContainsAny("LOGIN", "login"))
}

func TestResultEmpty(t *testing.T) {
	r := NewResult()
	assert.Empty(t, r.Items)
	assert.Empty(t, r.Count)
}

func TestResultAddItem(t *testing.T) {
	r := NewResult()
	r.AddItem(ScoredItem{ID: "test", Priority: PriorityHigh})
	assert.Len(t, r.Items, 1)
	assert.Equal(t, 1, r.Count[PriorityHigh])
}

func TestScorerCVERule(t *testing.T) {
	s := NewScorer()
	items := []Scorable{
		{
			ID:       "cve:CVE-2024-0001",
			Label:    "CVE-2024-0001 — Critical RCE in express",
			ItemType: "vulnerability",
			Data:     map[string]any{"severity": 4.0},
		},
	}
	result := s.Score(items)
	require.Len(t, result.Items, 1)
	item := result.Items[0]
	assert.Contains(t, item.Summary, "cve")
}

func TestScorerGraphQLInLabel(t *testing.T) {
	s := NewScorer()
	items := []Scorable{
		{
			ID:       "api:POST:/api/gql",
			Label:    "POST /api/gql with graphql query",
			ItemType: "api_endpoint",
		},
	}
	result := s.Score(items)
	require.Len(t, result.Items, 1)
	assert.Contains(t, result.Items[0].Summary, "graphql")
}

func TestScorerBreakdownNotShared(t *testing.T) {
	s := NewScorer()
	items := []Scorable{
		{ID: "a", Label: "item a", ItemType: "api_endpoint", URL: "https://example.com/login"},
		{ID: "b", Label: "item b", ItemType: "api_endpoint", URL: "https://example.com/page"},
	}
	result := s.Score(items)
	require.Len(t, result.Items, 2)
	authCount := 0
	for _, item := range result.Items {
		if _, ok := item.Breakdown[FactorAuth]; ok {
			authCount++
		}
	}
	assert.Equal(t, 1, authCount)
}

func TestScorableString(t *testing.T) {
	item := ScoredItem{
		ID: "test", Priority: PriorityHigh, Score: 3.5,
		ItemType: "api", Label: "POST /login",
	}
	s := item.String()
	assert.Contains(t, s, "HIGH")
	assert.Contains(t, s, "3.5")
	assert.Contains(t, s, "POST /login")
}

func TestCveSeverityToFloat(t *testing.T) {
	assert.Equal(t, 4.0, cveSeverityToFloat(depintel.SeverityCritical))
	assert.Equal(t, 3.0, cveSeverityToFloat(depintel.SeverityHigh))
	assert.Equal(t, 2.0, cveSeverityToFloat(depintel.SeverityMedium))
	assert.Equal(t, 1.0, cveSeverityToFloat(depintel.SeverityLow))
	assert.Equal(t, 0.0, cveSeverityToFloat(depintel.SeverityUnknown))
}

func TestScorerSetKnownEndpoints(t *testing.T) {
	s := NewScorer()
	s.SetKnownEndpoints([]string{"https://example.com/login", "https://example.com/home"})
	assert.True(t, s.knownEndpoint["https://example.com/login"])
	assert.False(t, s.knownEndpoint["https://example.com/unknown"])
}

func TestAllFactorsList(t *testing.T) {
	assert.Contains(t, AllFactors, FactorAuth)
	assert.Contains(t, AllFactors, FactorAdmin)
	assert.Contains(t, AllFactors, FactorUpload)
	assert.Contains(t, AllFactors, FactorGraphQL)
	assert.Contains(t, AllFactors, FactorRedirect)
	assert.Contains(t, AllFactors, FactorDangerSink)
	assert.Contains(t, AllFactors, FactorNewEP)
	assert.Len(t, AllFactors, 10)
}

func TestIsNewEndpoint(t *testing.T) {
	assert.True(t, IsNewEndpoint("https://example.com/new", []string{"https://example.com/old"}))
	assert.False(t, IsNewEndpoint("https://example.com/old", []string{"https://example.com/old"}))
}

func TestScorerSortByScore(t *testing.T) {
	s := NewScorer()
	items := []Scorable{
		{ID: "low", Label: "page", ItemType: "page", URL: "https://example.com"},
		{ID: "high", Label: "JWT token", ItemType: "jwt", SecretType: "jwt"},
		{ID: "mid", Label: "POST /admin/upload", ItemType: "api_endpoint", URL: "https://example.com/admin/upload", Method: "POST"},
	}
	result := s.Score(items)
	require.Len(t, result.Items, 3)
	for i := 1; i < len(result.Items); i++ {
		assert.GreaterOrEqual(t, result.Items[i-1].Score, result.Items[i].Score,
			"items should be sorted by score descending")
	}
}
