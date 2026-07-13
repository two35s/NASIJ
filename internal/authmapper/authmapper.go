package authmapper

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/nasij/nasij/internal/runtime"
)

type Mapping struct {
	Source        string          `json:"source"`
	JWTs          []JWTInfo       `json:"jwts"`
	OAuthFlows    []OAuthFlow     `json:"oauth_flows"`
	Cookies       []AuthCookie    `json:"cookies"`
	Endpoints     []AuthEndpoint  `json:"endpoints"`
	StorageTokens []StorageToken  `json:"storage_tokens"`
	Flows         []Flow          `json:"flows"`
	Diagram       string          `json:"diagram"`
}

type JWTInfo struct {
	Token     string         `json:"token"`
	Header    map[string]any `json:"header"`
	Payload   map[string]any `json:"payload"`
	Algorithm string         `json:"algorithm"`
	TokenType string         `json:"token_type"` // access, refresh, id, reset, unknown
	Source    string         `json:"source"`     // cookie, header, body, storage
	Location  string         `json:"location"`
	ExpiresAt *time.Time     `json:"expires_at,omitempty"`
	IssuedAt  *time.Time     `json:"issued_at,omitempty"`
	Subject   string         `json:"subject"`
	Issuer    string         `json:"issuer"`
	Scopes    []string       `json:"scopes"`
}

type OAuthFlow struct {
	Type          string   `json:"type"`           // authorization_code, implicit, client_credentials, pkce, hybrid
	AuthEndpoint  string   `json:"auth_endpoint"`
	TokenEndpoint string   `json:"token_endpoint"`
	RedirectURI   string   `json:"redirect_uri"`
	ClientID      string   `json:"client_id"`
	Scopes        []string `json:"scopes"`
	PKCE          bool     `json:"pkce"`
	OIDC          bool     `json:"oidc"`
	Signal        string   `json:"signal"`
}

type AuthCookie struct {
	Name        string `json:"name"`
	Domain      string `json:"domain"`
	Path        string `json:"path"`
	Secure      bool   `json:"secure"`
	HttpOnly    bool   `json:"http_only"`
	SameSite    string `json:"same_site"`
	TokenType   string `json:"token_type"`   // session, refresh, access, id, unknown
	Suspicious  bool   `json:"suspicious"`   // token-like value
}

type AuthEndpoint struct {
	URL        string   `json:"url"`
	Method     string   `json:"method"`
	EndpointType string `json:"endpoint_type"` // login, logout, register, token, authorize, callback, refresh, revoke
	Parameters []string `json:"parameters"`
	Signal     string   `json:"signal"`
}

type StorageToken struct {
	Source    string   `json:"source"`     // localStorage, sessionStorage, IndexedDB
	Key       string   `json:"key"`
	Value     string   `json:"value"`
	TokenType string   `json:"token_type"` // access, refresh, id, unknown
	JWT       *JWTInfo `json:"jwt,omitempty"`
}

type Flow struct {
	Name  string     `json:"name"`
	Steps []FlowStep `json:"steps"`
}

type FlowStep struct {
	Order  int    `json:"order"`
	Action string `json:"action"` // request, response, redirect, storage, script
	URL    string `json:"url"`
	Method string `json:"method"`
	Detail string `json:"detail"`
}

func MapFromResult(r *runtime.Result) *Mapping {
	m := &Mapping{
		Source: r.URL,
	}
	seenJWT := make(map[string]bool)
	seenEndpoint := make(map[string]bool)
	seenOAuthFlow := make(map[string]bool)

	for i := range r.Requests {
		req := &r.Requests[i]
		detectJWTInRequest(m, req, seenJWT)
		detectOAuthEndpoint(m, req, seenEndpoint, seenOAuthFlow)
		detectAuthEndpoint(m, req, seenEndpoint)
		detectRefreshFlow(m, req)
	}

	for i := range r.Cookies {
		c := &r.Cookies[i]
		analyzeCookie(m, c)
		detectJWTInCookie(m, c, seenJWT)
	}

	for i := range r.LocalStorage {
		s := &r.LocalStorage[i]
		analyzeStorage(m, "localStorage", s.Key, s.Value, seenJWT)
	}

	for i := range r.SessionStorage {
		s := &r.SessionStorage[i]
		analyzeStorage(m, "sessionStorage", s.Key, s.Value, seenJWT)
	}

	for i := range r.IndexedDB {
		db := &r.IndexedDB[i]
		for _, store := range db.Stores {
			for _, record := range store.Records {
				if s, ok := record.(string); ok {
					analyzeStorage(m, "IndexedDB/"+db.Database+"/"+store.Name, "", s, seenJWT)
				}
				if kv, ok := record.(map[string]any); ok {
					for k, v := range kv {
						vs, _ := v.(string)
						analyzeStorage(m, "IndexedDB/"+db.Database+"/"+store.Name, k, vs, seenJWT)
					}
				}
			}
		}
	}

	detectLoginFlow(m)
	detectLogoutFlow(m)
	detectOAuthFlowIfComplete(m)

	m.Diagram = GenerateDiagram(m)

	return m
}

