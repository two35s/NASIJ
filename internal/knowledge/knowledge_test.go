package knowledge

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/nasij/nasij/internal/authmapper"
	"github.com/nasij/nasij/internal/depintel"
	"github.com/nasij/nasij/internal/framework"
	"github.com/nasij/nasij/internal/runtime"
	"github.com/nasij/nasij/internal/secrets"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewGraphIsEmpty(t *testing.T) {
	g := New()
	assert.Empty(t, g.AllNodes())
	assert.Empty(t, g.AllEdges())
	stats := g.Stats()
	assert.Equal(t, 0, stats.TotalNodes)
	assert.Equal(t, 0, stats.TotalEdges)
}

func TestAddNode(t *testing.T) {
	g := New()
	n := g.AddNode(&Node{
		ID:    "page::https://example.com",
		Type:  NodePage,
		Label: "https://example.com",
	})
	require.NotNil(t, n)
	assert.Equal(t, "page::https://example.com", n.ID)
	assert.Equal(t, NodePage, n.Type)
	assert.NotNil(t, n.Properties)
}

func TestAddNodeDeduplicates(t *testing.T) {
	g := New()
	n1 := g.AddNode(&Node{ID: "a", Type: NodePage, Label: "page1"})
	n2 := g.AddNode(&Node{ID: "a", Type: NodeFinding, Label: "page2"})
	assert.Same(t, n1, n2)
	assert.Equal(t, NodePage, n2.Type)
	assert.Equal(t, 1, len(g.AllNodes()))
}

func TestGetNode(t *testing.T) {
	g := New()
	g.AddNode(&Node{ID: "a", Type: NodePage, Label: "pageA"})
	n := g.GetNode("a")
	require.NotNil(t, n)
	assert.Equal(t, "pageA", n.Label)
	assert.Nil(t, g.GetNode("nonexistent"))
}

func TestHasNode(t *testing.T) {
	g := New()
	g.AddNode(&Node{ID: "a", Type: NodePage})
	assert.True(t, g.HasNode("a"))
	assert.False(t, g.HasNode("b"))
}

func TestAddEdge(t *testing.T) {
	g := New()
	g.AddNode(&Node{ID: "a", Type: NodePage, Label: "src"})
	g.AddNode(&Node{ID: "b", Type: NodeAPIEndpoint, Label: "dst"})
	e := g.AddEdge(&Edge{Type: EdgeCalls, Source: "a", Target: "b"})
	require.NotNil(t, e)
	assert.Equal(t, EdgeCalls, e.Type)
	assert.Equal(t, "a", e.Source)
	assert.Equal(t, "b", e.Target)
	assert.NotEmpty(t, e.ID)
}

func TestAddEdgeAutoIDs(t *testing.T) {
	g := New()
	g.AddNode(&Node{ID: "a"})
	g.AddNode(&Node{ID: "b"})
	e := g.AddEdge(&Edge{Source: "a", Target: "b", Type: EdgeCalls})
	assert.Equal(t, "a->b:calls", e.ID)
}

func TestNodesByType(t *testing.T) {
	g := New()
	g.AddNode(&Node{ID: "p1", Type: NodePage})
	g.AddNode(&Node{ID: "p2", Type: NodePage})
	g.AddNode(&Node{ID: "api1", Type: NodeAPIEndpoint})
	g.AddNode(&Node{ID: "f1", Type: NodeFinding})
	pages := g.NodesByType(NodePage)
	assert.Len(t, pages, 2)
	apis := g.NodesByType(NodeAPIEndpoint)
	assert.Len(t, apis, 1)
	findings := g.NodesByType(NodeFinding)
	assert.Len(t, findings, 1)
	assert.Empty(t, g.NodesByType(NodeCookie))
}

func TestNeighbors(t *testing.T) {
	g := New()
	g.AddNode(&Node{ID: "a"})
	g.AddNode(&Node{ID: "b"})
	g.AddNode(&Node{ID: "c"})
	g.AddNode(&Node{ID: "d"})
	g.AddEdge(&Edge{Source: "a", Target: "b", Type: EdgeCalls})
	g.AddEdge(&Edge{Source: "a", Target: "c", Type: EdgeContains})
	g.AddEdge(&Edge{Source: "d", Target: "a", Type: EdgeHasFinding})

	neighbors := g.Neighbors("a")
	assert.Len(t, neighbors, 3)

	neighbors = g.Neighbors("a", EdgeCalls)
	require.Len(t, neighbors, 1)
	assert.Equal(t, "b", neighbors[0].ID)

	neighbors = g.Neighbors("x")
	assert.Empty(t, neighbors)
}

