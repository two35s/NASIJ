package crawler

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"

	"github.com/nasij/nasij/internal/httpclient"
	"github.com/nasij/nasij/internal/jscollect"
	"github.com/nasij/nasij/internal/scope"
)

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

// AssetType represents the type of a discovered asset.
type AssetType int

const (
	AssetUnknown  AssetType = iota
	AssetJS
	AssetCSS
	AssetImage
	AssetIframe
	AssetFont
	AssetVideo
	AssetAudio
	AssetJSON
	AssetFetch
	AssetForm
	AssetDynamicImport
	AssetWebpackChunk
	AssetViteAsset
	AssetSourceMap
)

func (a AssetType) String() string {
	switch a {
	case AssetJS:
		return "js"
	case AssetCSS:
		return "css"
	case AssetImage:
		return "image"
	case AssetIframe:
		return "iframe"
	case AssetFont:
		return "font"
	case AssetVideo:
		return "video"
	case AssetAudio:
		return "audio"
	case AssetJSON:
		return "json"
	case AssetFetch:
		return "fetch"
	case AssetForm:
		return "form"
	case AssetDynamicImport:
		return "dynamic_import"
	case AssetWebpackChunk:
		return "webpack_chunk"
	case AssetViteAsset:
		return "vite_asset"
	case AssetSourceMap:
		return "source_map"
	default:
		return "unknown"
	}
}

// Asset represents a discovered resource.
type Asset struct {
	URL  string    `json:"url"`
	Type AssetType `json:"type"`
}

// PageResult holds all data extracted from a single crawled page.
type PageResult struct {
	URL        string            `json:"url"`
	StatusCode int               `json:"status_code"`
	Depth      int               `json:"depth"`
	Title      string            `json:"title,omitempty"`
	Links      []string          `json:"links,omitempty"`      // internal discovered links
	Assets     []Asset           `json:"assets,omitempty"`     // external resources
	Forms      []Form            `json:"forms,omitempty"`
	Metadata   map[string]string `json:"metadata,omitempty"`  // meta tags
	Fragments  []string          `json:"fragments,omitempty"`  // SPA hash routes
	InlineScripts []string       `json:"inline_scripts,omitempty"` // raw inline JS content
	Size       int64             `json:"size"`
	FetchTime  time.Duration     `json:"fetch_time"`
}

// Form represents an HTML form discovered during crawling.
type Form struct {
	Action string            `json:"action"`
	Method string            `json:"method"` // GET, POST
	Inputs []FormInput       `json:"inputs,omitempty"`
	Attrs  map[string]string `json:"attrs,omitempty"`
}

type FormInput struct {
	Name  string `json:"name"`
	Type  string `json:"type"`
	Value string `json:"value,omitempty"`
}

// Options controls crawler behavior.
type Options struct {
	// MaxDepth is the maximum crawl depth (0 = unlimited).
	MaxDepth int

	// MaxPages is the maximum number of pages to crawl (0 = unlimited).
	MaxPages int

	// ConcurrentWorkers is the number of concurrent fetch workers.
	ConcurrentWorkers int

	// RequestDelay is the minimum delay between requests to the same host.
	RequestDelay time.Duration

	// RespectRobots disallows crawling paths blocked by robots.txt.
	RespectRobots bool

	// FollowSitemap discovers and crawls sitemap URLs.
	FollowSitemap bool

	// DiscoverJS enables extracting JavaScript files and inline scripts.
	DiscoverJS bool

	// CrawlJS enables crawling JavaScript file URLs (inline script src).
	CrawlJS bool

	// DiscoverCSS enables extracting CSS files and inline styles.
	DiscoverCSS bool

	// DiscoverImages enables extracting image URLs.
	DiscoverImages bool

	// DiscoverIframes enables extracting iframe sources.
	DiscoverIframes bool

	// DiscoverForms enables extracting form endpoints.
	DiscoverForms bool

	// SPA enables SPA-specific discovery (hash fragments, pushState patterns).
	SPA bool

	// ExtractMeta extracts meta tag content.
	ExtractMeta bool
}

