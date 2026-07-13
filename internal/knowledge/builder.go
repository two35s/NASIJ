package knowledge

import (
	"crypto/sha256"
	"fmt"
	"net/url"
	"strings"

	"github.com/nasij/nasij/internal/authmapper"
	"github.com/nasij/nasij/internal/depintel"
	"github.com/nasij/nasij/internal/framework"
	"github.com/nasij/nasij/internal/runtime"
	"github.com/nasij/nasij/internal/secrets"
)

type Builder struct {
	graph     *Graph
	pageNodes map[string]*Node
}

func NewBuilder() *Builder {
	return &Builder{
		graph:     New(),
		pageNodes: make(map[string]*Node),
	}
}

func (b *Builder) Graph() *Graph {
	return b.graph
}

func (b *Builder) Build() *Graph {
	return b.graph
}

func nodeID(parts ...string) string {
	return strings.Join(parts, "::")
}

func sanitizeURL(u string) string {
	u = strings.TrimSuffix(u, "/")
	return u
}

func (b *Builder) ensurePage(pageURL string) *Node {
	pageURL = sanitizeURL(pageURL)
	id := nodeID("page", pageURL)
	if n := b.graph.GetNode(id); n != nil {
		return n
	}

	u, err := url.Parse(pageURL)
	domain := pageURL
	if err == nil {
		domain = u.Host
	}

	n := b.graph.AddNode(&Node{
		ID:    id,
		Type:  NodePage,
		Label: pageURL,
		Properties: map[string]any{
			"url":    pageURL,
			"domain": domain,
		},
	})
	b.pageNodes[pageURL] = n
	return n
}

func (b *Builder) ensureAPI(method, apiURL string) *Node {
	apiURL = sanitizeURL(apiURL)
	id := nodeID("api", method, apiURL)
	if n := b.graph.GetNode(id); n != nil {
		return n
	}
	return b.graph.AddNode(&Node{
		ID:    id,
		Type:  NodeAPIEndpoint,
		Label: fmt.Sprintf("%s %s", method, apiURL),
		Properties: map[string]any{
			"url":    apiURL,
			"method": method,
		},
	})
}