func TestEdgesBetween(t *testing.T) {
	g := New()
	g.AddNode(&Node{ID: "a"})
	g.AddNode(&Node{ID: "b"})
	g.AddEdge(&Edge{Source: "a", Target: "b", Type: EdgeCalls, Label: "req1"})
	g.AddEdge(&Edge{Source: "a", Target: "b", Type: EdgeStores, Label: "store1"})
	g.AddEdge(&Edge{Source: "a", Target: "c", Type: EdgeCalls})

	edges := g.EdgesBetween("a", "b")
	assert.Len(t, edges, 2)
	edges = g.EdgesBetween("a", "c")
	assert.Len(t, edges, 1)
	edges = g.EdgesBetween("x", "y")
	assert.Empty(t, edges)
}

func TestSearch(t *testing.T) {
	g := New()
	g.AddNode(&Node{ID: "page::https://example.com", Type: NodePage, Label: "https://example.com"})
	g.AddNode(&Node{ID: "api::GET::https://api.example.com/users", Type: NodeAPIEndpoint, Label: "GET https://api.example.com/users"})
	g.AddNode(&Node{ID: "framework::react", Type: NodeFramework, Label: "react v18.2.0"})
	g.AddNode(&Node{ID: "finding::jwt::abc123", Type: NodeFinding, Label: "[JWT] access token"})

	results := g.Search("example")
	assert.Len(t, results, 2)

	results = g.Search("react")
	assert.Len(t, results, 1)
	assert.Equal(t, "framework::react", results[0].ID)

	results = g.Search("nonexistent")
	assert.Empty(t, results)

	results = g.Search("")
	assert.Empty(t, results)
}

func TestSearchRanked(t *testing.T) {
	g := New()
	g.AddNode(&Node{ID: "page::https://example.com", Type: NodePage, Label: "My Example Page"})
	g.AddNode(&Node{ID: "api::GET::https://api.example.com", Type: NodeAPIEndpoint, Label: "GET /users"})

	results := g.SearchRanked("example")
	require.Len(t, results, 2)
	// Page label "My Example Page" scores more for "My" and "Page", but
	// both have "example" in label (+3) and ID (+2). They can tie.
	// Just verify both are present and scored.
	for _, r := range results {
		assert.Greater(t, r.Score, 0)
	}
}

func TestFindPath(t *testing.T) {
	g := New()
	for _, id := range []string{"a", "b", "c", "d", "e"} {
		g.AddNode(&Node{ID: id})
	}
	g.AddEdge(&Edge{Source: "a", Target: "b", Type: EdgeCalls})
	g.AddEdge(&Edge{Source: "b", Target: "c", Type: EdgeCalls})
	g.AddEdge(&Edge{Source: "b", Target: "d", Type: EdgeContains})
	g.AddEdge(&Edge{Source: "d", Target: "e", Type: EdgeCalls})

	paths := g.FindPath("a", "c")
	require.NotEmpty(t, paths)
	assert.Equal(t, []string{"a", "b", "c"}, paths[0])

	paths = g.FindPath("a", "e")
	require.NotEmpty(t, paths)
	assert.Equal(t, []string{"a", "b", "d", "e"}, paths[0])

	paths = g.FindPath("a", "a")
	require.Len(t, paths, 1)
	assert.Equal(t, []string{"a"}, paths[0])

	paths = g.FindPath("a", "x")
	assert.Empty(t, paths)
}