// DefaultOptions returns sensible defaults for web reconnaissance.
func DefaultOptions() Options {
	return Options{
		ConcurrentWorkers: 5,
		MaxDepth:          3,
		MaxPages:          100,
		RequestDelay:      0,
		RespectRobots:     true,
		FollowSitemap:     true,
		DiscoverJS:        true,
		CrawlJS:           false,
		DiscoverCSS:       true,
		DiscoverImages:    true,
		DiscoverIframes:   true,
		DiscoverForms:     true,
		SPA:               true,
		ExtractMeta:       true,
	}
}

// ---------------------------------------------------------------------------
// URL Queue
// ---------------------------------------------------------------------------

type urlItem struct {
	url   string
	depth int
}

type urlQueue struct {
	mu    sync.Mutex
	items []urlItem
}

func newURLQueue() *urlQueue {
	return &urlQueue{}
}

func (q *urlQueue) Push(rawurl string, depth int) {
	q.mu.Lock()
	q.items = append(q.items, urlItem{url: rawurl, depth: depth})
	q.mu.Unlock()
}

func (q *urlQueue) Pop() (string, int, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.items) == 0 {
		return "", 0, false
	}
	item := q.items[0]
	q.items = q.items[1:]
	return item.url, item.depth, true
}

func (q *urlQueue) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.items)
}

// ---------------------------------------------------------------------------
// Crawler
// ---------------------------------------------------------------------------

// Crawler orchestrates web crawling with concurrent workers.
type Crawler struct {
	client  *httpclient.Client
	scope   *scope.Scope
	opts    Options
	logger  *zap.Logger
	robots  *scope.RobotsTxt
	scopeMu sync.RWMutex
	jsScanner *jscollect.Scanner

	// State
	queue    *urlQueue
	visited  *DedupSet
	results  chan *PageResult
	errors   chan error
	active   int32 // atomic count of active workers
	pending  int32 // atomic count of in-flight URL processing
	stopped  atomic.Bool
	started  time.Time
	pagesCrawled int32

	// Stats
	stats CrawlStats

	// For resolving relative URLs
	baseURL *url.URL

	// Cancel
	cancel context.CancelFunc
	ctx    context.Context
}

// CrawlStats holds real-time crawling statistics.
type CrawlStats struct {
	PagesVisited int32
	LinksFound   int32
	AssetsFound  int32
	Errors       int32
	BytesFetched int64
}

// New creates a new crawler.
func New(client *httpclient.Client, sc *scope.Scope, opts Options, logger *zap.Logger) *Crawler {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Crawler{
		client:    client,
		scope:     sc,
		opts:      opts,
		logger:    logger,
		queue:     newURLQueue(),
		visited:   newDedupSet(),
		results:   make(chan *PageResult, 1000),
		errors:    make(chan error, 100),
		jsScanner: jscollect.New(),
	}
}

func (c *Crawler) SetRobots(rt *scope.RobotsTxt) {
	c.scopeMu.Lock()
	c.robots = rt
	c.scopeMu.Unlock()
}

func (c *Crawler) SetScope(sc *scope.Scope) {
	c.scopeMu.Lock()
	c.scope = sc
	c.scopeMu.Unlock()
}

// Start begins crawling from the given seed URLs.
// Returns a channel of PageResults and a channel of errors.
// The caller must drain both channels until done is closed.
func (c *Crawler) Start(ctx context.Context, seeds []string) (<-chan *PageResult, <-chan error) {
	c.ctx, c.cancel = context.WithCancel(ctx)
	c.started = time.Now()

	// Enqueue seeds
	for _, seed := range seeds {
		normalized := NormalizeURL(seed)
		if normalized != "" {
			c.queue.Push(normalized, 0)
		}
	}

	// Parse base URL from first seed
	if len(seeds) > 0 {
		c.baseURL, _ = url.Parse(seeds[0])
	}

	// Start workers
	workerCount := c.opts.ConcurrentWorkers
	if workerCount <= 0 {
		workerCount = 5
	}

	// Set active before starting workers so the monitor can't outrun them
	atomic.StoreInt32(&c.active, int32(workerCount))
	for i := 0; i < workerCount; i++ {
		go c.worker(i)
	}

	// Monitor for completion
	go c.monitor()

	return c.results, c.errors
}

// Stop signals all workers to stop after their current request.
func (c *Crawler) Stop() {
	c.stopped.Store(true)
	if c.cancel != nil {
		c.cancel()
	}
}

