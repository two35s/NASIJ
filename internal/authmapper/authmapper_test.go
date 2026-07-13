package authmapper

import (
	"testing"

	"github.com/nasij/nasij/internal/runtime"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseJWT_Valid(t *testing.T) {
	token := "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyLCJleHAiOjk5OTk5OTk5OTksImlzcyI6ImF1dGguZXhhbXBsZS5jb20iLCJzY29wZSI6Im9wZW5pZCBwcm9maWxlIn0.signature"
	j := parseJWTRaw(token)
	require.NotNil(t, j)
	assert.Equal(t, "RS256", j.Algorithm)
	assert.Equal(t, "1234567890", j.Subject)
	assert.Equal(t, "auth.example.com", j.Issuer)
	assert.Equal(t, "access", j.TokenType)
	assert.Equal(t, []string{"openid", "profile"}, j.Scopes)
	require.NotNil(t, j.ExpiresAt)
	assert.Equal(t, int64(9999999999), j.ExpiresAt.Unix())
	require.NotNil(t, j.IssuedAt)
	assert.Equal(t, int64(1516239022), j.IssuedAt.Unix())
}

func TestParseJWT_RefreshToken(t *testing.T) {
	token := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiJ1c2VyMTIzIiwidHlwZSI6InJlZnJlc2giLCJpYXQiOjE1MTYyMzkwMjJ9.signature"
	j := parseJWTRaw(token)
	require.NotNil(t, j)
	assert.Equal(t, "HS256", j.Algorithm)
	assert.Equal(t, "refresh", j.TokenType)
}

func TestParseJWT_IDToken(t *testing.T) {
	token := "eyJhbGciOiJSUzI1NiJ9.eyJzdWIiOiJ1c2VyMTIzIiwibm9uY2UiOiJhYmMxMjMiLCJhdF9oYXNoIjoiZGVmNDU2In0.signature"
	j := parseJWTRaw(token)
	require.NotNil(t, j)
	assert.Equal(t, "id", j.TokenType)
}

func TestParseJWT_Invalid(t *testing.T) {
	assert.Nil(t, parseJWTRaw("not.a.jwt"))
	assert.Nil(t, parseJWTRaw("short.a.b"))
	assert.Nil(t, parseJWTRaw(""))
	assert.Nil(t, parseJWTRaw("a.b.c"))
}

func TestParseJWT_OIDCScope(t *testing.T) {
	token := "eyJhbGciOiJSUzI1NiJ9.eyJzdWIiOiJ1c2VyIiwic2NvcGVzIjpbIm9wZW5pZCIsInByb2ZpbGUiXX0.signature"
	j := parseJWTRaw(token)
	require.NotNil(t, j)
	assert.Equal(t, "id", j.TokenType)
	assert.Contains(t, j.Scopes, "openid")
}

func TestParseJWT_TokenTypeFromPayload(t *testing.T) {
	token := "eyJhbGciOiJSUzI1NiJ9.eyJzdWIiOiJ1c2VyIiwidG9rZW5fdHlwZSI6ImFjY2VzcyJ9.signature"
	j := parseJWTRaw(token)
	require.NotNil(t, j)
	assert.Equal(t, "access", j.TokenType)
}

func TestMapFromResult_NoAuth(t *testing.T) {
	r := &runtime.Result{URL: "https://example.com"}
	r.Requests = append(r.Requests, runtime.RequestRecord{
		URL: "https://example.com/", Method: "GET", ResourceType: "document", StatusCode: 200,
	})
	m := MapFromResult(r)
	assert.Empty(t, m.JWTs)
	assert.Empty(t, m.OAuthFlows)
	assert.Empty(t, m.Cookies)
	assert.Empty(t, m.Endpoints)
	assert.Empty(t, m.StorageTokens)
	assert.Empty(t, m.Diagram)
}

func TestMapFromResult_JWTInHeader(t *testing.T) {
	r := &runtime.Result{URL: "https://api.example.com/data"}
	r.Requests = append(r.Requests, runtime.RequestRecord{
		URL:    "https://api.example.com/data",
		Method: "GET",
		ResourceType: "fetch",
		StatusCode:   200,
		Headers: map[string]string{
			"authorization": "Bearer eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiJ1c2VyMTIzIiwiaXNzIjoiYXV0aC5leGFtcGxlLmNvbSIsImV4cCI6OTk5OTk5OTk5OSwiaWF0IjoxNTE2MjM5MDIyfQ.signature",
		},
	})
	m := MapFromResult(r)
	require.Len(t, m.JWTs, 1)
	assert.Equal(t, "header", m.JWTs[0].Source)
	assert.Equal(t, "authorization", m.JWTs[0].Location)
	assert.Equal(t, "RS256", m.JWTs[0].Algorithm)
	assert.Equal(t, "user123", m.JWTs[0].Subject)
}

func TestMapFromResult_JWTInCookie(t *testing.T) {
	r := &runtime.Result{URL: "https://example.com"}
	r.Cookies = append(r.Cookies, runtime.CookieRecord{
		Name: "session", Value: "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJ1c2VyIiwiaXNzIjoiZXhhbXBsZSJ9.signature",
	})
	m := MapFromResult(r)
	require.Len(t, m.JWTs, 1)
	assert.Equal(t, "cookie", m.JWTs[0].Source)
	assert.Equal(t, "session", m.JWTs[0].Location)

	require.Len(t, m.Cookies, 1)
	assert.Equal(t, "session", m.Cookies[0].TokenType)
	assert.True(t, m.Cookies[0].Suspicious)
}

func TestMapFromResult_JWTInURL(t *testing.T) {
	r := &runtime.Result{URL: "https://example.com/callback?code=abc&access_token=eyJhbGciOiJSUzI1NiJ9.eyJzdWIiOiJ1c2VyIn0.signature"}
	r.Requests = append(r.Requests, runtime.RequestRecord{
		URL:    "https://example.com/callback?code=abc&access_token=eyJhbGciOiJSUzI1NiJ9.eyJzdWIiOiJ1c2VyIn0.signature",
		Method: "GET", ResourceType: "document", StatusCode: 200,
	})
	m := MapFromResult(r)
	require.Len(t, m.JWTs, 1)
	assert.Equal(t, "param", m.JWTs[0].Source)
	assert.Equal(t, "access_token", m.JWTs[0].Location)
}

func TestMapFromResult_AuthEndpoints(t *testing.T) {
	r := &runtime.Result{URL: "https://example.com"}
	r.Requests = []runtime.RequestRecord{
		{URL: "https://example.com/login", Method: "POST", ResourceType: "document"},
		{URL: "https://example.com/logout", Method: "POST", ResourceType: "document"},
		{URL: "https://example.com/oauth/authorize?response_type=code&client_id=app&redirect_uri=https://app.com/callback&scope=openid+profile", Method: "GET", ResourceType: "document"},
		{URL: "https://example.com/oauth/token", Method: "POST", ResourceType: "fetch"},
		{URL: "https://example.com/auth/refresh", Method: "POST", ResourceType: "fetch"},
		{URL: "https://example.com/register", Method: "POST", ResourceType: "document"},
		{URL: "https://example.com/oauth/callback?code=abc123&state=xyz", Method: "GET", ResourceType: "document"},
	}
	m := MapFromResult(r)

	endpointTypes := make(map[string]int)
	for _, ep := range m.Endpoints {
		endpointTypes[ep.EndpointType]++
	}
	assert.Equal(t, 1, endpointTypes["login"])
	assert.Equal(t, 1, endpointTypes["logout"])
	assert.Equal(t, 1, endpointTypes["register"])
	assert.Equal(t, 1, endpointTypes["refresh"])
	assert.Equal(t, 1, endpointTypes["callback"])
	assert.Equal(t, 1, endpointTypes["authorize"])
	assert.Equal(t, 1, endpointTypes["token"])
}

func TestMapFromResult_OAuthFlow(t *testing.T) {
	r := &runtime.Result{URL: "https://example.com"}
	r.Requests = []runtime.RequestRecord{
		{
			URL:    "https://example.com/oauth/authorize?response_type=code&client_id=myapp&redirect_uri=https://app.com/callback&scope=openid+profile&state=abc123",
			Method: "GET", ResourceType: "document",
		},
	}
	m := MapFromResult(r)
	require.Len(t, m.OAuthFlows, 1)
	assert.Equal(t, "authorization_code", m.OAuthFlows[0].Type)
	assert.Equal(t, "myapp", m.OAuthFlows[0].ClientID)
	assert.Equal(t, "https://app.com/callback", m.OAuthFlows[0].RedirectURI)
	assert.Contains(t, m.OAuthFlows[0].Scopes, "openid")
}

func TestMapFromResult_OAuthPKCE(t *testing.T) {
	r := &runtime.Result{URL: "https://example.com"}
	r.Requests = []runtime.RequestRecord{
		{
			URL:    "https://example.com/oauth/authorize?response_type=code&client_id=app&redirect_uri=https://app.com/callback&code_challenge=abc123&code_challenge_method=S256&scope=openid",
			Method: "GET", ResourceType: "document",
		},
	}
	m := MapFromResult(r)
	require.Len(t, m.OAuthFlows, 1)
	assert.True(t, m.OAuthFlows[0].PKCE)
	assert.True(t, m.OAuthFlows[0].OIDC)
}

func TestMapFromResult_ClientCredentials(t *testing.T) {
	r := &runtime.Result{URL: "https://example.com"}
	r.Requests = []runtime.RequestRecord{
		{
			URL:    "https://example.com/oauth/token?grant_type=client_credentials&client_id=svc&client_secret=secret",
			Method: "POST", ResourceType: "fetch",
		},
	}
	m := MapFromResult(r)
	require.Len(t, m.OAuthFlows, 1)
	assert.Equal(t, "client_credentials", m.OAuthFlows[0].Type)
}

func TestMapFromResult_RefreshFlow(t *testing.T) {
	r := &runtime.Result{URL: "https://example.com"}
	r.Requests = []runtime.RequestRecord{
		{
			URL:    "https://example.com/auth/refresh?grant_type=refresh_token&refresh_token=abc123",
			Method: "POST", ResourceType: "fetch", StatusCode: 200,
		},
	}
	m := MapFromResult(r)
	require.Len(t, m.Endpoints, 1)
	assert.Equal(t, "refresh", m.Endpoints[0].EndpointType)

	foundRefresh := false
	for _, f := range m.Flows {
		if f.Name == "token_refresh" {
			foundRefresh = true
			assert.Len(t, f.Steps, 3)
			assert.Equal(t, "response", f.Steps[2].Action)
			assert.Contains(t, f.Steps[2].Detail, "new tokens")
		}
	}
	assert.True(t, foundRefresh)
}

func TestMapFromResult_Cookies(t *testing.T) {
	r := &runtime.Result{URL: "https://example.com"}
	r.Cookies = []runtime.CookieRecord{
		{Name: "session", Value: "abc123", Domain: "example.com", Path: "/", Secure: true, HttpOnly: true},
		{Name: "refresh_token", Value: "xyz789", Domain: "example.com", Path: "/", Secure: true, HttpOnly: true},
		{Name: "csrf-token", Value: "csrf123", Domain: "example.com"},
		{Name: "analytics", Value: "ga:12345", Domain: "example.com"},
	}
	m := MapFromResult(r)
	require.Len(t, m.Cookies, 4)

	cookieTypes := make(map[string]string)
	for _, c := range m.Cookies {
		cookieTypes[c.Name] = c.TokenType
	}
	assert.Equal(t, "session", cookieTypes["session"])
	assert.Equal(t, "refresh", cookieTypes["refresh_token"])
	assert.Equal(t, "csrf", cookieTypes["csrf-token"])
	assert.Equal(t, "unknown", cookieTypes["analytics"])
	assert.False(t, m.Cookies[2].Suspicious)
}

func TestMapFromResult_SuspiciousCookie(t *testing.T) {
	r := &runtime.Result{}
	r.Cookies = []runtime.CookieRecord{
		{Name: "auth", Value: "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJ1c2VyIn0.signature_abcdefgh_long_enough_suspect"},
	}
	m := MapFromResult(r)
	require.Len(t, m.Cookies, 1)
	assert.True(t, m.Cookies[0].Suspicious)
}

func TestMapFromResult_StorageTokens(t *testing.T) {
	r := &runtime.Result{URL: "https://example.com"}
	r.LocalStorage = []runtime.StorageRecord{
		{Key: "access_token", Value: "eyJhbGciOiJSUzI1NiJ9.eyJzdWIiOiJ1c2VyIiwidHlwZSI6ImFjY2VzcyJ9.signature"},
		{Key: "theme", Value: "dark"},
	}
	r.SessionStorage = []runtime.StorageRecord{
		{Key: "refresh_token", Value: "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJ1c2VyIiwidHlwZSI6InJlZnJlc2gifQ.signature"},
	}
	m := MapFromResult(r)
	require.Len(t, m.StorageTokens, 2)
	assert.Equal(t, "localStorage", m.StorageTokens[0].Source)
	assert.Equal(t, "access", m.StorageTokens[0].TokenType)
	assert.Equal(t, "sessionStorage", m.StorageTokens[1].Source)
	assert.Equal(t, "refresh", m.StorageTokens[1].TokenType)

	require.Len(t, m.JWTs, 2)
}

func TestMapFromResult_IndexedDBTokens(t *testing.T) {
	r := &runtime.Result{URL: "https://example.com"}
	r.IndexedDB = []runtime.IndexedDBRecord{
		{
			Database: "auth_db",
			Stores: []runtime.IndexedDBStore{
				{
					Name: "tokens",
					Records: []any{
						map[string]any{"key": "access", "value": "eyJhbGciOiJSUzI1NiJ9.eyJzdWIiOiJ1c2VyIiwidHlwZSI6ImFjY2VzcyJ9.signature"},
					},
				},
			},
		},
	}
	m := MapFromResult(r)
	require.Len(t, m.StorageTokens, 1)
	assert.Equal(t, "IndexedDB/auth_db/tokens", m.StorageTokens[0].Source)
}

func TestMapFromResult_LoginFlow(t *testing.T) {
	r := &runtime.Result{URL: "https://example.com"}
	r.Requests = []runtime.RequestRecord{
		{URL: "https://example.com/login", Method: "POST", ResourceType: "document"},
		{URL: "https://example.com/dashboard", Method: "GET", ResourceType: "document"},
	}
	r.Cookies = []runtime.CookieRecord{
		{Name: "session", Value: "abc123"},
	}
	m := MapFromResult(r)
	foundLogin := false
	for _, f := range m.Flows {
		if f.Name == "login" {
			foundLogin = true
			assert.Len(t, f.Steps, 2)
		}
	}
	assert.True(t, foundLogin)
}

func TestMapFromResult_OAuthLoginFlow(t *testing.T) {
	r := &runtime.Result{URL: "https://example.com"}
	r.Requests = []runtime.RequestRecord{
		{URL: "https://example.com/login", Method: "POST", ResourceType: "document"},
		{URL: "https://example.com/oauth/callback?code=abc", Method: "GET", ResourceType: "document"},
		{URL: "https://example.com/oauth/token", Method: "POST", ResourceType: "fetch"},
	}
	r.Cookies = []runtime.CookieRecord{
		{Name: "access_token", Value: "some_token_value"},
	}
	m := MapFromResult(r)
	foundOAuth := false
	for _, f := range m.Flows {
		if f.Name == "oauth_login" {
			foundOAuth = true
			assert.Len(t, f.Steps, 3)
		}
	}
	assert.True(t, foundOAuth)
}

func TestMapFromResult_LogoutFlow(t *testing.T) {
	r := &runtime.Result{URL: "https://example.com"}
	r.Requests = []runtime.RequestRecord{
		{URL: "https://example.com/logout", Method: "POST"},
	}
	r.Cookies = []runtime.CookieRecord{
		{Name: "session", Value: "abc"},
	}
	m := MapFromResult(r)
	foundLogout := false
	for _, f := range m.Flows {
		if f.Name == "logout" {
			foundLogout = true
			assert.Len(t, f.Steps, 3)
		}
	}
	assert.True(t, foundLogout)
}

func TestMapFromData(t *testing.T) {
	reqs := []runtime.RequestRecord{
		{URL: "https://example.com/login", Method: "POST"},
	}
	cookies := []runtime.CookieRecord{
		{Name: "session", Value: "abc"},
	}
	m := MapFromData("https://example.com", reqs, cookies, nil, nil, nil)
	require.NotNil(t, m)
	assert.Equal(t, "https://example.com", m.Source)
	assert.Len(t, m.Endpoints, 1)
	assert.Equal(t, "login", m.Endpoints[0].EndpointType)
	assert.Len(t, m.Cookies, 1)
}

func TestDiagramGeneration(t *testing.T) {
	r := &runtime.Result{URL: "https://example.com"}
	r.Requests = []runtime.RequestRecord{
		{URL: "https://example.com/login", Method: "POST"},
	}
	r.Cookies = []runtime.CookieRecord{
		{Name: "session", Value: "abc", Secure: true},
	}
	m := MapFromResult(r)
	assert.NotEmpty(t, m.Diagram)
	assert.Contains(t, m.Diagram, "sequenceDiagram")
	assert.Contains(t, m.Diagram, "login")
	assert.Contains(t, m.Diagram, "session")
	assert.Contains(t, m.Diagram, "Browser")
	assert.Contains(t, m.Diagram, "Server")
}

func TestDiagramGeneration_NoAuth(t *testing.T) {
	r := &runtime.Result{URL: "https://example.com"}
	r.Requests = []runtime.RequestRecord{
		{URL: "https://example.com/", Method: "GET"},
	}
	m := MapFromResult(r)
	assert.Empty(t, m.Diagram)
}

func TestIsLikelyToken(t *testing.T) {
	assert.True(t, isLikelyToken("eyJhbGciOiJSUzI1NiJ9.eyJzdWIiOiJ1c2VyIn0.signature"))
	assert.True(t, isLikelyToken("abcdef12345_ghijklmno_pqrstuvw_xyzabcdefgh_1234567890"))
	assert.False(t, isLikelyToken(""))
	assert.False(t, isLikelyToken("hello world"))
	assert.False(t, isLikelyToken("short"))
}

func TestClassifyJWT(t *testing.T) {
	j := &JWTInfo{Payload: map[string]any{"type": "refresh"}}
	assert.Equal(t, "refresh", classifyJWT(j))

	j = &JWTInfo{Payload: map[string]any{}, Subject: "user", Issuer: "auth.example.com"}
	assert.Equal(t, "access", classifyJWT(j))

	j = &JWTInfo{Payload: map[string]any{"nonce": "abc123"}}
	assert.Equal(t, "id", classifyJWT(j))
}

func TestMapFromResult_JWTExpiresAt(t *testing.T) {
	token := "eyJhbGciOiJSUzI1NiJ9.eyJleHAiOjE3MDAwMDAwMDB9.signature"
	j := parseJWTRaw(token)
	require.NotNil(t, j)
	require.NotNil(t, j.ExpiresAt)
	assert.Equal(t, int64(1700000000), j.ExpiresAt.Unix())
}

func TestMapFromResult_OAuthOnlyCallback(t *testing.T) {
	r := &runtime.Result{URL: "https://example.com"}
	r.Requests = []runtime.RequestRecord{
		{
			URL:    "https://example.com/callback?code=abc123&state=xyz",
			Method: "GET", ResourceType: "document",
		},
	}
	m := MapFromResult(r)

	assert.Len(t, m.OAuthFlows, 0)
	assert.Len(t, m.Endpoints, 1)
	assert.Equal(t, "callback", m.Endpoints[0].EndpointType)
}

func TestMapFromResult_MultipleOAuthFlows(t *testing.T) {
	r := &runtime.Result{URL: "https://example.com"}
	r.Requests = []runtime.RequestRecord{
		{
			URL:    "https://example.com/oauth/authorize?response_type=code&client_id=app1&redirect_uri=https://app1.com/callback&scope=openid",
			Method: "GET",
		},
		{
			URL:    "https://example.com/oauth/authorize?response_type=token&client_id=app2&redirect_uri=https://app2.com/callback",
			Method: "GET",
		},
	}
	m := MapFromResult(r)
	assert.Len(t, m.OAuthFlows, 2)
}

func TestMapFromResult_ImplicitFlow(t *testing.T) {
	r := &runtime.Result{URL: "https://example.com"}
	r.Requests = []runtime.RequestRecord{
		{
			URL:    "https://example.com/oauth/authorize?response_type=token&client_id=app&redirect_uri=https://app.com/callback&scope=openid+profile",
			Method: "GET",
		},
	}
	m := MapFromResult(r)
	require.Len(t, m.OAuthFlows, 1)
	assert.Equal(t, "implicit", m.OAuthFlows[0].Type)
}

func TestGenerateDiagram_EmptyRecipes(t *testing.T) {
	d := GenerateDiagram(&Mapping{})
	assert.Equal(t, "", d)
}

func TestEscaping(t *testing.T) {
	s := escapeLabel(`he said "hello"`)
	assert.Equal(t, `he said 'hello'`, s)

	multi := escapeLabel("line1\nline2")
	assert.Equal(t, "line1 line2", multi)
}

func TestMapFromResult_AllSourceTypes(t *testing.T) {
	m := &Mapping{Source: "test"}

	jwt := JWTInfo{
		Token: "eyJhbGciOiJSUzI1NiJ9.eyJzdWIiOiJ1c2VyIn0.sig",
		Source: "url", Location: "https://example.com/login",
		Header: map[string]any{"alg": "RS256"},
		Payload: map[string]any{"sub": "user"},
		Algorithm: "RS256", TokenType: "access",
	}
	m.JWTs = append(m.JWTs, jwt)

	m.OAuthFlows = append(m.OAuthFlows, OAuthFlow{
		Type: "authorization_code", AuthEndpoint: "https://example.com/auth",
	})

	m.Cookies = append(m.Cookies, AuthCookie{
		Name: "session", TokenType: "session", Secure: true,
	})

	m.Endpoints = append(m.Endpoints, AuthEndpoint{
		URL: "https://example.com/login", Method: "POST", EndpointType: "login",
	})

	m.StorageTokens = append(m.StorageTokens, StorageToken{
		Source: "localStorage", Key: "token", TokenType: "access",
	})

	d := GenerateDiagram(m)
	assert.NotEmpty(t, d)
	assert.Contains(t, d, "RS256")
	assert.Contains(t, d, "session")
	assert.Contains(t, d, "Token Summary")
	assert.Contains(t, d, "Auth Cookies")
}

func TestOAuthFlowDetection_Hybrid(t *testing.T) {
	r := &runtime.Result{URL: "https://example.com"}
	r.Requests = []runtime.RequestRecord{
		{
			URL:    "https://example.com/oauth/authorize?response_type=code+id_token&client_id=app&redirect_uri=https://app.com/callback&scope=openid&nonce=abc",
			Method: "GET",
		},
	}
	m := MapFromResult(r)
	require.Len(t, m.OAuthFlows, 1)
	assert.Equal(t, "hybrid", m.OAuthFlows[0].Type)
}

func TestMapFromResult_AuthEndpointParameters(t *testing.T) {
	r := &runtime.Result{}
	r.Requests = []runtime.RequestRecord{
		{URL: "https://example.com/login?email=test@test.com&password=secret", Method: "POST"},
	}
	m := MapFromResult(r)
	require.Len(t, m.Endpoints, 1)
	assert.Contains(t, m.Endpoints[0].Parameters, "email")
	assert.Contains(t, m.Endpoints[0].Parameters, "password")
}

func TestMapFromResult_RefreshFailedFlow(t *testing.T) {
	r := &runtime.Result{}
	r.Requests = []runtime.RequestRecord{
		{
			URL: "https://example.com/auth/refresh", Method: "POST",
			ResourceType: "fetch", StatusCode: 401,
		},
	}
	m := MapFromResult(r)
	found := false
	for _, f := range m.Flows {
		if f.Name == "token_refresh" {
			found = true
			assert.Len(t, f.Steps, 2)
			assert.Contains(t, f.Steps[1].Detail, "expired")
		}
	}
	assert.True(t, found)
}
