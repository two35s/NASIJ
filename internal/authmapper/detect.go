package authmapper

import (
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/nasij/nasij/internal/runtime"
)

var (
	reJWTPattern      = regexp.MustCompile(`(eyJ[A-Za-z0-9_-]+\.eyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+)`)
	reBearerPattern   = regexp.MustCompile(`(?i)bearer\s+(eyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+)`)
	reTokenParam      = regexp.MustCompile(`(?i)(access_token|refresh_token|id_token|token)=([^&\s]+)`)
	reGrantType       = regexp.MustCompile(`(?i)grant_type=([^&\s]+)`)
	reOAuthEndpoint   = regexp.MustCompile(`(?i)(/oauth/|/authorize|/token|/auth/realms/|/oidc/)`)
	reLoginEndpoint   = regexp.MustCompile(`(?i)(/login|/signin|/sign-in|/auth/login|/authenticate)`)
	reLogoutEndpoint  = regexp.MustCompile(`(?i)(/logout|/signout|/sign-out|/auth/logout|/revoke)`)
	reRefreshEndpoint = regexp.MustCompile(`(?i)(/refresh|/token/refresh|/auth/refresh|/renew)`)
	reRegisterEndpoint = regexp.MustCompile(`(?i)(/register|/signup|/sign-up|/auth/register)`)
	reCallbackEndpoint = regexp.MustCompile(`(?i)(/callback|/oauth/callback|/auth/callback|/redirect)`)
	reTokenEndpoint   = regexp.MustCompile(`(?i)(/token$|/oauth/token|/auth/token)`)
	reAuthorizeEndpoint = regexp.MustCompile(`(?i)(/authorize|/oauth/authorize|/auth/authorize)`)
	rePKCEChallenge   = regexp.MustCompile(`(?i)code_challenge=`)
	reOIDCScope       = regexp.MustCompile(`(?i)scope=([^&\s]*)`)
)

func detectJWTInRequest(m *Mapping, req *runtime.RequestRecord, seen map[string]bool) {
	for key, val := range req.Headers {
		if bm := reBearerPattern.FindStringSubmatch(val); len(bm) > 1 {
			if j := parseJWTRaw(bm[1]); j != nil && !seen[j.Token] {
				seen[j.Token] = true
				j.Source = "header"
				j.Location = key
				m.JWTs = append(m.JWTs, *j)
			}
			continue
		}
		if strings.EqualFold(key, "authorization") {
			continue
		}
		if m2 := reJWTPattern.FindString(val); m2 != "" {
			if j := parseJWTRaw(m2); j != nil && !seen[j.Token] {
				seen[j.Token] = true
				j.Source = "header"
				j.Location = key
				m.JWTs = append(m.JWTs, *j)
			}
		}
	}

	bodyStr := req.URL
	if idx := strings.Index(bodyStr, "?"); idx >= 0 {
		params := bodyStr[idx+1:]
		if tm := reTokenParam.FindAllStringSubmatch(params, -1); tm != nil {
			for _, t := range tm {
				if j := parseJWTRaw(t[2]); j != nil && !seen[j.Token] {
					seen[j.Token] = true
					j.Source = "param"
					j.Location = t[1]
					m.JWTs = append(m.JWTs, *j)
				}
			}
		}
	}
}

func detectJWTInCookie(m *Mapping, c *runtime.CookieRecord, seen map[string]bool) {
	if j := parseJWTRaw(c.Value); j != nil && !seen[j.Token] {
		seen[j.Token] = true
		j.Source = "cookie"
		j.Location = c.Name
		m.JWTs = append(m.JWTs, *j)
	}
}