// Stats returns a snapshot of current crawl statistics.
func (c *Crawler) Stats() CrawlStats {
	return CrawlStats{
		PagesVisited: atomic.LoadInt32(&c.stats.PagesVisited),
		LinksFound:   atomic.LoadInt32(&c.stats.LinksFound),
		AssetsFound:  atomic.LoadInt32(&c.stats.AssetsFound),
		Errors:       atomic.LoadInt32(&c.stats.Errors),
		BytesFetched: atomic.LoadInt64(&c.stats.BytesFetched),
	}
}

// Done returns a channel that closes when all workers have exited.
func (c *Crawler) Done() <-chan struct{} {
	done := make(chan struct{})
	go func() {
		for atomic.LoadInt32(&c.active) > 0 {
			time.Sleep(100 * time.Millisecond)
		}
		close(done)
	}()
	return done
}

// ---------------------------------------------------------------------------
// Workers
// ---------------------------------------------------------------------------

func (c *Crawler) worker(id int) {
	defer atomic.AddInt32(&c.active, -1)

	c.logger.Debug("crawler worker started", zap.Int("worker_id", id))

	// Track consecutive empty polls to detect idle shutdown
	emptyPolls := 0

	for {
		if c.stopped.Load() {
			return
		}

		// Check limits
		maxPages := c.opts.MaxPages
		if maxPages > 0 && atomic.LoadInt32(&c.pagesCrawled) >= int32(maxPages) {
			return
		}

		rawurl, depth, ok := c.queue.Pop()
		if !ok {
			emptyPolls++
			if emptyPolls >= 10 && atomic.LoadInt32(&c.pending) == 0 {
				return
			}
			time.Sleep(100 * time.Millisecond)
			continue
		}
		emptyPolls = 0

		// Depth check
		if c.opts.MaxDepth > 0 && depth > c.opts.MaxDepth {
			continue
		}

		// Dedup
		if !c.visited.Add(rawurl) {
			continue
		}

		atomic.AddInt32(&c.pending, 1)
		c.processURL(rawurl, depth)
		atomic.AddInt32(&c.pending, -1)
	}
}

func (c *Crawler) processURL(rawurl string, depth int) {
	c.logger.Debug("crawling", zap.String("url", rawurl), zap.Int("depth", depth))
	atomic.AddInt32(&c.pagesCrawled, 1)

	start := time.Now()

	req, err := http.NewRequestWithContext(c.ctx, http.MethodGet, rawurl, nil)
	if err != nil {
		atomic.AddInt32(&c.stats.Errors, 1)
		c.logger.Warn("bad URL", zap.String("url", rawurl), zap.Error(err))
		c.safeSendError(fmt.Errorf("bad URL %s: %w", rawurl, err))
		return
	}
	resp, err := c.client.Do(req)
	if err != nil {
		atomic.AddInt32(&c.stats.Errors, 1)
		c.logger.Warn("fetch failed", zap.String("url", rawurl), zap.Error(err))
		c.safeSendError(fmt.Errorf("fetch %s: %w", rawurl, err))
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		atomic.AddInt32(&c.stats.Errors, 1)
		return
	}

	atomic.AddInt64(&c.stats.BytesFetched, int64(len(body)))
	contentType := resp.Header.Get("Content-Type")

	result := &PageResult{
		URL:        rawurl,
		StatusCode: resp.StatusCode,
		Depth:      depth,
		Size:       int64(len(body)),
		FetchTime:  time.Since(start),
	}

	// Parse based on content type
	if strings.Contains(contentType, "text/html") || strings.Contains(contentType, "application/xhtml") {
		c.parseHTML(body, rawurl, depth, result)
	} else if strings.Contains(contentType, "text/css") {
		c.parseCSS(body, rawurl, result)
	} else if strings.Contains(contentType, "application/json") || strings.Contains(contentType, "text/json") {
		result.Assets = append(result.Assets, Asset{URL: rawurl, Type: AssetJSON})
	} else if strings.Contains(contentType, "javascript") || strings.Contains(contentType, "ecmascript") {
		c.parseJS(body, rawurl, result)
	}

	atomic.AddInt32(&c.stats.PagesVisited, 1)

	c.safeSendResult(result)

	// Enqueue discovered links
	for _, link := range result.Links {
		normalized := NormalizeURL(link)
		if normalized == "" {
			continue
		}
		c.scopeMu.RLock()
		inScope := c.scope == nil || c.scope.IsInScope(normalized)
		c.scopeMu.RUnlock()
		if !inScope {
			continue
		}
		atomic.AddInt32(&c.stats.LinksFound, 1)
		c.queue.Push(normalized, depth+1)
	}

	// Track assets
	atomic.AddInt32(&c.stats.AssetsFound, int32(len(result.Assets)))
}