func TestStats(t *testing.T) {
	g := New()
	g.AddNode(&Node{ID: "p1", Type: NodePage})
	g.AddNode(&Node{ID: "p2", Type: NodePage})
	g.AddNode(&Node{ID: "f1", Type: NodeFinding})
	g.AddNode(&Node{ID: "api1", Type: NodeAPIEndpoint})
	g.AddEdge(&Edge{Source: "p1", Target: "api1", Type: EdgeCalls})
	g.AddEdge(&Edge{Source: "p1", Target: "f1", Type: EdgeHasFinding})

	stats := g.Stats()
	assert.Equal(t, 4, stats.TotalNodes)
	assert.Equal(t, 2, stats.TotalEdges)
	assert.Equal(t, 2, stats.NodesByType[NodePage])
	assert.Equal(t, 1, stats.NodesByType[NodeFinding])
	assert.Equal(t, 1, stats.NodesByType[NodeAPIEndpoint])
	assert.Equal(t, 1, stats.EdgesByType[EdgeCalls])
	assert.Equal(t, 1, stats.EdgesByType[EdgeHasFinding])
}

func TestBuilderEmpty(t *testing.T) {
	b := NewBuilder()
	g := b.Build()
	assert.NotNil(t, g)
	assert.Empty(t, g.AllNodes())
}

func TestBuilderRuntimeResult(t *testing.T) {
	b := NewBuilder()
	rr := &runtime.Result{
		URL: "https://example.com",
		Requests: []runtime.RequestRecord{
			{URL: "https://api.example.com/data", Method: "GET", ResourceType: "xhr", StatusCode: 200},
			{URL: "https://api.example.com/login", Method: "POST", ResourceType: "fetch", StatusCode: 401},
		},
		Cookies: []runtime.CookieRecord{
			{Name: "session", Domain: "example.com", Path: "/", Secure: true, HttpOnly: true},
		},
		LocalStorage: []runtime.StorageRecord{
			{Key: "token", Value: "eyJhbGciOiJIUzI1NiJ9"},
		},
		SessionStorage: []runtime.StorageRecord{
			{Key: "nonce", Value: "abc123"},
		},
		WebSockets: []runtime.WebSocketRecord{
			{URL: "wss://ws.example.com/socket"},
		},
	}
	b.AddRuntimeResult("https://example.com", rr)
	g := b.Graph()

	assert.True(t, g.HasNode("page::https://example.com"))
	assert.True(t, g.HasNode("api::GET::https://api.example.com/data"))
	assert.True(t, g.HasNode("api::POST::https://api.example.com/login"))
	assert.True(t, g.HasNode("api::WS::wss://ws.example.com/socket"))
	assert.True(t, g.HasNode("cookie::session::example.com"))
	assert.True(t, g.HasNode("storage::localStorage::token::https://example.com"))
	assert.True(t, g.HasNode("storage::sessionStorage::nonce::https://example.com"))

	stats := g.Stats()
	assert.Equal(t, 7, stats.TotalNodes)
}

func TestBuilderFrameworkResult(t *testing.T) {
	b := NewBuilder()
	fr := &framework.Result{
		Frameworks: []framework.Framework{
			{Name: "react", Version: "18.2.0", Confidence: 0.95},
			{Name: "next.js", Version: "14.0.0", Confidence: 0.80},
		},
	}
	b.AddFrameworkResult("https://example.com", fr)
	g := b.Graph()

	assert.True(t, g.HasNode("page::https://example.com"))
	assert.True(t, g.HasNode("framework::react"))
	assert.True(t, g.HasNode("framework::next.js"))

	stats := g.Stats()
	assert.Equal(t, 3, stats.TotalNodes)
	assert.Equal(t, 2, stats.TotalEdges)

	neighbors := g.Neighbors("page::https://example.com")
	assert.Len(t, neighbors, 2)
}

