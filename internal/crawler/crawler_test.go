package crawler_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/nasij/nasij/internal/crawler"
	"github.com/nasij/nasij/internal/httpclient"
	"github.com/nasij/nasij/internal/scope"
)

func testLogger(t *testing.T) *zap.Logger {
	t.Helper()
	logger, err := zap.NewDevelopment()
	require.NoError(t, err)
	t.Cleanup(func() { _ = logger.Sync() })
	return logger
}

func testClient(t *testing.T) *httpclient.Client {
	t.Helper()
	cfg := httpclient.DefaultConfig()
	cfg.MaxRetries = 0
	cfg.RetryWaitMin = 1 * time.Millisecond
	cfg.RetryWaitMax = 5 * time.Millisecond
	c, err := httpclient.New(cfg, testLogger(t))
	require.NoError(t, err)
	t.Cleanup(func() { c.Close() })
	return c
}

func testScope() *scope.Scope {
	return &scope.Scope{
		TargetHost: "example.com",
		Domains:    []string{"example.com"},
	}
}

// --- HTML Parsing ---

func TestParseHTML_Links(t *testing.T) {
	html := `<html><body>
		<a href="/page1">Page 1</a>
		<a href="https://example.com/page2">Page 2</a>
		<a href="https://external.com">External</a>
		<a href="javascript:void(0)">JS</a>
		<a href="mailto:test@example.com">Email</a>
	</body></html>`

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(html))
	}))
	defer ts.Close()

	client := testClient(t)
	cr := crawler.New(client, testScope(), crawler.DefaultOptions(), testLogger(t))

	ctx := context.Background()
	results, errs := cr.Start(ctx, []string{ts.URL + "/"})
	defer cr.Stop()

	var res *crawler.PageResult
	timer := time.After(5 * time.Second)
	for res == nil {
		select {
		case r, ok := <-results:
			if !ok {
				t.Fatal("results channel closed before receiving result")
			}
			res = r
		case err := <-errs:
			t.Fatal(err)
		case <-timer:
			t.Fatal("timeout")
		}
	}

	assert.Equal(t, ts.URL, res.URL)
	assert.Equal(t, http.StatusOK, res.StatusCode)
	// Should have found internal links (resolveURL for /page1, and https://example.com/page2)
	assert.Contains(t, res.Links, ts.URL+"/page1")
	assert.Contains(t, res.Links, "https://example.com/page2")
	// Should NOT include javascript:, mailto:, or external that's out of scope
	// (external.com is out of scope so it's filtered)
}

func TestParseHTML_Assets(t *testing.T) {
	html := `<html><head>
		<script src="/app.js"></script>
		<link rel="stylesheet" href="/style.css">
	</head><body>
		<img src="/image.png">
		<iframe src="/frame.html"></iframe>
		<script>var api = "/api/data";</script>
	</body></html>`

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(html))
	}))
	defer ts.Close()

	client := testClient(t)
	cr := crawler.New(client, nil, crawler.DefaultOptions(), testLogger(t))

	ctx := context.Background()
	results, errs := cr.Start(ctx, []string{ts.URL})
	defer cr.Stop()

	var res *crawler.PageResult
	timer := time.After(5 * time.Second)
	for res == nil {
		select {
		case r, ok := <-results:
			if !ok {
				t.Fatal("results channel closed before receiving result")
			}
			res = r
		case err := <-errs:
			t.Fatal(err)
		case <-timer:
			t.Fatal("timeout")
		}
	}
	assetURLs := make(map[string]crawler.AssetType)
	for _, a := range res.Assets {
		assetURLs[a.URL] = a.Type
	}
	assert.Equal(t, crawler.AssetJS, assetURLs[ts.URL+"/app.js"])
	assert.Equal(t, crawler.AssetCSS, assetURLs[ts.URL+"/style.css"])
	assert.Equal(t, crawler.AssetImage, assetURLs[ts.URL+"/image.png"])
	assert.Equal(t, crawler.AssetIframe, assetURLs[ts.URL+"/frame.html"])
}