// ---------------------------------------------------------------------------
// Parser
// ---------------------------------------------------------------------------

func (c *Crawler) parseHTML(body []byte, rawurl string, depth int, result *PageResult) {
	base, _ := url.Parse(rawurl)

	doc, err := html.Parse(strings.NewReader(string(body)))
	if err != nil {
		c.logger.Warn("HTML parse error", zap.String("url", rawurl), zap.Error(err))
		return
	}

	var crawl func(*html.Node)
	crawl = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch n.DataAtom {
			case atom.A:
				href := getAttr(n, "href")
				if href != "" {
					if abs := resolveURL(base, href); abs != "" {
						result.Links = append(result.Links, abs)
					}
					// SPA hash fragments in href
					if c.opts.SPA && strings.HasPrefix(href, "#") && len(href) > 1 {
						if strings.HasPrefix(href[1:], "!/") || strings.HasPrefix(href[1:], "/") {
							result.Fragments = append(result.Fragments, href)
						}
					}
				}

			case atom.Img:
				if src := getAttr(n, "src"); src != "" {
					if abs := resolveURL(base, src); abs != "" {
						result.Assets = append(result.Assets, Asset{URL: abs, Type: AssetImage})
					}
				}
				if srcset := getAttr(n, "srcset"); srcset != "" {
					for _, part := range strings.Split(srcset, ",") {
						part = strings.TrimSpace(part)
						if idx := strings.IndexAny(part, " \t"); idx > 0 {
							part = part[:idx]
						}
						if abs := resolveURL(base, part); abs != "" {
							result.Assets = append(result.Assets, Asset{URL: abs, Type: AssetImage})
						}
					}
				}

			case atom.Script:
				if src := getAttr(n, "src"); src != "" {
					if abs := resolveURL(base, src); abs != "" {
						result.Assets = append(result.Assets, Asset{URL: abs, Type: AssetJS})
						if c.opts.CrawlJS {
							result.Links = append(result.Links, abs)
						}
					}
				} else if c.opts.DiscoverJS && n.FirstChild != nil {
					// Store inline script content
					code := n.FirstChild.Data
					result.InlineScripts = append(result.InlineScripts, code)

					// Scan inline JS for URLs and patterns
					sr := c.jsScanner.Scan(code, rawurl)
					for _, u := range sr.ScriptURLs {
						if abs := resolveURL(base, u); abs != "" {
							result.Assets = append(result.Assets, Asset{URL: abs, Type: AssetFetch})
						}
					}
					c.addJSAssets(result, base, sr)
				}

			case atom.Link:
				rel := strings.ToLower(getAttr(n, "rel"))
				href := getAttr(n, "href")
				if href == "" {
					break
				}
				switch rel {
				case "stylesheet":
					if abs := resolveURL(base, href); abs != "" {
						result.Assets = append(result.Assets, Asset{URL: abs, Type: AssetCSS})
					}
				case "preload", "prefetch", "dns-prefetch":
					if abs := resolveURL(base, href); abs != "" {
						as := strings.ToLower(getAttr(n, "as"))
						assetType := assetTypeFromPreload(as)
						result.Assets = append(result.Assets, Asset{URL: abs, Type: assetType})
					}
				case "modulepreload":
					if abs := resolveURL(base, href); abs != "" {
						result.Assets = append(result.Assets, Asset{URL: abs, Type: AssetJS})
					}
				case "icon", "apple-touch-icon", "shortcut icon":
					if abs := resolveURL(base, href); abs != "" {
						result.Assets = append(result.Assets, Asset{URL: abs, Type: AssetImage})
					}
				case "canonical", "alternate", "next", "prev":
					if abs := resolveURL(base, href); abs != "" {
						result.Links = append(result.Links, abs)
					}
				case "manifest":
					if abs := resolveURL(base, href); abs != "" {
						result.Assets = append(result.Assets, Asset{URL: abs, Type: AssetJSON})
					}
				}

			case atom.Iframe, atom.Frame:
				if src := getAttr(n, "src"); src != "" {
					if abs := resolveURL(base, src); abs != "" {
						result.Links = append(result.Links, abs)
						result.Assets = append(result.Assets, Asset{URL: abs, Type: AssetIframe})
					}
				}

			case atom.Form:
				action := getAttr(n, "action")
				method := strings.ToUpper(getAttr(n, "method"))
				if method == "" {
					method = "GET"
				}
				f := Form{
					Action: action,
					Method: method,
					Attrs:  make(map[string]string),
				}
				// Collect form attributes
				for _, attr := range n.Attr {
					if attr.Key == "action" || attr.Key == "method" {
						continue
					}
					f.Attrs[attr.Key] = attr.Val
				}
				// Find inputs
				collectInputs(n, &f)
				result.Forms = append(result.Forms, f)
				if action != "" {
					if abs := resolveURL(base, action); abs != "" {
						result.Links = append(result.Links, abs)
					}
				}

			case atom.Source:
				if src := getAttr(n, "src"); src != "" {
					if abs := resolveURL(base, src); abs != "" {
						result.Assets = append(result.Assets, Asset{URL: abs, Type: AssetVideo})
					}
				}
				if srcset := getAttr(n, "srcset"); srcset != "" {
					for _, part := range strings.Split(srcset, ",") {
						part = strings.TrimSpace(part)
						if idx := strings.IndexAny(part, " \t"); idx > 0 {
							part = part[:idx]
						}
						if abs := resolveURL(base, part); abs != "" {
							result.Assets = append(result.Assets, Asset{URL: abs, Type: AssetImage})
						}
					}
				}

			case atom.Video, atom.Audio:
				if src := getAttr(n, "src"); src != "" {
					at := AssetVideo
					if n.DataAtom == atom.Audio {
						at = AssetAudio
					}
					if abs := resolveURL(base, src); abs != "" {
						result.Assets = append(result.Assets, Asset{URL: abs, Type: at})
					}
				}

			case atom.Meta:
				if c.opts.ExtractMeta {
					name := getAttr(n, "name")
					prop := getAttr(n, "property")
					content := getAttr(n, "content")
					httpEquiv := strings.ToLower(getAttr(n, "http-equiv"))

					key := name
					if key == "" {
						key = prop
					}
					if key != "" && content != "" {
						if result.Metadata == nil {
							result.Metadata = make(map[string]string)
						}
						result.Metadata[key] = content
					}

					// Meta refresh redirect
					if httpEquiv == "refresh" && content != "" {
						if idx := strings.LastIndex(content, "url="); idx >= 0 {
							redirectURL := strings.TrimSpace(content[idx+4:])
							if abs := resolveURL(base, redirectURL); abs != "" {
								result.Links = append(result.Links, abs)
							}
						}
					}
				}

			case atom.Title:
				if n.FirstChild != nil {
					result.Title = strings.TrimSpace(n.FirstChild.Data)
				}

			case atom.Base:
				if href := getAttr(n, "href"); href != "" {
					if newBase, err := url.Parse(href); err == nil {
						base = base.ResolveReference(newBase)
					}
				}

			case atom.Style:
				if c.opts.DiscoverCSS && n.FirstChild != nil {
					cssURLs := extractURLsFromCSS(n.FirstChild.Data)
					for _, u := range cssURLs {
						if abs := resolveURL(base, u); abs != "" {
							result.Assets = append(result.Assets, Asset{URL: abs, Type: AssetCSS})
						}
					}
				}
			}

			// Event handlers (SPA detection)
			if c.opts.SPA {
				for _, attr := range n.Attr {
					if isEventAttr(attr.Key) && strings.TrimSpace(attr.Val) != "" {
						jsURLs := extractURLsFromJS(attr.Val)
						for _, u := range jsURLs {
							if abs := resolveURL(base, u); abs != "" {
								result.Assets = append(result.Assets, Asset{URL: abs, Type: AssetFetch})
							}
						}
						break
					}
				}
			}
		}

		// SPA hash fragments
		if c.opts.SPA && n.Type == html.TextNode {
			frags := extractHashFragments(n.Data)
			result.Fragments = append(result.Fragments, frags...)
		}

		for child := n.FirstChild; child != nil; child = child.NextSibling {
			crawl(child)
		}
	}

	crawl(doc)

	// Deduplicate
	result.Links = uniqueStrings(result.Links)
	result.Assets = uniqueAssets(result.Assets)
}