func TestBuilderAuthMapping(t *testing.T) {
	b := NewBuilder()
	am := &authmapper.Mapping{
		Endpoints: []authmapper.AuthEndpoint{
			{URL: "https://example.com/login", Method: "POST", EndpointType: "login"},
			{URL: "https://example.com/oauth/callback", Method: "GET", EndpointType: "callback"},
		},
		OAuthFlows: []authmapper.OAuthFlow{
			{
				Type:          "authorization_code",
				AuthEndpoint:  "https://example.com/authorize",
				TokenEndpoint: "https://example.com/token",
				PKCE:          true,
			},
		},
		JWTs: []authmapper.JWTInfo{
			{Token: "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0", Algorithm: "HS256", Subject: "user123", Source: "header"},
		},
		Cookies: []authmapper.AuthCookie{
			{Name: "auth_session", Domain: "example.com", TokenType: "session"},
		},
		StorageTokens: []authmapper.StorageToken{
			{Source: "localStorage", Key: "access_token", TokenType: "access"},
		},
	}
	b.AddAuthMapping("https://example.com", am)
	g := b.Graph()

	assert.True(t, g.HasNode("page::https://example.com"))
	assert.True(t, g.HasNode("auth_endpoint::login::https://example.com/login"))
	assert.True(t, g.HasNode("auth_endpoint::callback::https://example.com/oauth/callback"))
	assert.True(t, g.HasNode("api::POST::https://example.com/login"))
	assert.True(t, g.HasNode("api::GET::https://example.com/authorize"))
	assert.True(t, g.HasNode("api::POST::https://example.com/token"))

	foundJWT := false
	for _, n := range g.NodesByType(NodeFinding) {
		if n.Properties["finding_type"] == "jwt" {
			foundJWT = true
			break
		}
	}
	assert.True(t, foundJWT, "should find JWT node")

	assert.True(t, g.HasNode("cookie::auth_session::example.com"))
	assert.True(t, g.HasNode("storage::localStorage::access_token"))
}

func TestBuilderSecretsResult(t *testing.T) {
	b := NewBuilder()
	sr := &secrets.ScanResult{
		Target: "https://example.com",
		Findings: []secrets.Finding{
			{SecretType: secrets.TypeAWS, Provider: "AWS", Key: "AWS Access Key ID", Match: "AKIAIOSFODNN7EXAMPLE", Severity: secrets.SeverityHigh},
			{SecretType: secrets.TypeJWT, Provider: "JWT", Key: "JWT Token", Match: "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxIn0", Severity: secrets.SeverityCritical},
		},
	}
	b.AddSecretsResult(sr)
	g := b.Graph()

	page := g.GetNode("page::https://example.com")
	require.NotNil(t, page)

	findings := g.NodesByType(NodeFinding)
	assert.Len(t, findings, 2)
}

func TestBuilderDepResult(t *testing.T) {
	b := NewBuilder()
	dr := &depintel.DepResult{
		Target: "/test",
		Packages: []depintel.Package{
			{Name: "lodash", Version: "4.17.19", Ecosystem: depintel.EcosystemNPM, Source: "package.json"},
			{Name: "express", Version: "4.17.21", Ecosystem: depintel.EcosystemNPM, Source: "package.json"},
		},
		Vulnerabilities: []depintel.Vulnerability{
			{ID: "CVE-2020-8203", AffectedPackage: "lodash", AffectedVersion: "4.17.19", Severity: depintel.SeverityHigh, Summary: "Prototype pollution", FixedVersion: "4.17.20"},
		},
	}
	b.AddDepResult(dr)
	g := b.Graph()

	assert.True(t, g.HasNode("dep::npm::lodash"))
	assert.True(t, g.HasNode("dep::npm::express"))
	assert.True(t, g.HasNode("vuln::CVE-2020-8203"))

	neighbors := g.Neighbors("dep::npm::lodash")
	assert.NotEmpty(t, neighbors)

	stats := g.Stats()
	assert.Equal(t, 3, stats.TotalNodes)
	assert.Equal(t, 1, stats.TotalEdges)
}