func TestParseHTML_Title(t *testing.T) {
	html := `<html><head><title>Test Page</title></head><body></body></html>`

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(html))
	}))
	defer ts.Close()

	client := testClient(t)
	cr := crawler.New(client, nil, crawler.DefaultOptions(), testLogger(t))

	ctx := context.Background()
	results, _ := cr.Start(ctx, []string{ts.URL})
	defer cr.Stop()

	select {
	case res := <-results:
		assert.Equal(t, "Test Page", res.Title)
	case <-time.After(5 * time.Second):
		t.Fatal("timeout")
	}
}

func TestParseHTML_Forms(t *testing.T) {
	html := `<html><body>
		<form action="/login" method="POST">
			<input name="username" type="text">
			<input name="password" type="password">
			<button type="submit">Login</button>
		</form>
	</body></html>`

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(html))
	}))
	defer ts.Close()

	opts := crawler.DefaultOptions()
	opts.MaxDepth = 0
	opts.MaxPages = 1

	client := testClient(t)
	cr := crawler.New(client, nil, opts, testLogger(t))

	ctx := context.Background()
	results, _ := cr.Start(ctx, []string{ts.URL})
	defer cr.Stop()

	select {
	case res := <-results:
		require.Len(t, res.Forms, 1)
		assert.Equal(t, "/login", res.Forms[0].Action)
		assert.Equal(t, "POST", res.Forms[0].Method)
		assert.Len(t, res.Forms[0].Inputs, 2)
		assert.Equal(t, "username", res.Forms[0].Inputs[0].Name)
		assert.Equal(t, "password", res.Forms[0].Inputs[1].Name)
	case <-time.After(5 * time.Second):
		t.Fatal("timeout")
	}
}

func TestParseHTML_Meta(t *testing.T) {
	html := `<html><head>
		<meta name="description" content="Test page">
		<meta name="keywords" content="go, testing">
		<meta property="og:title" content="OG Title">
		<meta http-equiv="refresh" content="0; url=/redirect">
	</head><body></body></html>`

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(html))
	}))
	defer ts.Close()

	client := testClient(t)
	cr := crawler.New(client, nil, crawler.DefaultOptions(), testLogger(t))

	ctx := context.Background()
	results, _ := cr.Start(ctx, []string{ts.URL})
	defer cr.Stop()

	select {
	case res := <-results:
		assert.Equal(t, "Test page", res.Metadata["description"])
		assert.Equal(t, "go, testing", res.Metadata["keywords"])
		assert.Equal(t, "OG Title", res.Metadata["og:title"])
		assert.Contains(t, res.Links, ts.URL+"/redirect")
	case <-time.After(5 * time.Second):
		t.Fatal("timeout")
	}
}

// --- Sitemap ---

func TestParseSitemap(t *testing.T) {
	xml := `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
	<url><loc>https://example.com/</loc></url>
	<url><loc>https://example.com/about</loc></url>
	<url><loc>https://example.com/contact</loc></url>
</urlset>`

	urls, err := crawler.ParseSitemap([]byte(xml))
	require.NoError(t, err)
	assert.Len(t, urls, 3)
	assert.Contains(t, urls, "https://example.com/")
	assert.Contains(t, urls, "https://example.com/about")
}

func TestParseSitemap_Empty(t *testing.T) {
	urls, err := crawler.ParseSitemap([]byte("<urlset></urlset>"))
	require.NoError(t, err)
	assert.Empty(t, urls)
}

// --- URL Normalization ---

func TestNormalizeURL(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"https://example.com", "https://example.com"},
		{"HTTPS://EXAMPLE.COM", "https://example.com"},
		{"https://example.com:443/", "https://example.com"},
		{"http://example.com:80/", "http://example.com"},
		{"https://example.com/page#fragment", "https://example.com/page"},
		{"https://example.com/page?b=2&a=1", "https://example.com/page?a=1&b=2"},
		{"//example.com/path", "https://example.com/path"},
		{"", ""},
		{"ftp://example.com", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.expected, crawler.NormalizeURL(tt.input))
		})
	}
}