func (c *Crawler) parseCSS(body []byte, rawurl string, result *PageResult) {
	urls := extractURLsFromCSS(string(body))
	base, _ := url.Parse(rawurl)
	for _, u := range urls {
		if abs := resolveURL(base, u); abs != "" {
			result.Assets = append(result.Assets, Asset{URL: abs, Type: AssetCSS})
		}
	}
}

func (c *Crawler) parseJS(body []byte, rawurl string, result *PageResult) {
	base, _ := url.Parse(rawurl)
	sr := c.jsScanner.Scan(string(body), rawurl)
	c.addJSAssets(result, base, sr)
}

func (c *Crawler) addJSAssets(result *PageResult, base *url.URL, sr *jscollect.ScanResult) {
	// Dynamic imports
	for _, spec := range sr.DynamicImports {
		if abs := resolveURL(base, spec); abs != "" {
			result.Assets = append(result.Assets, Asset{URL: abs, Type: AssetDynamicImport})
		}
	}

	// Webpack chunks
	for _, ref := range sr.WebpackChunks {
		if strings.HasPrefix(ref, "/") || strings.HasPrefix(ref, "./") || strings.HasPrefix(ref, "../") ||
			strings.HasPrefix(ref, "http://") || strings.HasPrefix(ref, "https://") || strings.HasPrefix(ref, "//") {
			if abs := resolveURL(base, ref); abs != "" {
				result.Assets = append(result.Assets, Asset{URL: abs, Type: AssetWebpackChunk})
			}
		}
	}

	// Vite assets
	for _, asset := range sr.ViteAssets {
		if abs := resolveURL(base, asset); abs != "" {
			result.Assets = append(result.Assets, Asset{URL: abs, Type: AssetViteAsset})
		}
	}

	// Source maps
	for _, sm := range sr.SourceMaps {
		if sm.Inline {
			continue
		}
		if abs := resolveURL(base, sm.URL); abs != "" {
			result.Assets = append(result.Assets, Asset{URL: abs, Type: AssetSourceMap})
		}
	}
}

