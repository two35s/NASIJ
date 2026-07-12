package scope_test

import (
	"context"
	"net"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nasij/nasij/internal/scope"
	"github.com/nasij/nasij/internal/storage"
)

func openTestDB(t *testing.T) *storage.DB {
	t.Helper()
	ctx := context.Background()
	dir := t.TempDir()
	db, err := storage.Open(ctx, filepath.Join(dir, "test.db"))
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	return db
}

func TestScope_IsInScope_DefaultTarget(t *testing.T) {
	s := &scope.Scope{
		TargetHost: "example.com",
	}

	assert.True(t, s.IsInScope("https://example.com/page"))
	assert.True(t, s.IsInScope("https://sub.example.com/page"))
	assert.False(t, s.IsInScope("https://other.com/page"))
}

func TestScope_IsInScope_DomainInclude(t *testing.T) {
	s := &scope.Scope{
		Domains: []string{"example.com"},
	}

	assert.True(t, s.IsInScope("https://example.com"))
	assert.True(t, s.IsInScope("https://www.example.com"))
	assert.True(t, s.IsInScope("https://deep.sub.example.com"))
	assert.False(t, s.IsInScope("https://example.org"))
}

func TestScope_IsInScope_SubdomainExact(t *testing.T) {
	s := &scope.Scope{
		Subdomains: []string{"admin.example.com"},
	}

	assert.True(t, s.IsInScope("https://admin.example.com"))
	assert.False(t, s.IsInScope("https://example.com"))
	assert.False(t, s.IsInScope("https://sub.admin.example.com"))
}

func TestScope_IsInScope_ExcludeOverridesInclude(t *testing.T) {
	s := &scope.Scope{
		Domains:  []string{"example.com"},
		Excludes: []string{"internal.example.com"},
	}

	assert.True(t, s.IsInScope("https://www.example.com"))
	assert.False(t, s.IsInScope("https://internal.example.com"))
}

func TestScope_IsInScope_PathExclude(t *testing.T) {
	s := &scope.Scope{
		Domains:  []string{"example.com"},
		Excludes: []string{"/logout"},
	}

	assert.True(t, s.IsInScope("https://example.com/home"))
	assert.False(t, s.IsInScope("https://example.com/logout"))
	assert.False(t, s.IsInScope("https://example.com/logout/all"))
}

func TestScope_IsInScope_NoRulesNoTarget(t *testing.T) {
	s := &scope.Scope{}
	assert.False(t, s.IsInScope("https://example.com"))
}

func TestScope_IsInScope_CIDR(t *testing.T) {
	_, cidr, _ := net.ParseCIDR("10.0.0.0/8")
	s := &scope.Scope{
		CIDRs: []*net.IPNet{cidr},
	}

	assert.False(t, s.IsInScope("https://example.com"))
}

func TestScope_IsInScope_Regex(t *testing.T) {
	s := &scope.Scope{
		TargetHost: "example.com",
		Regexes:    scope.CompileRegex([]string{`.*internal.*`}),
	}

	assert.True(t, s.IsInScope("https://internal.example.com"))
	assert.False(t, s.IsInScope("https://external.example.com"))
}

func TestScope_ExtractHost(t *testing.T) {
	assert.Equal(t, "example.com", scope.ExtractHost("https://example.com/path"))
	assert.Equal(t, "example.com", scope.ExtractHost("http://example.com:8080"))
	assert.Equal(t, "", scope.ExtractHost(""))
}

func TestScope_NormalizeDomain(t *testing.T) {
	assert.Equal(t, "example.com", scope.NormalizeDomain("https://example.com"))
	assert.Equal(t, "example.com", scope.NormalizeDomain("example.com"))
	assert.Equal(t, "example.com", scope.NormalizeDomain("EXAMPLE.COM"))
}

func TestScope_ValidateDomain(t *testing.T) {
	assert.NoError(t, scope.ValidateDomain("example.com"))
	assert.NoError(t, scope.ValidateDomain("*.example.com"))
	assert.Error(t, scope.ValidateDomain("notadomain"))
}

func TestScope_ValidateCIDR(t *testing.T) {
	assert.NoError(t, scope.ValidateCIDR("10.0.0.0/8"))
	assert.NoError(t, scope.ValidateCIDR("192.168.1.0/24"))
	assert.Error(t, scope.ValidateCIDR("not-a-cidr"))
	assert.Error(t, scope.ValidateCIDR("10.0.0.0/33"))
}

func TestScope_ValidateRegex(t *testing.T) {
	assert.NoError(t, scope.ValidateRegex(`.*\.example\.com`))
	assert.Error(t, scope.ValidateRegex(`[invalid`))
}