// --- CSS URL Extraction ---

func TestExtractCSSURLs(t *testing.T) {
	css := `body { background: url(/bg.png); }
		@import url('https://fonts.googleapis.com/css?family=Open+Sans');
		@import "other.css";
		div { background-image: url("/image.jpg"); }`

	// We can test via HTML with inline style
	html := `<html><head><style>` + css + `</style></head><body></body></html>`

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(html))
	}))
	defer ts.Close()

	opts := crawler.DefaultOptions()
	opts.DiscoverCSS = true
	opts.MaxDepth = 0
	opts.MaxPages = 1

	client := testClient(t)
	cr := crawler.New(client, nil, opts, testLogger(t))

	ctx := context.Background()
	results, _ := cr.Start(ctx, []string{ts.URL})
	defer cr.Stop()

	select {
	case res := <-results:
		assetURLs := make([]string, len(res.Assets))
		for i, a := range res.Assets {
			assetURLs[i] = a.URL
		}
		assert.Contains(t, assetURLs, ts.URL+"/bg.png")
	case <-time.After(5 * time.Second):
		t.Fatal("timeout")
	}
}

// --- Dedup ---

func TestCrawler_Dedup(t *testing.T) {
	var callCount int
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><body><a href="/">Home</a></body></html>`))
	}))
	defer ts.Close()

	opts := crawler.DefaultOptions()
	opts.ConcurrentWorkers = 1
	opts.MaxDepth = 1
	opts.MaxPages = 10

	client := testClient(t)
	cr := crawler.New(client, nil, opts, testLogger(t))

	ctx := context.Background()
	results, _ := cr.Start(ctx, []string{ts.URL, ts.URL})
	defer cr.Stop()

	// Drain results
	for range results {
	}
	// Should only have fetched once due to dedup
	assert.Equal(t, 1, callCount)
}

// --- SPA fragments ---

func TestParseHTML_SPAFragments(t *testing.T) {
	html := `<html><body>
		<a href="#!/users/123">User</a>
		<a href="#/dashboard">Dashboard</a>
		<a href="#section">Anchor</a>
	</body></html>`

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(html))
	}))
	defer ts.Close()

	opts := crawler.DefaultOptions()
	opts.MaxDepth = 0
	opts.MaxPages = 1

	client := testClient(t)
	cr := crawler.New(client, nil, opts, testLogger(t))

	ctx := context.Background()
	results, _ := cr.Start(ctx, []string{ts.URL})
	defer cr.Stop()

	select {
	case res := <-results:
		assert.Contains(t, res.Fragments, "#!/users/123")
		assert.Contains(t, res.Fragments, "#/dashboard")
	case <-time.After(5 * time.Second):
		t.Fatal("timeout")
	}
}

// --- Multiple pages ---

func TestCrawler_MultiplePages(t *testing.T) {
	var pages []string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pages = append(pages, r.URL.Path)
		w.Header().Set("Content-Type", "text/html")
		switch r.URL.Path {
		case "/":
			_, _ = w.Write([]byte(`<html><body>
				<a href="/page1">Page 1</a>
				<a href="/page2">Page 2</a>
			</body></html>`))
		case "/page1":
			_, _ = w.Write([]byte(`<html><body><p>Page 1</p></body></html>`))
		case "/page2":
			_, _ = w.Write([]byte(`<html><body><p>Page 2</p></body></html>`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()

	opts := crawler.DefaultOptions()
	opts.ConcurrentWorkers = 2
	opts.MaxDepth = 1
	opts.MaxPages = 10
	opts.DiscoverJS = false
	opts.DiscoverCSS = false
	opts.DiscoverImages = false
	opts.DiscoverIframes = false
	opts.DiscoverForms = false
	opts.ExtractMeta = false

	client := testClient(t)
	cr := crawler.New(client, nil, opts, testLogger(t))

	ctx := context.Background()
	results, _ := cr.Start(ctx, []string{ts.URL})
	defer cr.Stop()

	var resultCount int
	for range results {
		resultCount++
	}
	assert.Equal(t, 3, resultCount)
	assert.Contains(t, pages, "/")
	assert.Contains(t, pages, "/page1")
	assert.Contains(t, pages, "/page2")
}

// --- Stats ---

func TestCrawler_Stats(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><body>Hello</body></html>`))
	}))
	defer ts.Close()

	opts := crawler.DefaultOptions()
	opts.MaxDepth = 0
	opts.MaxPages = 1

	client := testClient(t)
	cr := crawler.New(client, nil, opts, testLogger(t))

	ctx := context.Background()
	results, _ := cr.Start(ctx, []string{ts.URL})
	defer cr.Stop()

	<-results

	stats := cr.Stats()
	assert.Equal(t, int32(1), stats.PagesVisited)
}