// ---------------------------------------------------------------------------
// Sitemap parser
// ---------------------------------------------------------------------------

// ParseSitemap parses a sitemap.xml body and returns discovered URLs.
func ParseSitemap(body []byte) ([]string, error) {
	var urls []string

	// Simple XML parsing for sitemap format
	content := string(body)
	for {
		start := strings.Index(content, "<loc>")
		if start < 0 {
			break
		}
		start += 5
		end := strings.Index(content[start:], "</loc>")
		if end < 0 {
			break
		}
		url := strings.TrimSpace(content[start : start+end])
		if url != "" {
			urls = append(urls, url)
		}
		content = content[start+end:]
	}

	return urls, nil
}

// FetchSitemap fetches and parses /sitemap.xml from the base URL.
func (c *Crawler) FetchSitemap(ctx context.Context, baseURL string) ([]string, error) {
	u, err := url.Parse(baseURL)
	if err != nil {
		return nil, err
	}

	// Common sitemap locations
	locations := []string{
		u.Scheme + "://" + u.Host + "/sitemap.xml",
		u.Scheme + "://" + u.Host + "/sitemap_index.xml",
		u.Scheme + "://" + u.Host + "/sitemap/",
	}

	for _, loc := range locations {
		resp, err := c.client.Get(loc)
		if err != nil {
			continue
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			continue
		}
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			continue
		}

		urls, err := ParseSitemap(body)
		if err != nil {
			continue
		}
		if len(urls) > 0 {
			return urls, nil
		}
	}

	return nil, nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func getAttr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if strings.EqualFold(a.Key, key) {
			return a.Val
		}
	}
	return ""
}

func resolveURL(base *url.URL, href string) string {
	if href == "" || strings.HasPrefix(href, "javascript:") || strings.HasPrefix(href, "mailto:") ||
		strings.HasPrefix(href, "tel:") || strings.HasPrefix(href, "data:") || strings.HasPrefix(href, "blob:") {
		return ""
	}
	u, err := url.Parse(href)
	if err != nil {
		return ""
	}
	if base != nil {
		u = base.ResolveReference(u)
	}
	return u.String()
}