func TestBuilderFullPipeline(t *testing.T) {
	b := NewBuilder()

	rr := &runtime.Result{
		URL: "https://example.com",
		Requests: []runtime.RequestRecord{
			{URL: "https://api.example.com/users", Method: "GET", ResourceType: "xhr"},
			{URL: "https://api.example.com/login", Method: "POST", ResourceType: "fetch"},
		},
		Cookies: []runtime.CookieRecord{
			{Name: "session", Domain: "example.com", Path: "/"},
		},
	}
	b.AddRuntimeResult("https://example.com", rr)

	fr := &framework.Result{
		Frameworks: []framework.Framework{
			{Name: "react", Version: "18.2.0", Confidence: 0.9},
			{Name: "express", Version: "4.18.0", Confidence: 0.7},
		},
	}
	b.AddFrameworkResult("https://example.com", fr)

	am := &authmapper.Mapping{
		Endpoints: []authmapper.AuthEndpoint{
			{URL: "https://example.com/login", Method: "POST", EndpointType: "login"},
		},
	}
	b.AddAuthMapping("https://example.com", am)

	sr := &secrets.ScanResult{
		Target: "https://example.com",
		Findings: []secrets.Finding{
			{SecretType: secrets.TypeAWS, Key: "AWS Key", Match: "AKIAIOSFODNN7EXAMPLE"},
		},
	}
	b.AddSecretsResult(sr)

	dr := &depintel.DepResult{
		Packages: []depintel.Package{
			{Name: "react", Version: "18.2.0", Ecosystem: depintel.EcosystemNPM},
		},
		Vulnerabilities: []depintel.Vulnerability{
			{ID: "CVE-2024-0001", AffectedPackage: "react", AffectedVersion: "18.2.0", Severity: depintel.SeverityHigh},
		},
	}
	b.AddDepResult(dr)

	g := b.Graph()

	stats := g.Stats()
	assert.Greater(t, stats.TotalNodes, 5)
	assert.Greater(t, stats.TotalEdges, 5)

	assert.True(t, g.HasNode("page::https://example.com"))
	assert.True(t, g.HasNode("api::GET::https://api.example.com/users"))
	assert.True(t, g.HasNode("api::POST::https://api.example.com/login"))
	assert.True(t, g.HasNode("framework::react"))
	assert.True(t, g.HasNode("auth_endpoint::login::https://example.com/login"))
	assert.True(t, g.HasNode("dep::npm::react"))
	assert.True(t, g.HasNode("vuln::CVE-2024-0001"))
}

func TestJSONExport(t *testing.T) {
	g := New()
	g.AddNode(&Node{ID: "page::https://example.com", Type: NodePage, Label: "https://example.com"})
	g.AddNode(&Node{ID: "api::GET::https://api.example.com", Type: NodeAPIEndpoint, Label: "GET https://api.example.com"})
	g.AddEdge(&Edge{Source: "page::https://example.com", Target: "api::GET::https://api.example.com", Type: EdgeCalls})

	data, err := g.ToJSON()
	require.NoError(t, err)

	var parsed struct {
		Nodes []Node `json:"nodes"`
		Edges []Edge `json:"edges"`
	}
	err = json.Unmarshal(data, &parsed)
	require.NoError(t, err)
	assert.Len(t, parsed.Nodes, 2)
	assert.Len(t, parsed.Edges, 1)
}

func TestDOTExport(t *testing.T) {
	g := New()
	g.AddNode(&Node{ID: "page::https://example.com", Type: NodePage, Label: "Homepage"})
	g.AddNode(&Node{ID: "api::GET::/users", Type: NodeAPIEndpoint, Label: "GET /users"})
	g.AddEdge(&Edge{Source: "page::https://example.com", Target: "api::GET::/users", Type: EdgeCalls, Label: "fetch"})

	dot := g.ToDOT()
	assert.Contains(t, dot, "digraph NASIJ")
	assert.Contains(t, dot, "page::https://example.com")
	assert.Contains(t, dot, "api::GET::/users")
	assert.Contains(t, dot, "calls")
}

func TestFindPathNoPath(t *testing.T) {
	g := New()
	g.AddNode(&Node{ID: "a"})
	g.AddNode(&Node{ID: "b"})
	paths := g.FindPath("a", "b")
	assert.Empty(t, paths)
}

func TestNeighborsWithEdgeFilter(t *testing.T) {
	g := New()
	g.AddNode(&Node{ID: "a"})
	g.AddNode(&Node{ID: "b"})
	g.AddNode(&Node{ID: "c"})
	g.AddEdge(&Edge{Source: "a", Target: "b", Type: EdgeCalls})
	g.AddEdge(&Edge{Source: "a", Target: "c", Type: EdgeStores})

	neighbors := g.Neighbors("a", EdgeCalls)
	assert.Len(t, neighbors, 1)
	assert.Equal(t, "b", neighbors[0].ID)

	neighbors = g.Neighbors("a", EdgeCalls, EdgeStores)
	assert.Len(t, neighbors, 2)
}