func detectOAuthEndpoint(m *Mapping, req *runtime.RequestRecord, seenEP map[string]bool, seenFlow map[string]bool) {
	u := req.URL
	if !reOAuthEndpoint.MatchString(u) {
		return
	}

	parsed, err := url.Parse(u)
	if err != nil {
		return
	}
	q := parsed.Query()

	hasResponseType := q.Has("response_type")
	hasClientID := q.Has("client_id")
	hasScope := q.Has("scope")
	hasGrantType := q.Has("grant_type")
	hasCode := q.Has("code")

	epType := "oauth"
	switch {
	case reAuthorizeEndpoint.MatchString(u):
		epType = "authorize"
	case reTokenEndpoint.MatchString(u):
		epType = "token"
	case reCallbackEndpoint.MatchString(u):
		epType = "callback"
	default:
		if hasResponseType || hasClientID {
			epType = "authorize"
		} else if hasGrantType || hasCode {
			epType = "token"
		}
	}

	epKey := u + "|" + epType
	if !seenEP[epKey] {
		seenEP[epKey] = true
		endpoint := AuthEndpoint{
			URL:          u,
			Method:       req.Method,
			EndpointType: epType,
			Signal:       "oauth_url_pattern",
		}
		for k := range q {
			endpoint.Parameters = append(endpoint.Parameters, k)
		}
		m.Endpoints = append(m.Endpoints, endpoint)
	}

	if !(hasResponseType || hasClientID || hasGrantType) {
		return
	}

	flow := OAuthFlow{
		Signal: "oauth_endpoint_detected",
	}
	if q.Has("redirect_uri") {
		flow.RedirectURI = q.Get("redirect_uri")
	}
	if q.Has("client_id") {
		flow.ClientID = q.Get("client_id")
	}
	if q.Has("scope") || hasScope {
		if s := q.Get("scope"); s != "" {
			flow.Scopes = strings.Fields(s)
		}
	}
	if rePKCEChallenge.MatchString(u) {
		flow.PKCE = true
	}
	if reOIDCScope.MatchString(u) {
		if sc := q.Get("scope"); strings.Contains(strings.ToLower(sc), "openid") {
			flow.OIDC = true
		}
	}

	rt := q.Get("response_type")
	switch rt {
	case "code":
		flow.Type = "authorization_code"
	case "token":
		flow.Type = "implicit"
	case "code id_token", "id_token code":
		flow.Type = "hybrid"
	case "id_token":
		flow.Type = "implicit"
		flow.OIDC = true
	default:
		if hasClientID && !hasResponseType {
			if q.Get("grant_type") == "client_credentials" {
				flow.Type = "client_credentials"
			} else {
				flow.Type = "authorization_code"
			}
		}
	}
	if gt := q.Get("grant_type"); gt != "" {
		switch gt {
		case "authorization_code":
			flow.Type = "authorization_code"
		case "implicit":
			flow.Type = "implicit"
		case "client_credentials":
			flow.Type = "client_credentials"
		case "refresh_token":
			if flow.Type == "" {
				flow.Type = "refresh"
			}
		}
	}
	if reAuthorizeEndpoint.MatchString(u) {
		flow.AuthEndpoint = u
	}
	if reTokenEndpoint.MatchString(u) {
		flow.TokenEndpoint = u
	}
	if flow.Type != "" {
		m.OAuthFlows = append(m.OAuthFlows, flow)
	}
}

func detectAuthEndpoint(m *Mapping, req *runtime.RequestRecord, seen map[string]bool) {
	u := req.URL
	var epType string

	switch {
	case reLoginEndpoint.MatchString(u):
		epType = "login"
	case reLogoutEndpoint.MatchString(u):
		epType = "logout"
	case reRegisterEndpoint.MatchString(u):
		epType = "register"
	case reRefreshEndpoint.MatchString(u):
		epType = "refresh"
	case reTokenEndpoint.MatchString(u):
		epType = "token"
	case reAuthorizeEndpoint.MatchString(u):
		epType = "authorize"
	case reCallbackEndpoint.MatchString(u):
		epType = "callback"
	}

	if epType == "" {
		return
	}

	epKey := u + "|" + epType
	if seen[epKey] {
		return
	}
	seen[epKey] = true

	ep := AuthEndpoint{
		URL:          u,
		Method:       req.Method,
		EndpointType: epType,
		Signal:       "url_path",
	}
	if parsed, err := url.Parse(u); err == nil {
		for k := range parsed.Query() {
			ep.Parameters = append(ep.Parameters, k)
		}
	}
	m.Endpoints = append(m.Endpoints, ep)
}