func collectInputs(n *html.Node, f *Form) {
	if n.Type == html.ElementNode && n.DataAtom == atom.Input {
		input := FormInput{
			Name:  getAttr(n, "name"),
			Type:  getAttr(n, "type"),
			Value: getAttr(n, "value"),
		}
		if input.Name != "" || input.Type != "" {
			f.Inputs = append(f.Inputs, input)
		}
	}
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		collectInputs(child, f)
	}
}

func isEventAttr(key string) bool {
	eventAttrs := []string{
		"onclick", "ondblclick", "onchange", "onsubmit",
		"onload", "onerror", "onfocus", "onblur",
		"onkeydown", "onkeyup", "onkeypress",
		"onmousedown", "onmouseup", "onmousemove",
		"onmouseover", "onmouseout",
		"onscroll", "onresize",
		"oninput", "onsearch",
		"onpointerdown", "onpointerup",
		"ontouchstart", "ontouchend",
	}
	lower := strings.ToLower(key)
	for _, e := range eventAttrs {
		if lower == e {
			return true
		}
	}
	return false
}

func assetTypeFromPreload(as string) AssetType {
	switch strings.ToLower(as) {
	case "script":
		return AssetJS
	case "style":
		return AssetCSS
	case "image":
		return AssetImage
	case "font":
		return AssetFont
	case "video":
		return AssetVideo
	case "audio":
		return AssetAudio
	case "fetch":
		return AssetFetch
	default:
		return AssetUnknown
	}
}

func uniqueStrings(slice []string) []string {
	if len(slice) == 0 {
		return slice
	}
	seen := make(map[string]struct{}, len(slice))
	result := make([]string, 0, len(slice))
	for _, s := range slice {
		if _, ok := seen[s]; !ok {
			seen[s] = struct{}{}
			result = append(result, s)
		}
	}
	return result
}

func uniqueAssets(slice []Asset) []Asset {
	if len(slice) == 0 {
		return slice
	}
	seen := make(map[string]AssetType, len(slice))
	result := make([]Asset, 0, len(slice))
	for _, a := range slice {
		if existing, ok := seen[a.URL]; ok && existing == a.Type {
			continue
		}
		seen[a.URL] = a.Type
		result = append(result, a)
	}
	return result
}

// ---------------------------------------------------------------------------
// URL extraction helpers
// ---------------------------------------------------------------------------

// extractURLsFromCSS parses CSS text and returns URLs found in url() references.
func extractURLsFromCSS(css string) []string {
	var urls []string
	remaining := css
	for {
		idx := strings.Index(remaining, "url(")
		if idx < 0 {
			break
		}
		remaining = remaining[idx+4:]
		end := strings.IndexAny(remaining, ")")
		if end < 0 {
			break
		}
		url := strings.TrimSpace(remaining[:end])
		url = strings.Trim(url, "\"' \t")
		if url != "" && !strings.HasPrefix(url, "#") {
			urls = append(urls, url)
		}
		remaining = remaining[end+1:]
	}

	// @import "..." or @import '...'
	remaining = css
	for {
		idx := strings.Index(remaining, "@import")
		if idx < 0 {
			break
		}
		remaining = remaining[idx+7:]
		// Skip whitespace
		remaining = strings.TrimSpace(remaining)
		if len(remaining) == 0 {
			break
		}
		if remaining[0] == '"' || remaining[0] == '\'' {
			quote := remaining[0]
			end := strings.Index(remaining[1:], string(quote))
			if end < 0 {
				break
			}
			url := remaining[1 : end+1]
			if url != "" {
				urls = append(urls, url)
			}
			remaining = remaining[end+2:]
		} else {
			end := strings.IndexAny(remaining, ";{")
			if end < 0 {
				break
			}
			remaining = remaining[end:]
		}
	}

	return urls
}