func (b *Builder) AddRuntimeResult(sourceURL string, rr *runtime.Result) *Builder {
	page := b.ensurePage(sourceURL)

	for _, req := range rr.Requests {
		reqURL := sanitizeURL(req.URL)
		if reqURL == sourceURL || reqURL == "" {
			continue
		}

		apiNode := b.ensureAPI(req.Method, reqURL)
		b.graph.AddEdge(&Edge{
			Type:   EdgeCalls,
			Source: page.ID,
			Target: apiNode.ID,
			Label:  fmt.Sprintf("%s %s", req.Method, req.ResourceType),
			Properties: map[string]any{
				"status_code":   req.StatusCode,
				"resource_type": req.ResourceType,
			},
		})
	}

	for _, ws := range rr.WebSockets {
		wsURL := sanitizeURL(ws.URL)
		apiNode := b.ensureAPI("WS", wsURL)
		b.graph.AddEdge(&Edge{
			Type:   EdgeCalls,
			Source: page.ID,
			Target: apiNode.ID,
			Label:  "WebSocket",
		})
	}

	for _, es := range rr.EventSources {
		esURL := sanitizeURL(es.URL)
		apiNode := b.ensureAPI("GET", esURL)
		b.graph.AddEdge(&Edge{
			Type:   EdgeCalls,
			Source: page.ID,
			Target: apiNode.ID,
			Label:  "EventSource",
		})
	}

	for _, c := range rr.Cookies {
		cookieID := nodeID("cookie", c.Name, c.Domain)
		cookieNode := b.graph.AddNode(&Node{
			ID:    cookieID,
			Type:  NodeCookie,
			Label: fmt.Sprintf("%s @ %s", c.Name, c.Domain),
			Properties: map[string]any{
				"name":     c.Name,
				"domain":   c.Domain,
				"path":     c.Path,
				"secure":   c.Secure,
				"httpOnly": c.HttpOnly,
			},
		})
		b.graph.AddEdge(&Edge{
			Type:   EdgeStores,
			Source: page.ID,
			Target: cookieNode.ID,
			Label:  fmt.Sprintf("sets cookie %s", c.Name),
		})
	}

	for _, ls := range rr.LocalStorage {
		storeID := nodeID("storage", "localStorage", ls.Key, sanitizeURL(sourceURL))
		storeNode := b.graph.AddNode(&Node{
			ID:    storeID,
			Type:  NodeStorage,
			Label: fmt.Sprintf("localStorage[%s]", ls.Key),
			Properties: map[string]any{
				"type":  "localStorage",
				"key":   ls.Key,
				"value": truncate(ls.Value, 100),
			},
		})
		b.graph.AddEdge(&Edge{
			Type:   EdgeStores,
			Source: page.ID,
			Target: storeNode.ID,
			Label:  fmt.Sprintf("localStorage[%s]", ls.Key),
		})
	}

	for _, ss := range rr.SessionStorage {
		storeID := nodeID("storage", "sessionStorage", ss.Key, sanitizeURL(sourceURL))
		storeNode := b.graph.AddNode(&Node{
			ID:    storeID,
			Type:  NodeStorage,
			Label: fmt.Sprintf("sessionStorage[%s]", ss.Key),
			Properties: map[string]any{
				"type":  "sessionStorage",
				"key":   ss.Key,
				"value": truncate(ss.Value, 100),
			},
		})
		b.graph.AddEdge(&Edge{
			Type:   EdgeStores,
			Source: page.ID,
			Target: storeNode.ID,
			Label:  fmt.Sprintf("sessionStorage[%s]", ss.Key),
		})
	}

	for _, idb := range rr.IndexedDB {
		idbID := nodeID("storage", "indexeddb", idb.Database, sanitizeURL(sourceURL))
		idbNode := b.graph.AddNode(&Node{
			ID:    idbID,
			Type:  NodeStorage,
			Label: fmt.Sprintf("IndexedDB[%s]", idb.Database),
			Properties: map[string]any{
				"type":     "indexeddb",
				"database": idb.Database,
				"version":  idb.Version,
			},
		})
		b.graph.AddEdge(&Edge{
			Type:   EdgeStores,
			Source: page.ID,
			Target: idbNode.ID,
			Label:  fmt.Sprintf("IndexedDB[%s]", idb.Database),
		})
	}

	for _, sw := range rr.ServiceWorkers {
		swID := nodeID("sw", sw.URL)
		swNode := b.graph.AddNode(&Node{
			ID:    swID,
			Type:  NodeServiceWorker,
			Label: fmt.Sprintf("SW: %s", sw.URL),
			Properties: map[string]any{
				"url":    sw.URL,
				"scope":  sw.Scope,
				"active": sw.Active,
			},
		})
		b.graph.AddEdge(&Edge{
			Type:   EdgeServes,
			Source: page.ID,
			Target: swNode.ID,
			Label:  sw.Scope,
		})
	}

	return b
}

func (b *Builder) AddFrameworkResult(pageURL string, fr *framework.Result) *Builder {
	pageURL = sanitizeURL(pageURL)
	page := b.ensurePage(pageURL)

	for _, fw := range fr.Frameworks {
		fwID := nodeID("framework", fw.Name)
		fwNode := b.graph.AddNode(&Node{
			ID:    fwID,
			Type:  NodeFramework,
			Label: fmt.Sprintf("%s v%s", fw.Name, fw.Version),
			Properties: map[string]any{
				"name":       fw.Name,
				"version":    fw.Version,
				"confidence": fw.Confidence,
			},
		})
		b.graph.AddEdge(&Edge{
			Type:   EdgeDetected,
			Source: page.ID,
			Target: fwNode.ID,
			Label:  fmt.Sprintf("confidence %.0f%%", fw.Confidence*100),
		})
	}

	return b
}