func TestScope_IsDomain(t *testing.T) {
	assert.True(t, scope.IsDomain("example.com"))
	assert.False(t, scope.IsDomain("notadomain"))
	assert.False(t, scope.IsDomain("*.example.com"))
}

func TestScope_IsWildcardDomain(t *testing.T) {
	assert.True(t, scope.IsWildcardDomain("*.example.com"))
	assert.False(t, scope.IsWildcardDomain("example.com"))
}

func TestScope_DomainFromWildcard(t *testing.T) {
	assert.Equal(t, "example.com", scope.DomainFromWildcard("*.example.com"))
}

func TestScope_CompileRegex_Valid(t *testing.T) {
	res := scope.CompileRegex([]string{`^https://`, `valid`})
	assert.Len(t, res, 2)
}

func TestScope_CompileRegex_SkipsInvalid(t *testing.T) {
	res := scope.CompileRegex([]string{`valid`, `[invalid`})
	assert.Len(t, res, 1)
}

func TestScope_ParseCIDRs(t *testing.T) {
	parsed, valid := scope.ParseCIDRs([]string{"10.0.0.0/8", "invalid"})
	assert.Len(t, parsed, 1)
	assert.Len(t, valid, 1)
}

func TestScope_String(t *testing.T) {
	s := &scope.Scope{
		WorkspaceID: "test-id",
		TargetHost:  "example.com",
		Domains:     []string{"example.com"},
		RateLimit:   scope.RateLimitConfig{RequestsPerSec: 5, Burst: 2},
	}
	str := s.String()
	assert.Contains(t, str, "test-id")
	assert.Contains(t, str, "example.com")
	assert.Contains(t, str, "5.0")

}

func TestScope_IsInScope_CaseInsensitiveDomain(t *testing.T) {
	s := &scope.Scope{
		Domains: []string{"Example.COM"},
	}

	assert.True(t, s.IsInScope("https://example.com"))
	assert.True(t, s.IsInScope("https://EXAMPLE.COM"))
	assert.False(t, s.IsInScope("https://notexample.com"))
}

func TestScope_IsInScope_MixedIncludeTypes(t *testing.T) {
	s := &scope.Scope{
		Domains:    []string{"example.com"},
		Subdomains: []string{"special.example.org"},
	}

	assert.True(t, s.IsInScope("https://example.com"))
	assert.True(t, s.IsInScope("https://special.example.org"))
	assert.False(t, s.IsInScope("https://example.org"))
}

func TestScope_IsInScope_ExcludePriority(t *testing.T) {
	s := &scope.Scope{
		Domains:  []string{"example.com"},
		Excludes: []string{"example.com"},
	}

	assert.False(t, s.IsInScope("https://example.com"))
}

// --- Manager tests ---

func TestManager_AddAndListEntries(t *testing.T) {
	db := openTestDB(t)
	m := scope.NewManager(db.DB())
	ctx := context.Background()
	wsID := "test-ws"

	require.NoError(t, m.AddRule(ctx, wsID, "domain", "example.com"))
	require.NoError(t, m.AddRule(ctx, wsID, "subdomain", "admin.example.com"))
	require.NoError(t, m.AddRule(ctx, wsID, "cidr", "10.0.0.0/8"))
	require.NoError(t, m.AddRule(ctx, wsID, "exclude", "/admin"))
	require.NoError(t, m.AddRule(ctx, wsID, "regex", `.*\.internal\.com`))

	entries, err := m.ListEntries(ctx, wsID)
	require.NoError(t, err)
	assert.Len(t, entries, 5)
}

func TestManager_LoadScope(t *testing.T) {
	db := openTestDB(t)
	m := scope.NewManager(db.DB())
	ctx := context.Background()
	wsID := "load-test"

	require.NoError(t, m.AddRule(ctx, wsID, "domain", "example.com"))
	require.NoError(t, m.AddRule(ctx, wsID, "exclude", "/private"))

	s, err := m.Load(ctx, wsID, "example.com")
	require.NoError(t, err)
	assert.Equal(t, wsID, s.WorkspaceID)
	assert.Equal(t, "example.com", s.TargetHost)
	assert.Len(t, s.Domains, 1)
	assert.Len(t, s.Excludes, 1)

	// Verify scope matching works after load
	assert.True(t, s.IsInScope("https://example.com/public"))
	assert.False(t, s.IsInScope("https://example.com/private"))
}