// extractURLsFromJS looks for string literals that look like URLs in JS code.
func extractURLsFromJS(js string) []string {
	var urls []string
	seen := make(map[string]struct{})

	// Find quoted strings that look like URLs
	for _, quote := range []string{"\"", "'", "`"} {
		remaining := js
		for {
			idx := strings.Index(remaining, quote)
			if idx < 0 {
				break
			}
			remaining = remaining[idx+1:]
			end := strings.Index(remaining, quote)
			if end < 0 {
				break
			}
			str := remaining[:end]
			remaining = remaining[end+1:]

			str = strings.TrimSpace(str)
			if isLikelyURL(str) {
				if _, exists := seen[str]; !exists {
					seen[str] = struct{}{}
					urls = append(urls, str)
				}
			}
		}
	}

	return urls
}

// extractHashFragments finds SPA hash fragments in text.
func extractHashFragments(text string) []string {
	var fragments []string
	seen := make(map[string]struct{})

	remaining := text
	for {
		idx := strings.Index(remaining, "#")
		if idx < 0 {
			break
		}
		remaining = remaining[idx+1:]
		end := strings.IndexAny(remaining, " \t\n\r\"'<>")
		if end < 0 {
			end = len(remaining)
		}
		if end > 0 {
			frag := remaining[:end]
			if strings.HasPrefix(frag, "!/") || strings.HasPrefix(frag, "/") {
				full := "#" + frag
				if _, exists := seen[full]; !exists {
					seen[full] = struct{}{}
					fragments = append(fragments, full)
				}
			}
		}
	}

	return fragments
}

func isLikelyURL(s string) bool {
	if len(s) < 4 {
		return false
	}
	if strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://") {
		return true
	}
	if strings.HasPrefix(s, "//") && len(s) > 4 {
		return true
	}
	if strings.HasPrefix(s, "/") && strings.Contains(s[1:], ".") && !strings.Contains(s, " ") {
		return true
	}
	if strings.Count(s, ".") >= 1 && strings.Contains(s, "/") && !strings.Contains(s, " ") {
		return true
	}
	return false
}

// NormalizeURL normalizes a URL for consistent deduplication.
func NormalizeURL(rawurl string) string {
	if rawurl == "" {
		return ""
	}

	// Handle protocol-relative URLs
	if strings.HasPrefix(rawurl, "//") {
		rawurl = "https:" + rawurl
	}

	u, err := url.Parse(rawurl)
	if err != nil {
		return ""
	}

	if u.Scheme == "" {
		u.Scheme = "https"
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return ""
	}

	// Lowercase scheme and host
	u.Scheme = strings.ToLower(u.Scheme)
	u.Host = strings.ToLower(u.Host)

	// Reject URLs without a host
	if u.Host == "" {
		return ""
	}

	// Remove default ports
	if (u.Scheme == "http" && u.Port() == "80") || (u.Scheme == "https" && u.Port() == "443") {
		host := u.Hostname()
		if strings.Contains(u.Host, ":") {
			u.Host = host
		}
	}

	// Remove fragments
	u.Fragment = ""

	// Sort query parameters
	if u.RawQuery != "" {
		params := u.Query()
		if len(params) > 0 {
			keys := make([]string, 0, len(params))
			for k := range params {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			values := url.Values{}
			for _, k := range keys {
				values[k] = params[k]
			}
			u.RawQuery = values.Encode()
		}
	}

	// Remove trailing slash from path
	path := u.Path
	if path == "/" {
		u.Path = ""
	} else if strings.HasSuffix(path, "/") {
		u.Path = strings.TrimSuffix(path, "/")
	}

	return u.String()
}

// ---------------------------------------------------------------------------
// Logging
// ---------------------------------------------------------------------------

// SetLogger sets the crawler's logger. Not goroutine-safe; call before Start.
func (c *Crawler) SetLogger(logger *zap.Logger) {
	if logger != nil {
		c.logger = logger
	}
}

// safeSendResult sends a result to the results channel, recovering from panic
// if the channel has been closed.
func (c *Crawler) safeSendResult(result *PageResult) {
	defer func() { recover() }()
	select {
	case c.results <- result:
	default:
	}
}

// safeSendError sends an error to the errors channel, recovering from panic
// if the channel has been closed.
func (c *Crawler) safeSendError(err error) {
	defer func() { recover() }()
	select {
	case c.errors <- err:
	default:
	}
}

// monitor waits for completion and closes result/error channels.
func (c *Crawler) monitor() {
	<-c.Done()
	close(c.results)
	close(c.errors)
}