func TestBuilderPageDedup(t *testing.T) {
	b := NewBuilder()
	b.AddRuntimeResult("https://example.com", &runtime.Result{URL: "https://example.com"})
	b.AddFrameworkResult("https://example.com", &framework.Result{})
	b.AddAuthMapping("https://example.com", &authmapper.Mapping{})

	g := b.Graph()
	pages := g.NodesByType(NodePage)
	assert.Len(t, pages, 1)
}

func TestRuntimeEmpty(t *testing.T) {
	b := NewBuilder()
	rr := &runtime.Result{URL: "https://example.com"}
	b.AddRuntimeResult("https://example.com", rr)
	g := b.Graph()
	assert.True(t, g.HasNode("page::https://example.com"))
}

func TestRuntimeIndexedDB(t *testing.T) {
	b := NewBuilder()
	rr := &runtime.Result{
		URL: "https://example.com",
		IndexedDB: []runtime.IndexedDBRecord{
			{Database: "appCache", Version: 1, Stores: []runtime.IndexedDBStore{
				{Name: "sessions", Records: []any{map[string]any{"id": "abc"}}},
			}},
		},
	}
	b.AddRuntimeResult("https://example.com", rr)
	g := b.Graph()
	assert.True(t, g.HasNode("storage::indexeddb::appCache::https://example.com"))
}

func TestRuntimeServiceWorker(t *testing.T) {
	b := NewBuilder()
	rr := &runtime.Result{
		URL: "https://example.com",
		ServiceWorkers: []runtime.ServiceWorkerRecord{
			{URL: "https://example.com/sw.js", Scope: "/", Active: true},
		},
	}
	b.AddRuntimeResult("https://example.com", rr)
	g := b.Graph()
	assert.True(t, g.HasNode("sw::https://example.com/sw.js"))
}

func TestSecretsWithFileSource(t *testing.T) {
	b := NewBuilder()
	sr := &secrets.ScanResult{
		Target: "/path/to/file.env",
		Findings: []secrets.Finding{
			{SecretType: secrets.TypeAPIKey, Provider: "GitHub", Key: "GitHub PAT", Match: "ghp_abc123", Source: "file.env"},
		},
	}
	b.AddSecretsResult(sr)
	g := b.Graph()

	findings := g.NodesByType(NodeFinding)
	require.NotEmpty(t, findings)
	page := g.GetNode("page::/path/to/file.env")
	require.NotNil(t, page)
}

func TestSearchFindsByProperty(t *testing.T) {
	g := New()
	g.AddNode(&Node{
		ID:    "finding::secret::abc",
		Type:  NodeFinding,
		Label: "AWS Key",
		Properties: map[string]any{
			"match": "AKIAIOSFODNN7EXAMPLE",
		},
	})
	results := g.Search("AKIAIOSFODNN7EXAMPLE")
	assert.Len(t, results, 1)
}

func TestStatsEmpty(t *testing.T) {
	g := New()
	stats := g.Stats()
	assert.Equal(t, 0, stats.TotalNodes)
	assert.Equal(t, 0, stats.TotalEdges)
	assert.Empty(t, stats.NodesByType)
	assert.Empty(t, stats.EdgesByType)
}

func TestEdgePropertiesPreserved(t *testing.T) {
	g := New()
	g.AddNode(&Node{ID: "a"})
	g.AddNode(&Node{ID: "b"})
	e := g.AddEdge(&Edge{
		Source: "a", Target: "b", Type: EdgeCalls,
		Properties: map[string]any{"status_code": 200, "method": "GET"},
	})
	assert.Equal(t, 200, e.Properties["status_code"])
	assert.Equal(t, "GET", e.Properties["method"])
}

func TestNodeIDGeneration(t *testing.T) {
	assert.Equal(t, "page::https://example.com", nodeID("page", "https://example.com"))
	assert.Equal(t, "api::GET::https://example.com/api", nodeID("api", "GET", "https://example.com/api"))
	assert.Equal(t, "dep::npm::lodash", nodeID("dep", "npm", "lodash"))
}