func TestManager_RemoveRule(t *testing.T) {
	db := openTestDB(t)
	m := scope.NewManager(db.DB())
	ctx := context.Background()
	wsID := "remove-test"

	require.NoError(t, m.AddRule(ctx, wsID, "domain", "example.com"))
	entries, err := m.ListEntries(ctx, wsID)
	require.NoError(t, err)
	require.Len(t, entries, 1)

	require.NoError(t, m.RemoveRule(ctx, entries[0].ID))

	entries, err = m.ListEntries(ctx, wsID)
	require.NoError(t, err)
	assert.Len(t, entries, 0)
}

func TestManager_RemoveRule_NotFound(t *testing.T) {
	db := openTestDB(t)
	m := scope.NewManager(db.DB())
	ctx := context.Background()

	err := m.RemoveRule(ctx, "nonexistent-id")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestManager_ClearRules_All(t *testing.T) {
	db := openTestDB(t)
	m := scope.NewManager(db.DB())
	ctx := context.Background()
	wsID := "clear-all"

	require.NoError(t, m.AddRule(ctx, wsID, "domain", "example.com"))
	require.NoError(t, m.AddRule(ctx, wsID, "cidr", "10.0.0.0/8"))

	require.NoError(t, m.ClearRules(ctx, wsID, ""))

	entries, err := m.ListEntries(ctx, wsID)
	require.NoError(t, err)
	assert.Len(t, entries, 0)
}

func TestManager_ClearRules_ByType(t *testing.T) {
	db := openTestDB(t)
	m := scope.NewManager(db.DB())
	ctx := context.Background()
	wsID := "clear-type"

	require.NoError(t, m.AddRule(ctx, wsID, "domain", "example.com"))
	require.NoError(t, m.AddRule(ctx, wsID, "cidr", "10.0.0.0/8"))

	require.NoError(t, m.ClearRules(ctx, wsID, "cidr"))

	entries, err := m.ListEntries(ctx, wsID)
	require.NoError(t, err)
	assert.Len(t, entries, 1)
	assert.Equal(t, "domain", entries[0].EntryType)
}

func TestManager_AddRule_Validation(t *testing.T) {
	db := openTestDB(t)
	m := scope.NewManager(db.DB())
	ctx := context.Background()

	assert.Error(t, m.AddRule(ctx, "ws", "domain", "notadomain"))
	assert.Error(t, m.AddRule(ctx, "ws", "cidr", "invalid"))
	assert.Error(t, m.AddRule(ctx, "ws", "regex", `[invalid`))
	assert.Error(t, m.AddRule(ctx, "ws", "unknown_type", "value"))
	assert.NoError(t, m.AddRule(ctx, "ws", "exclude", "anything goes here"))
}

func TestManager_WildcardDomainConverted(t *testing.T) {
	db := openTestDB(t)
	m := scope.NewManager(db.DB())
	ctx := context.Background()
	wsID := "wildcard-test"

	require.NoError(t, m.AddRule(ctx, wsID, "subdomain", "*.example.com"))

	// Should have been converted to a domain entry for "example.com"
	entries, err := m.ListEntries(ctx, wsID)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "domain", entries[0].EntryType)
	assert.Equal(t, "example.com", entries[0].Pattern)
}

func TestManager_SetRateLimit(t *testing.T) {
	db := openTestDB(t)
	m := scope.NewManager(db.DB())
	ctx := context.Background()
	wsID := "ratelimit-test"

	require.NoError(t, m.SetRateLimit(ctx, wsID, 5.0, 10))

	s, err := m.Load(ctx, wsID, "")
	require.NoError(t, err)
	assert.InDelta(t, 5.0, s.RateLimit.RequestsPerSec, 0.01)
	assert.Equal(t, 10, s.RateLimit.Burst)
}

func TestManager_SetAuth(t *testing.T) {
	db := openTestDB(t)
	m := scope.NewManager(db.DB())
	ctx := context.Background()
	wsID := "auth-test"

	require.NoError(t, m.SetAuth(ctx, wsID, "bearer", "tok-123"))

	s, err := m.Load(ctx, wsID, "")
	require.NoError(t, err)
	assert.Equal(t, "bearer", s.Auth.Type)
	assert.Equal(t, "tok-123", s.Auth.Value)
}

func TestManager_SetRespectRobots(t *testing.T) {
	db := openTestDB(t)
	m := scope.NewManager(db.DB())
	ctx := context.Background()
	wsID := "robots-test"

	require.NoError(t, m.SetRespectRobots(ctx, wsID, false))

	s, err := m.Load(ctx, wsID, "")
	require.NoError(t, err)
	assert.False(t, s.RespectRobots)
}