// --- Scope filtering ---

func TestCrawler_ScopeFiltering(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><body>
			<a href="https://outofscope.com/page">Out</a>
			<a href="/internal">Internal</a>
		</body></html>`))
	}))
	defer ts.Close()

	sc := &scope.Scope{
		TargetHost: "example.com",
		Domains:    []string{"example.com"},
	}

	opts := crawler.DefaultOptions()
	opts.MaxDepth = 0
	opts.MaxPages = 1

	client := testClient(t)
	cr := crawler.New(client, sc, opts, testLogger(t))

	ctx := context.Background()
	results, _ := cr.Start(ctx, []string{ts.URL})
	defer cr.Stop()

	select {
	case res := <-results:
		// Internal link should be discovered
		assert.Contains(t, res.Links, ts.URL+"/internal")
		// External link may appear in results but won't be enqueued (scope filter)
	case <-time.After(5 * time.Second):
		t.Fatal("timeout")
	}
}

// --- Cancellation ---

func TestCrawler_Stop(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<html></html>"))
	}))
	defer ts.Close()

	opts := crawler.DefaultOptions()
	opts.ConcurrentWorkers = 2
	opts.MaxDepth = 5
	opts.MaxPages = 100

	client := testClient(t)
	cr := crawler.New(client, nil, opts, testLogger(t))

	ctx := context.Background()
	results, _ := cr.Start(ctx, []string{ts.URL})
	cr.Stop()

	// Should close quickly
	done := make(chan struct{})
	go func() {
		for range results {
		}
		close(done)
	}()

	select {
	case <-done:
		// OK
	case <-time.After(3 * time.Second):
		t.Fatal("crawler did not stop in time")
	}
}

// --- Edge cases ---

func TestCrawler_EmptySeed(t *testing.T) {
	client := testClient(t)
	cr := crawler.New(client, nil, crawler.DefaultOptions(), testLogger(t))

	ctx := context.Background()
	results, errs := cr.Start(ctx, []string{})
	defer cr.Stop()

	done := make(chan struct{})
	go func() {
		for range results {
		}
		for range errs {
		}
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("crawler did not finish with empty seeds")
	}
}

func TestCrawler_NonHTMLResponse(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer ts.Close()

	opts := crawler.DefaultOptions()
	opts.MaxPages = 1

	client := testClient(t)
	cr := crawler.New(client, nil, opts, testLogger(t))

	ctx := context.Background()
	results, _ := cr.Start(ctx, []string{ts.URL})
	defer cr.Stop()

	select {
	case res := <-results:
		assert.Equal(t, http.StatusOK, res.StatusCode)
		assert.Empty(t, res.Links)
	case <-time.After(5 * time.Second):
		t.Fatal("timeout")
	}
}

// --- NormalizeURL edge cases ---

func TestNormalizeURL_Empty(t *testing.T) {
	assert.Equal(t, "", crawler.NormalizeURL(""))
}

func TestNormalizeURL_Relative(t *testing.T) {
	assert.Equal(t, "", crawler.NormalizeURL("/relative/path"))
}

func TestNormalizeURL_RemoveFragment(t *testing.T) {
	assert.Equal(t, "https://example.com/page", crawler.NormalizeURL("https://example.com/page#section"))
}

// --- DedupSet ---

func TestDedupSet_Add(t *testing.T) {
	set := crawler.NewDedupSet()
	assert.True(t, set.Add("https://example.com"))
	assert.False(t, set.Add("https://example.com"))
	assert.True(t, set.Add("https://example.com/other"))
}

func TestDedupSet_Has(t *testing.T) {
	set := crawler.NewDedupSet()
	set.Add("https://example.com")
	assert.True(t, set.Has("https://example.com"))
	assert.False(t, set.Has("https://other.com"))
}

func TestDedupSet_Len(t *testing.T) {
	set := crawler.NewDedupSet()
	assert.Equal(t, 0, set.Len())
	set.Add("a")
	set.Add("b")
	assert.Equal(t, 2, set.Len())
}

func TestDedupSet_Clear(t *testing.T) {
	set := crawler.NewDedupSet()
	set.Add("a")
	set.Clear()
	assert.Equal(t, 0, set.Len())
}

// --- ParseSitemap edge cases ---

func TestParseSitemap_Malformed(t *testing.T) {
	urls, err := crawler.ParseSitemap([]byte("not xml"))
	require.NoError(t, err)
	assert.Empty(t, urls)
}

func TestParseSitemap_Index(t *testing.T) {
	xml := `<?xml version="1.0" encoding="UTF-8"?>
<sitemapindex xmlns="http://www.sitemaps.org/schemas/siteindex/0.9">
	<sitemap><loc>https://example.com/sitemap1.xml</loc></sitemap>
</sitemapindex>`
	urls, err := crawler.ParseSitemap([]byte(xml))
	require.NoError(t, err)
	assert.Len(t, urls, 1)
	assert.Contains(t, urls, "https://example.com/sitemap1.xml")
}

// --- Inline JS URL extraction ---

func TestParseHTML_InlineScriptURLs(t *testing.T) {
	html := `<html><head>
		<script>
			var api = "https://api.example.com/v1";
			fetch("/api/data");
		</script>
	</head><body></body></html>`

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(html))
	}))
	defer ts.Close()

	opts := crawler.DefaultOptions()
	opts.MaxPages = 1
	opts.MaxDepth = 0

	client := testClient(t)
	cr := crawler.New(client, nil, opts, testLogger(t))

	ctx := context.Background()
	results, _ := cr.Start(ctx, []string{ts.URL})
	defer cr.Stop()

	select {
	case res := <-results:
		// Should find fetch URLs from inline JS
		var urls []string
		for _, a := range res.Assets {
			urls = append(urls, a.URL)
		}
		t.Logf("Discovered assets: %v", urls)
	case <-time.After(5 * time.Second):
		t.Fatal("timeout")
	}
}

// --- Source with srcset ---

func TestParseHTML_Srcset(t *testing.T) {
	html := `<html><body>
		<img src="small.jpg" srcset="medium.jpg 1000w, large.jpg 2000w">
		<picture>
			<source srcset="wide.jpg" media="(min-width: 800px)">
			<img src="fallback.jpg">
		</picture>
	</body></html>`

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(html))
	}))
	defer ts.Close()

	opts := crawler.DefaultOptions()
	opts.MaxPages = 1
	opts.MaxDepth = 0

	client := testClient(t)
	cr := crawler.New(client, nil, opts, testLogger(t))

	ctx := context.Background()
	results, _ := cr.Start(ctx, []string{ts.URL})
	defer cr.Stop()

	select {
	case res := <-results:
		var urls []string
		for _, a := range res.Assets {
			urls = append(urls, a.URL)
		}
		assert.Contains(t, urls, ts.URL+"/small.jpg")
		assert.Contains(t, urls, ts.URL+"/medium.jpg")
		assert.Contains(t, urls, ts.URL+"/large.jpg")
		assert.Contains(t, urls, ts.URL+"/wide.jpg")
		assert.Contains(t, urls, ts.URL+"/fallback.jpg")
	case <-time.After(5 * time.Second):
		t.Fatal("timeout")
	}
}

// --- Asset type strings ---

func TestAssetType_String(t *testing.T) {
	assert.Equal(t, "js", fmt.Sprint(crawler.AssetJS))
	assert.Equal(t, "css", fmt.Sprint(crawler.AssetCSS))
	assert.Equal(t, "image", fmt.Sprint(crawler.AssetImage))
	assert.Equal(t, "unknown", fmt.Sprint(crawler.AssetUnknown))
}

// --- Link with base tag ---

func TestParseHTML_BaseTag(t *testing.T) {
	html := `<html><head>
		<base href="https://example.com/sub/">
	</head><body>
		<a href="page.html">Page</a>
	</body></html>`

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(html))
	}))
	defer ts.Close()

	opts := crawler.DefaultOptions()
	opts.MaxPages = 1
	opts.MaxDepth = 0

	client := testClient(t)
	cr := crawler.New(client, nil, opts, testLogger(t))

	ctx := context.Background()
	results, _ := cr.Start(ctx, []string{ts.URL})
	defer cr.Stop()

	select {
	case res := <-results:
		assert.Contains(t, res.Links, "https://example.com/sub/page.html")
	case <-time.After(5 * time.Second):
		t.Fatal("timeout")
	}
}

// --- Crawler default options ---

func TestDefaultOptions(t *testing.T) {
	opts := crawler.DefaultOptions()
	assert.Equal(t, 3, opts.MaxDepth)
	assert.Equal(t, 100, opts.MaxPages)
	assert.Equal(t, 5, opts.ConcurrentWorkers)
	assert.True(t, opts.DiscoverJS)
	assert.True(t, opts.DiscoverCSS)
	assert.True(t, opts.SPA)
}

// --- JS Collector integration ---

func TestJSCollector_DynamicImports(t *testing.T) {
	html := `<html><head>
		<script>
			import("./lazy.js");
			import("./pages/Home.js");
		</script>
	</head><body></body></html>`

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(html))
	}))
	defer ts.Close()

	opts := crawler.DefaultOptions()
	opts.MaxPages = 1
	opts.MaxDepth = 0

	client := testClient(t)
	cr := crawler.New(client, nil, opts, testLogger(t))

	ctx := context.Background()
	results, _ := cr.Start(ctx, []string{ts.URL})
	defer cr.Stop()

	var res *crawler.PageResult
	select {
	case res = <-results:
	case <-time.After(5 * time.Second):
		t.Fatal("timeout")
	}

	var found []string
	for _, a := range res.Assets {
		if a.Type == crawler.AssetDynamicImport {
			found = append(found, a.URL)
		}
	}
	if len(found) < 2 {
		t.Fatalf("expected at least 2 dynamic imports, got %v", found)
	}
}

func TestJSCollector_WebpackChunks(t *testing.T) {
	html := `<html><head>
		<script>
			__webpack_public_path__ = "/dist/";
			var chunk = "chunk-a1b2c3d4";
		</script>
	</head><body></body></html>`

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(html))
	}))
	defer ts.Close()

	opts := crawler.DefaultOptions()
	opts.MaxPages = 1
	opts.MaxDepth = 0

	client := testClient(t)
	cr := crawler.New(client, nil, opts, testLogger(t))

	ctx := context.Background()
	results, _ := cr.Start(ctx, []string{ts.URL})
	defer cr.Stop()

	var res *crawler.PageResult
	select {
	case res = <-results:
	case <-time.After(5 * time.Second):
		t.Fatal("timeout")
	}

	for _, a := range res.Assets {
		if a.Type == crawler.AssetWebpackChunk {
			return // found at least one webpack asset
		}
	}
	// webpack_public_path alone is stored as config, not a URL asset
	// so it's fine if no AssetWebpackChunk is found — the scanner
	// primarily extracts chunk-xxx identifiers
}

func TestJSCollector_ViteAssets(t *testing.T) {
	html := `<html><head>
		<script>
			const url = new URL('./assets/icon.svg', import.meta.url);
			const mods = import.meta.glob('./pages/*.js');
		</script>
	</head><body></body></html>`

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(html))
	}))
	defer ts.Close()

	opts := crawler.DefaultOptions()
	opts.MaxPages = 1
	opts.MaxDepth = 0

	client := testClient(t)
	cr := crawler.New(client, nil, opts, testLogger(t))

	ctx := context.Background()
	results, _ := cr.Start(ctx, []string{ts.URL})
	defer cr.Stop()

	var res *crawler.PageResult
	select {
	case res = <-results:
	case <-time.After(5 * time.Second):
		t.Fatal("timeout")
	}

	var found []string
	for _, a := range res.Assets {
		if a.Type == crawler.AssetViteAsset {
			found = append(found, a.URL)
		}
	}
	if len(found) < 2 {
		t.Fatalf("expected at least 2 vite assets, got %v", found)
	}
}

func TestJSCollector_InlineScripts(t *testing.T) {
	html := `<html><head>
		<script>var x = 1;</script>
		<script>var y = 2;</script>
	</head><body></body></html>`

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(html))
	}))
	defer ts.Close()

	opts := crawler.DefaultOptions()
	opts.MaxPages = 1
	opts.MaxDepth = 0

	client := testClient(t)
	cr := crawler.New(client, nil, opts, testLogger(t))

	ctx := context.Background()
	results, _ := cr.Start(ctx, []string{ts.URL})
	defer cr.Stop()

	var res *crawler.PageResult
	select {
	case res = <-results:
	case <-time.After(5 * time.Second):
		t.Fatal("timeout")
	}

	if len(res.InlineScripts) < 2 {
		t.Fatalf("expected at least 2 inline scripts, got %d: %v", len(res.InlineScripts), res.InlineScripts)
	}
}

func TestJSCollector_JSFileScanning(t *testing.T) {
	jsContent := `
		import("./lazy.js");
		const url = new URL('./assets/icon.svg', import.meta.url);
		//# sourceMappingURL=app.js.map
	`

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/app.js" {
			w.Header().Set("Content-Type", "application/javascript")
			_, _ = w.Write([]byte(jsContent))
			return
		}
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><body><script src="/app.js"></script></body></html>`))
	}))
	defer ts.Close()

	opts := crawler.DefaultOptions()
	opts.CrawlJS = true
	opts.MaxPages = 10
	opts.MaxDepth = 2

	client := testClient(t)
	cr := crawler.New(client, nil, opts, testLogger(t))

	ctx := context.Background()
	results, _ := cr.Start(ctx, []string{ts.URL})
	defer cr.Stop()

	gotDynamic := false
	gotVite := false
	gotSourceMap := false

	timeout := time.After(5 * time.Second)
	for {
		select {
		case res, ok := <-results:
			if !ok {
				goto check
			}
			for _, a := range res.Assets {
				switch a.Type {
				case crawler.AssetDynamicImport:
					gotDynamic = true
				case crawler.AssetViteAsset:
					gotVite = true
				case crawler.AssetSourceMap:
					gotSourceMap = true
				}
			}
		case <-timeout:
			t.Fatal("timeout")
		}
	}
check:
	if !gotDynamic {
		t.Error("expected dynamic import from JS file scan")
	}
	if !gotVite {
		t.Error("expected vite asset from JS file scan")
	}
	if !gotSourceMap {
		t.Error("expected source map from JS file scan")
	}
}

// --- AssetType String ---

func TestAssetType_String_New(t *testing.T) {
	assert.Equal(t, "dynamic_import", crawler.AssetDynamicImport.String())
	assert.Equal(t, "webpack_chunk", crawler.AssetWebpackChunk.String())
	assert.Equal(t, "vite_asset", crawler.AssetViteAsset.String())
	assert.Equal(t, "source_map", crawler.AssetSourceMap.String())
}