func TestBuilderFrameworkConfidence(t *testing.T) {
	b := NewBuilder()
	fr := &framework.Result{
		Frameworks: []framework.Framework{
			{Name: "vue", Version: "3.3.0", Confidence: 0.85},
		},
	}
	b.AddFrameworkResult("https://example.com", fr)
	g := b.Graph()
	fwNode := g.GetNode("framework::vue")
	require.NotNil(t, fwNode)
	assert.Equal(t, 0.85, fwNode.Properties["confidence"])
	assert.Equal(t, "3.3.0", fwNode.Properties["version"])
}

func TestGraphAllNodesAndEdges(t *testing.T) {
	g := New()
	g.AddNode(&Node{ID: "a"})
	g.AddNode(&Node{ID: "b"})
	g.AddEdge(&Edge{Source: "a", Target: "b", Type: EdgeCalls})

	assert.Len(t, g.AllNodes(), 2)
	assert.Len(t, g.AllEdges(), 1)
}

func TestDOTOutputFormat(t *testing.T) {
	g := New()
	g.AddNode(&Node{ID: "x", Type: NodePage, Label: "Page X"})
	g.AddNode(&Node{ID: "y", Type: NodeAPIEndpoint, Label: "API Y"})
	g.AddEdge(&Edge{Source: "x", Target: "y", Type: EdgeCalls})

	dot := g.ToDOT()
	assert.True(t, strings.HasPrefix(dot, "digraph NASIJ"))
	assert.True(t, strings.Contains(dot, `"x"`))
	assert.True(t, strings.Contains(dot, `"y"`))
	assert.True(t, strings.Contains(dot, "calls"))
	assert.True(t, strings.Contains(dot, "->"))
}

func TestMultiplePaths(t *testing.T) {
	g := New()
	for _, id := range []string{"a", "b", "c", "d"} {
		g.AddNode(&Node{ID: id})
	}
	g.AddEdge(&Edge{Source: "a", Target: "b", Type: EdgeCalls})
	g.AddEdge(&Edge{Source: "a", Target: "c", Type: EdgeCalls})
	g.AddEdge(&Edge{Source: "b", Target: "d", Type: EdgeCalls})
	g.AddEdge(&Edge{Source: "c", Target: "d", Type: EdgeCalls})

	paths := g.FindPath("a", "d")
	require.NotEmpty(t, paths)
	for _, path := range paths {
		assert.Equal(t, "a", path[0])
		assert.Equal(t, "d", path[len(path)-1])
	}
}

func TestBuilderVulnLinkingToDep(t *testing.T) {
	b := NewBuilder()
	dr := &depintel.DepResult{
		Packages: []depintel.Package{
			{Name: "lodash", Version: "4.17.19", Ecosystem: depintel.EcosystemNPM},
			{Name: "express", Version: "4.17.21", Ecosystem: depintel.EcosystemNPM},
		},
		Vulnerabilities: []depintel.Vulnerability{
			{ID: "CVE-2020-8203", AffectedPackage: "lodash", AffectedVersion: "4.17.19", Severity: depintel.SeverityHigh},
		},
	}
	b.AddDepResult(dr)
	g := b.Graph()

	depNode := g.GetNode("dep::npm::lodash")
	require.NotNil(t, depNode)

	edges := g.EdgesBetween("dep::npm::lodash", "vuln::CVE-2020-8203")
	assert.Len(t, edges, 1)

	nonVulnEdges := g.EdgesBetween("dep::npm::express", "vuln::CVE-2020-8203")
	assert.Empty(t, nonVulnEdges)
}

func TestAuthFlowConnectsToAPI(t *testing.T) {
	b := NewBuilder()
	am := &authmapper.Mapping{
		OAuthFlows: []authmapper.OAuthFlow{
			{
				Type:          "authorization_code",
				AuthEndpoint:  "https://example.com/auth",
				TokenEndpoint: "https://example.com/token",
			},
		},
	}
	b.AddAuthMapping("https://example.com", am)
	g := b.Graph()

	assert.True(t, g.HasNode("api::GET::https://example.com/auth"))
	assert.True(t, g.HasNode("api::POST::https://example.com/token"))
}

func TestSanitizeURL(t *testing.T) {
	assert.Equal(t, "https://example.com", sanitizeURL("https://example.com/"))
	assert.Equal(t, "https://example.com/path", sanitizeURL("https://example.com/path"))
	assert.Equal(t, "", sanitizeURL(""))
}