func MapFromData(url string, requests []runtime.RequestRecord, cookies []runtime.CookieRecord, localStorage, sessionStorage []runtime.StorageRecord, indexedDB []runtime.IndexedDBRecord) *Mapping {
	r := &runtime.Result{
		URL:            url,
		Requests:       requests,
		Cookies:        cookies,
		LocalStorage:   localStorage,
		SessionStorage: sessionStorage,
		IndexedDB:      indexedDB,
	}
	return MapFromResult(r)
}

var jwtRegex = fmt.Sprintf(`[A-Za-z0-9_-]{%d,}\.[A-Za-z0-9_-]{%d,}\.[A-Za-z0-9_-]+`, 10, 10)

func parseJWTRaw(token string) *JWTInfo {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil
	}
	if len(parts[0]) < 10 || len(parts[1]) < 10 {
		return nil
	}

	headerJSON, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		if headerJSON, err = base64.StdEncoding.DecodeString(parts[0]); err != nil {
			return nil
		}
	}

	payloadJSON, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		if payloadJSON, err = base64.StdEncoding.DecodeString(parts[1]); err != nil {
			return nil
		}
	}

	var header map[string]any
	if err := json.Unmarshal(headerJSON, &header); err != nil {
		return nil
	}
	var payload map[string]any
	if err := json.Unmarshal(payloadJSON, &payload); err != nil {
		return nil
	}

	info := &JWTInfo{
		Token:     token,
		Header:    header,
		Payload:   payload,
		TokenType: "unknown",
	}

	if alg, ok := header["alg"].(string); ok {
		info.Algorithm = alg
	}
	if sub, ok := payload["sub"].(string); ok {
		info.Subject = sub
	}
	if iss, ok := payload["iss"].(string); ok {
		info.Issuer = iss
	}
	if exp, ok := payload["exp"].(float64); ok {
		t := time.Unix(int64(exp), 0)
		info.ExpiresAt = &t
	}
	if iat, ok := payload["iat"].(float64); ok {
		t := time.Unix(int64(iat), 0)
		info.IssuedAt = &t
	}
	if scope, ok := payload["scope"].(string); ok {
		info.Scopes = strings.Fields(scope)
	}
	if scp, ok := payload["scopes"].([]any); ok {
		for _, s := range scp {
			if ss, ok := s.(string); ok {
				info.Scopes = append(info.Scopes, ss)
			}
		}
	}

	if typ, ok := payload["type"].(string); ok {
		info.TokenType = typ
	} else if typ, ok := payload["token_type"].(string); ok {
		info.TokenType = typ
	} else {
		info.TokenType = classifyJWT(info)
	}

	return info
}

func classifyJWT(j *JWTInfo) string {
	typ, _ := j.Payload["type"].(string)
	switch strings.ToLower(typ) {
	case "access", "refresh", "id":
		return strings.ToLower(typ)
	}
	if j.Subject != "" && j.Issuer != "" && j.ExpiresAt != nil {
		return "access"
	}
	if _, ok := j.Payload["nonce"]; ok {
		return "id"
	}
	if _, ok := j.Payload["refresh"]; ok || strings.Contains(j.Token, "refresh") {
		return "refresh"
	}
	for _, scope := range j.Scopes {
		if scope == "openid" {
			return "id"
		}
	}
	return "access"
}

func isLikelyToken(v string) bool {
	if len(v) < 20 {
		return false
	}
	if strings.HasPrefix(v, "eyJ") || strings.HasPrefix(v, "eyI") {
		return true
	}
	alphaNum := 0
	for _, c := range v {
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '_' || c == '.' {
			alphaNum++
		}
	}
	return float64(alphaNum)/float64(len(v)) > 0.8 && len(v) > 40
}

var authCookieNames = map[string]string{
	"session":           "session",
	"sessionid":         "session",
	"sid":               "session",
	"connect.sid":       "session",
	"token":             "access",
	"access_token":      "access",
	"access-token":      "access",
	"refresh_token":     "refresh",
	"refresh-token":     "refresh",
	"id_token":          "id",
	"jwt":               "access",
	"auth":              "access",
	"authorization":     "access",
	"xsrf-token":        "csrf",
	"csrf-token":        "csrf",
	"x-csrf-token":      "csrf",
	"remember_me":       "session",
	"remember-me":       "session",
}

func lookupCookieType(name string) (string, bool) {
	lower := strings.ToLower(name)
	if t, ok := authCookieNames[lower]; ok {
		return t, true
	}
	for pattern, t := range authCookieNames {
		if strings.Contains(lower, pattern) {
			return t, true
		}
	}
	return "", false
}