func (b *Builder) AddAuthMapping(pageURL string, am *authmapper.Mapping) *Builder {
	pageURL = sanitizeURL(pageURL)
	page := b.ensurePage(pageURL)

	for _, ep := range am.Endpoints {
		epID := nodeID("auth_endpoint", ep.EndpointType, ep.URL)
		epNode := b.graph.AddNode(&Node{
			ID:    epID,
			Type:  NodeAuthEndpoint,
			Label: fmt.Sprintf("%s %s", ep.Method, ep.URL),
			Properties: map[string]any{
				"url":           ep.URL,
				"method":        ep.Method,
				"endpoint_type": ep.EndpointType,
			},
		})

		apiNode := b.ensureAPI(ep.Method, ep.URL)
		b.graph.AddEdge(&Edge{
			Type:   EdgeReferences,
			Source: epNode.ID,
			Target: apiNode.ID,
			Label:  fmt.Sprintf("auth %s", ep.EndpointType),
		})

		b.graph.AddEdge(&Edge{
			Type:   EdgeContains,
			Source: page.ID,
			Target: epNode.ID,
			Label:  ep.EndpointType,
		})
	}

	for _, flow := range am.OAuthFlows {
		flowID := nodeID("auth_flow", flow.Type, flow.AuthEndpoint, flow.TokenEndpoint)
		flowNode := b.graph.AddNode(&Node{
			ID:    flowID,
			Type:  NodeAuthFlow,
			Label: fmt.Sprintf("OAuth %s", flow.Type),
			Properties: map[string]any{
				"flow_type":     flow.Type,
				"auth_endpoint": flow.AuthEndpoint,
				"token_endpoint": flow.TokenEndpoint,
				"pkce":          flow.PKCE,
				"oidc":          flow.OIDC,
			},
		})
		b.graph.AddEdge(&Edge{
			Type:   EdgeAuthenticates,
			Source: page.ID,
			Target: flowNode.ID,
			Label:  fmt.Sprintf("OAuth %s", flow.Type),
		})

		if flow.AuthEndpoint != "" {
			apiNode := b.ensureAPI("GET", flow.AuthEndpoint)
			b.graph.AddEdge(&Edge{
				Type:   EdgeReferences,
				Source: flowNode.ID,
				Target: apiNode.ID,
				Label:  "authorize endpoint",
			})
		}
		if flow.TokenEndpoint != "" {
			apiNode := b.ensureAPI("POST", flow.TokenEndpoint)
			b.graph.AddEdge(&Edge{
				Type:   EdgeReferences,
				Source: flowNode.ID,
				Target: apiNode.ID,
				Label:  "token endpoint",
			})
		}
	}

	for _, j := range am.JWTs {
		jwtID := nodeID("finding", "jwt", truncate(j.Token, 20))
		jwtNode := b.graph.AddNode(&Node{
			ID:    jwtID,
			Type:  NodeFinding,
			Label: fmt.Sprintf("JWT %s (%s)", j.TokenType, j.Subject),
			Properties: map[string]any{
				"finding_type": "jwt",
				"token_type":   j.TokenType,
				"algorithm":    j.Algorithm,
				"subject":      j.Subject,
				"issuer":       j.Issuer,
				"source":       j.Source,
				"location":     j.Location,
			},
		})
		b.graph.AddEdge(&Edge{
			Type:   EdgeHasFinding,
			Source: page.ID,
			Target: jwtNode.ID,
			Label:  fmt.Sprintf("JWT %s", j.TokenType),
		})
	}

	for _, c := range am.Cookies {
		cookieID := nodeID("cookie", c.Name, c.Domain)
		cookieNode := b.graph.AddNode(&Node{
			ID:    cookieID,
			Type:  NodeCookie,
			Label: fmt.Sprintf("%s @ %s", c.Name, c.Domain),
			Properties: map[string]any{
				"name":       c.Name,
				"domain":     c.Domain,
				"path":       c.Path,
				"secure":     c.Secure,
				"httpOnly":   c.HttpOnly,
				"same_site":  c.SameSite,
				"token_type": c.TokenType,
				"suspicious": c.Suspicious,
			},
		})
		b.graph.AddEdge(&Edge{
			Type:   EdgeStores,
			Source: page.ID,
			Target: cookieNode.ID,
			Label:  fmt.Sprintf("auth cookie %s", c.Name),
		})
	}

	for _, st := range am.StorageTokens {
		storeID := nodeID("storage", st.Source, st.Key)
		storeNode := b.graph.AddNode(&Node{
			ID:    storeID,
			Type:  NodeStorage,
			Label: fmt.Sprintf("%s[%s]", st.Source, st.Key),
			Properties: map[string]any{
				"type":       st.Source,
				"key":        st.Key,
				"token_type": st.TokenType,
			},
		})
		b.graph.AddEdge(&Edge{
			Type:   EdgeStores,
			Source: page.ID,
			Target: storeNode.ID,
			Label:  fmt.Sprintf("auth token in %s", st.Source),
		})
	}

	return b
}