func detectRefreshFlow(m *Mapping, req *runtime.RequestRecord) {
	if !reRefreshEndpoint.MatchString(req.URL) {
		return
	}

	parsed, err := url.Parse(req.URL)
	if err != nil {
		return
	}
	q := parsed.Query()

	flow := Flow{Name: "token_refresh"}
	flow.Steps = append(flow.Steps, FlowStep{
		Order:  1,
		Action: "request",
		URL:    req.URL,
		Method: req.Method,
		Detail: "refresh token request",
	})

	if gt := q.Get("grant_type"); gt != "" {
		flow.Steps = append(flow.Steps, FlowStep{
			Order:  2,
			Action: "request",
			Detail: "grant_type: " + gt,
		})
	}

	if req.StatusCode > 0 && req.StatusCode < 400 {
		flow.Steps = append(flow.Steps, FlowStep{
			Order:  3,
			Action: "response",
			URL:    req.URL,
			Detail: "status: " + strconv.Itoa(req.StatusCode) + " — new tokens issued",
		})
	} else if req.StatusCode >= 400 {
		flow.Steps = append(flow.Steps, FlowStep{
			Order:  3,
			Action: "response",
			Detail: "status: " + strconv.Itoa(req.StatusCode) + " — refresh failed/expired",
		})
	}

	if len(flow.Steps) > 0 {
		m.Flows = append(m.Flows, flow)
	}
}

func analyzeCookie(m *Mapping, c *runtime.CookieRecord) {
	ac := AuthCookie{
		Name:   c.Name,
		Domain: c.Domain,
		Path:   c.Path,
		Secure: c.Secure,
		HttpOnly: c.HttpOnly,
		TokenType: "unknown",
	}

	if t, ok := lookupCookieType(c.Name); ok {
		ac.TokenType = t
	}

	if isLikelyToken(c.Value) {
		ac.Suspicious = true
	}

	m.Cookies = append(m.Cookies, ac)
}

func detectLoginFlow(m *Mapping) {
	var loginEndpoints []AuthEndpoint
	var callbackEndpoints []AuthEndpoint
	var tokenEndpoints []AuthEndpoint
	var hasAuthCookies bool
	var hasTokens bool

	for _, ep := range m.Endpoints {
		switch ep.EndpointType {
		case "login":
			loginEndpoints = append(loginEndpoints, ep)
		case "callback":
			callbackEndpoints = append(callbackEndpoints, ep)
		case "token":
			tokenEndpoints = append(tokenEndpoints, ep)
		}
	}

	for _, c := range m.Cookies {
		if c.TokenType != "unknown" || c.Suspicious {
			hasAuthCookies = true
			break
		}
	}

	hasTokens = len(m.JWTs) > 0

	if len(loginEndpoints) > 0 && (hasAuthCookies || hasTokens) {
		flow := Flow{Name: "login"}
		step := 1
		for _, ep := range loginEndpoints {
			flow.Steps = append(flow.Steps, FlowStep{
				Order:  step, Action: "request", URL: ep.URL,
				Method: ep.Method, Detail: "login request",
			})
			step++
			flow.Steps = append(flow.Steps, FlowStep{
				Order: step, Action: "response",
				Detail: "authentication response (cookies/tokens issued)",
			})
			step++
		}
		m.Flows = append(m.Flows, flow)
	}

	if len(loginEndpoints) > 0 && len(callbackEndpoints) > 0 {
		oauthFlow := Flow{Name: "oauth_login"}
		step := 1
		for _, ep := range loginEndpoints {
			oauthFlow.Steps = append(oauthFlow.Steps, FlowStep{
				Order: step, Action: "request", URL: ep.URL,
				Method: ep.Method, Detail: "initial login request",
			})
			step++
		}
		for _, ep := range callbackEndpoints {
			oauthFlow.Steps = append(oauthFlow.Steps, FlowStep{
				Order: step, Action: "redirect", URL: ep.URL,
				Method: ep.Method, Detail: "OAuth callback with authorization code",
			})
			step++
		}
		for _, ep := range tokenEndpoints {
			oauthFlow.Steps = append(oauthFlow.Steps, FlowStep{
				Order: step, Action: "request", URL: ep.URL,
				Method: ep.Method, Detail: "token exchange (code → tokens)",
			})
			step++
		}
		if hasTokens {
			oauthFlow.Steps = append(oauthFlow.Steps, FlowStep{
				Order: step, Action: "response",
				Detail: "access/refresh/id tokens issued",
			})
		}
		m.Flows = append(m.Flows, oauthFlow)
	}
}