func TestManager_ScopeIsolation(t *testing.T) {
	db := openTestDB(t)
	m := scope.NewManager(db.DB())
	ctx := context.Background()

	require.NoError(t, m.AddRule(ctx, "ws-a", "domain", "example.com"))
	require.NoError(t, m.AddRule(ctx, "ws-b", "domain", "other.com"))

	entriesA, err := m.ListEntries(ctx, "ws-a")
	require.NoError(t, err)
	assert.Len(t, entriesA, 1)

	entriesB, err := m.ListEntries(ctx, "ws-b")
	require.NoError(t, err)
	assert.Len(t, entriesB, 1)
}

// --- Robots.txt tests ---

func TestParseRobotsTxt_DefaultAgent(t *testing.T) {
	body := `User-agent: *
Disallow: /admin
Disallow: /private
Allow: /public

Sitemap: https://example.com/sitemap.xml
Crawl-delay: 5`

	rt := scope.ParseRobotsTxt(body)
	assert.Equal(t, []string{"/admin", "/private"}, rt.DisallowedPaths)
	assert.Equal(t, []string{"/public"}, rt.AllowedPaths)
	assert.Equal(t, []string{"https://example.com/sitemap.xml"}, rt.Sitemaps)
	assert.InDelta(t, 5*float64(time.Second), float64(rt.CrawlDelay), float64(time.Millisecond))
}

func TestParseRobotsTxt_OtherAgent(t *testing.T) {
	body := `User-agent: Googlebot
Disallow: /

User-agent: *
Disallow: /api`
	rt := scope.ParseRobotsTxt(body)
	assert.Equal(t, []string{"/api"}, rt.DisallowedPaths)
}

func TestParseRobotsTxt_Empty(t *testing.T) {
	rt := scope.ParseRobotsTxt("")
	assert.Empty(t, rt.DisallowedPaths)
	assert.Empty(t, rt.AllowedPaths)
}

func TestParseRobotsTxt_CommentsAndBlanks(t *testing.T) {
	body := `# This is a comment

User-agent: *
# Another comment
Disallow: /secret
`
	rt := scope.ParseRobotsTxt(body)
	assert.Equal(t, []string{"/secret"}, rt.DisallowedPaths)
}

func TestRobotsTxt_IsAllowed(t *testing.T) {
	rt := scope.ParseRobotsTxt(`User-agent: *
Disallow: /admin
Allow: /admin/public`)

	assert.False(t, rt.IsAllowed("https://example.com/admin/private"))
	assert.True(t, rt.IsAllowed("https://example.com/admin/public"))
	assert.True(t, rt.IsAllowed("https://example.com/home"))
}

func TestManager_Load_EmptyDB(t *testing.T) {
	db := openTestDB(t)
	m := scope.NewManager(db.DB())
	ctx := context.Background()

	s, err := m.Load(ctx, "nonexistent-ws", "example.com")
	require.NoError(t, err)
	assert.Equal(t, 10.0, s.RateLimit.RequestsPerSec)
	assert.Equal(t, 5, s.RateLimit.Burst)
	assert.True(t, s.RespectRobots)
	assert.Empty(t, s.Domains)
}

func TestScope_String_Formatting(t *testing.T) {
	s := &scope.Scope{
		WorkspaceID: "abc123",
		TargetHost:  "example.com",
		RateLimit:   scope.RateLimitConfig{RequestsPerSec: 20, Burst: 10},
	}

	str := s.String()
	assert.Contains(t, str, "20.0")
	assert.Contains(t, str, "10")
}

// Edge cases
func TestScope_IsInScope_MalformedURL(t *testing.T) {
	s := &scope.Scope{
		Domains: []string{"example.com"},
	}
	assert.False(t, s.IsInScope("://invalid-url"))
}

func TestScope_IsInScope_EmptyURL(t *testing.T) {
	s := &scope.Scope{
		TargetHost: "example.com",
	}
	assert.False(t, s.IsInScope(""))
}

func TestScope_IsInScope_ExcludeWithDotAll(t *testing.T) {
	s := &scope.Scope{
		Domains:  []string{"example.com"},
		Excludes: []string{"sub.example.com"},
	}

	assert.True(t, s.IsInScope("https://example.com/page"))
	assert.False(t, s.IsInScope("https://sub.example.com/page"))
	assert.False(t, s.IsInScope("https://deep.sub.example.com/page"))
}

func TestScope_ValidateDomain_MultipleDots(t *testing.T) {
	assert.NoError(t, scope.ValidateDomain("deep.sub.example.com"))
}

func TestScope_NormalizeDomain_Various(t *testing.T) {
	assert.Equal(t, "example.com", scope.NormalizeDomain("  https://example.com/path?q=1  "))
	assert.Equal(t, "example.com", scope.NormalizeDomain("example.com:8080"))
	assert.Equal(t, "", scope.NormalizeDomain(""))
}