func (b *Builder) AddSecretsResult(sr *secrets.ScanResult) *Builder {
	for _, f := range sr.Findings {
		findingID := nodeID("finding", string(f.SecretType), hashString(f.Match))
		findingNode := b.graph.AddNode(&Node{
			ID:    findingID,
			Type:  NodeFinding,
			Label: fmt.Sprintf("[%s] %s: %s", f.Provider, f.Key, truncate(f.Match, 40)),
			Properties: map[string]any{
				"finding_type": string(f.SecretType),
				"severity":     f.Severity.String(),
				"provider":     f.Provider,
				"key":          f.Key,
				"match":        truncate(f.Match, 200),
				"source":       f.Source,
				"entropy":      f.Entropy,
			},
		})

		pageURL := sr.Target
		if pageURL == "" {
			pageURL = f.Source
		}
		if pageURL != "" {
			pid := nodeID("page", sanitizeURL(pageURL))
			if !b.graph.HasNode(pid) {
				b.ensurePage(pageURL)
			}
			b.graph.AddEdge(&Edge{
				Type:   EdgeHasFinding,
				Source: pid,
				Target: findingNode.ID,
				Label:  fmt.Sprintf("secret: %s", f.Key),
			})
		}
	}

	return b
}

func (b *Builder) AddDepResult(dr *depintel.DepResult) *Builder {
	for _, pkg := range dr.Packages {
		depID := nodeID("dep", string(pkg.Ecosystem), pkg.Name)
		depNode := b.graph.AddNode(&Node{
			ID:    depID,
			Type:  NodeDependency,
			Label: fmt.Sprintf("%s @ %s", pkg.Name, pkg.Version),
			Properties: map[string]any{
				"ecosystem": string(pkg.Ecosystem),
				"name":      pkg.Name,
				"version":   pkg.Version,
				"source":    pkg.Source,
			},
		})

		_ = depNode

		if pkg.Version != "" {
			for _, vuln := range dr.Vulnerabilities {
				if vuln.AffectedPackage == pkg.Name {
					vulnID := nodeID("vuln", vuln.ID)
					vulnNode := b.graph.AddNode(&Node{
						ID:    vulnID,
						Type:  NodeVulnerability,
						Label: fmt.Sprintf("%s: %s", vuln.ID, truncate(vuln.Summary, 60)),
						Properties: map[string]any{
							"cve_id":         vuln.ID,
							"summary":        vuln.Summary,
							"severity":       vuln.Severity.String(),
							"fixed_version":  vuln.FixedVersion,
							"affected_pkg":   vuln.AffectedPackage,
							"affected_ver":   vuln.AffectedVersion,
							"source":         vuln.Source,
						},
					})
					b.graph.AddEdge(&Edge{
						Type:   EdgeHasFinding,
						Source: depID,
						Target: vulnNode.ID,
						Label:  fmt.Sprintf("vulnerable: %s", vuln.ID),
					})
				}
			}
		}
	}

	for _, vuln := range dr.Vulnerabilities {
		vulnID := nodeID("vuln", vuln.ID)
		if b.graph.HasNode(vulnID) {
			continue
		}
		b.graph.AddNode(&Node{
			ID:    vulnID,
			Type:  NodeVulnerability,
			Label: fmt.Sprintf("%s: %s", vuln.ID, truncate(vuln.Summary, 60)),
			Properties: map[string]any{
				"cve_id":         vuln.ID,
				"summary":        vuln.Summary,
				"severity":       vuln.Severity.String(),
				"fixed_version":  vuln.FixedVersion,
				"affected_pkg":   vuln.AffectedPackage,
				"affected_ver":   vuln.AffectedVersion,
				"source":         vuln.Source,
			},
		})
	}

	return b
}

func hashString(s string) string {
	h := sha256.Sum256([]byte(s))
	return fmt.Sprintf("%x", h[:8])
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