func detectLogoutFlow(m *Mapping) {
	for _, ep := range m.Endpoints {
		if ep.EndpointType != "logout" {
			continue
		}
		flow := Flow{Name: "logout"}
		flow.Steps = append(flow.Steps, FlowStep{
			Order: 1, Action: "request", URL: ep.URL,
			Method: ep.Method, Detail: "logout request",
		})
		flow.Steps = append(flow.Steps, FlowStep{
			Order: 2, Action: "response",
			Detail: "session invalidated, cookies cleared",
		})
		hasCookieClears := false
		for _, c := range m.Cookies {
			if c.TokenType != "unknown" {
				hasCookieClears = true
				break
			}
		}
		if hasCookieClears {
			flow.Steps = append(flow.Steps, FlowStep{
				Order: 3, Action: "response",
				Detail: "auth cookies should be cleared client-side",
			})
		}
		m.Flows = append(m.Flows, flow)
	}
}

func detectOAuthFlowIfComplete(m *Mapping) {
	for _, of := range m.OAuthFlows {
		if of.Type == "" {
			continue
		}
		already := false
		for _, f := range m.Flows {
			if f.Name == "oauth_"+of.Type {
				already = true
				break
			}
		}
		if already {
			continue
		}
		flow := Flow{Name: "oauth_" + of.Type}
		step := 1
		if of.AuthEndpoint != "" {
			flow.Steps = append(flow.Steps, FlowStep{
				Order: step, Action: "request", URL: of.AuthEndpoint,
				Detail: "authorization request to " + of.Type + " flow",
			})
			step++
		}
		if of.RedirectURI != "" {
			flow.Steps = append(flow.Steps, FlowStep{
				Order: step, Action: "redirect",
				URL: of.RedirectURI, Detail: "user redirected back to app",
			})
			step++
		}
		if of.TokenEndpoint != "" {
			flow.Steps = append(flow.Steps, FlowStep{
				Order: step, Action: "request", URL: of.TokenEndpoint,
				Method: "POST", Detail: "token exchange",
			})
			step++
		}
		if of.PKCE {
			flow.Steps = append(flow.Steps, FlowStep{
				Order: step, Action: "script",
				Detail: "PKCE code_challenge/code_verifier generated",
			})
			step++
		}
		m.Flows = append(m.Flows, flow)
	}
}

func analyzeStorage(m *Mapping, source, key, value string, seenJWT map[string]bool) {
	if value == "" {
		return
	}

	tokenTypes := []string{"access_token", "refresh_token", "id_token", "token", "jwt"}
	isAuthKey := false
	detectedType := "unknown"

	lowerKey := strings.ToLower(key)
	for _, tt := range tokenTypes {
		if strings.Contains(lowerKey, tt) {
			isAuthKey = true
			detectedType = strings.ReplaceAll(tt, "_token", "")
			break
		}
	}

	if !isAuthKey && isLikelyToken(value) {
		isAuthKey = true
	}

	if !isAuthKey {
		return
	}

	st := StorageToken{
		Source:    source,
		Key:       key,
		Value:     value,
		TokenType: detectedType,
	}

	if j := parseJWTRaw(value); j != nil && !seenJWT[j.Token] {
		seenJWT[j.Token] = true
		j.Source = "storage"
		j.Location = source + "/" + key
		st.JWT = j
		if detectedType == "unknown" {
			st.TokenType = j.TokenType
		}
		m.JWTs = append(m.JWTs, *j)
	}

	m.StorageTokens = append(m.StorageTokens, st)
}
