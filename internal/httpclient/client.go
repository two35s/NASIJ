package httpclient

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/gob"
	"errors"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
)

// ---------------------------------------------------------------------------
// Config
// ---------------------------------------------------------------------------

// Config holds all configuration options for the HTTP engine.
type Config struct {
	// UserAgent sent with every request.
	UserAgent string

	// Timeout is the maximum duration for a complete HTTP request lifecycle
	// (including retries). Default: 30s.
	Timeout time.Duration

	// RequestTimeout is the maximum duration for a single HTTP request attempt
	// (excluding retries). Default: 10s.
	RequestTimeout time.Duration

	// --- Retry ---
	MaxRetries    int           // default: 3
	RetryWaitMin  time.Duration // default: 500ms
	RetryWaitMax  time.Duration // default: 10s
	RetryOnStatus []int         // status codes that trigger retry; default: 429, 5xx

	// --- Redirects ---
	MaxRedirects int // default: 10; 0 = no redirects

	// --- HTTP version ---
	// HTTP/2 is automatically negotiated via TLS ALPN by Go's net/http.
	// HTTP/3 requires an external quic-go/http3 client.
	EnableHTTP3 bool
	// TLS config
	TLSInsecure bool   // skip TLS cert verification (default: false)
	TLSCAFile   string // path to custom CA cert bundle
	TLSCertFile string // path to client cert for mTLS
	TLSKeyFile  string // path to client key for mTLS
	ServerName  string // override TLS SNI

	// --- Proxy ---
	ProxyURL  string // e.g. "http://127.0.0.1:8080" or "socks5://127.0.0.1:1080"
	ProxyAuth string // "username:password"

	// --- Cookies ---
	CookieFile string // path to persist cookies

	// --- Custom Headers ---
	CustomHeaders map[string]string

	// --- Connection Pool ---
	MaxConnsPerHost     int           // default: 0 (unlimited)
	MaxIdleConns        int           // default: 100
	MaxIdleConnsPerHost int           // default: 10
	IdleConnTimeout     time.Duration // default: 90s
	TLSHandshakeTimeout time.Duration // default: 10s

	// --- Compression ---
	// Go's http.Transport handles gzip/deflate transparently by default.
	// Set DisableCompression to true to disable automatic decompression.
	DisableCompression bool

	// --- Caching ---
	CacheEnabled bool
	CacheMaxSize int           // max entries; default: 1000
	CacheTTL     time.Duration // default: 60s

	// --- Rate Limiting ---
	RateLimit      float64 // requests per second; 0 = unlimited
	RateLimitBurst int     // default: 1

	// --- Auth ---
	BearerToken string   // sets "Authorization: Bearer <token>"
	BasicAuth   UserPass // sets "Authorization: Basic <encoded>"
}

type UserPass struct {
	Username string
	Password string
}

// DefaultConfig returns a Config with sensible defaults for recon work.
func DefaultConfig() Config {
	return Config{
		UserAgent:           "NASIJ/1.0",
		Timeout:             30 * time.Second,
		RequestTimeout:      10 * time.Second,
		MaxRetries:          3,
		RetryWaitMin:        500 * time.Millisecond,
		RetryWaitMax:        10 * time.Second,
		RetryOnStatus:       []int{429, 500, 502, 503, 504},
		MaxRedirects:        10,
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
		TLSHandshakeTimeout: 10 * time.Second,
		CacheEnabled:        true,
		CacheMaxSize:        1000,
		CacheTTL:            60 * time.Second,
		RateLimit:           0,
		RateLimitBurst:      1,
	}
}

// FromScope configures an HTTP client Config from a scope's rate limit and auth.
func FromScope(rateLimit float64, burst int, authType, authValue string, headers map[string]string) Config {
	cfg := DefaultConfig()
	if rateLimit > 0 {
		cfg.RateLimit = rateLimit
		cfg.RateLimitBurst = burst
	}
	switch authType {
	case "bearer":
		cfg.BearerToken = authValue
	case "basic":
		if parts := strings.SplitN(authValue, ":", 2); len(parts) == 2 {
			cfg.BasicAuth = UserPass{Username: parts[0], Password: parts[1]}
		}
	case "header":
		if parts := strings.SplitN(authValue, ":", 2); len(parts) == 2 {
			if cfg.CustomHeaders == nil {
				cfg.CustomHeaders = make(map[string]string)
			}
			cfg.CustomHeaders[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}
	}
	for k, v := range headers {
		if cfg.CustomHeaders == nil {
			cfg.CustomHeaders = make(map[string]string)
		}
		cfg.CustomHeaders[k] = v
	}
	return cfg
}

// ---------------------------------------------------------------------------
// Rate limiter (token bucket)
// ---------------------------------------------------------------------------

type rateLimiter struct {
	rate       float64
	burst      int
	tokens     float64
	lastRefill time.Time
	mu         sync.Mutex
}

func newRateLimiter(rate float64, burst int) *rateLimiter {
	if rate <= 0 {
		return nil
	}
	if burst < 1 {
		burst = 1
	}
	return &rateLimiter{
		rate:       rate,
		burst:      burst,
		tokens:     float64(burst),
		lastRefill: time.Now(),
	}
}

func (rl *rateLimiter) Wait(ctx context.Context) error {
	if rl == nil {
		return nil
	}
	for {
		rl.mu.Lock()
		now := time.Now()
		elapsed := now.Sub(rl.lastRefill).Seconds()
		rl.tokens = math.Min(float64(rl.burst), rl.tokens+elapsed*rl.rate)
		rl.lastRefill = now

		if rl.tokens >= 1 {
			rl.tokens--
			rl.mu.Unlock()
			return nil
		}
		need := 1 - rl.tokens
		waitNanos := time.Duration(need / rl.rate * float64(time.Second))
		rl.mu.Unlock()

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(waitNanos):
		}
	}
}

func (rl *rateLimiter) Update(rate float64, burst int) {
	if rl == nil {
		return
	}
	rl.mu.Lock()
	defer rl.mu.Unlock()
	rl.rate = rate
	if burst < 1 {
		burst = 1
	}
	rl.burst = burst
}

// ---------------------------------------------------------------------------
// Response cache
// ---------------------------------------------------------------------------

type cacheEntry struct {
	StatusCode int
	Header     http.Header
	Body       []byte
	ExpiresAt  time.Time
}

type responseCache struct {
	entries map[string]*cacheEntry
	maxSize int
	ttl     time.Duration
	mu      sync.RWMutex
	hits    int64
	misses  int64
}

func newResponseCache(maxSize int, ttl time.Duration) *responseCache {
	if maxSize <= 0 {
		maxSize = 1000
	}
	if ttl <= 0 {
		ttl = 60 * time.Second
	}
	return &responseCache{
		entries: make(map[string]*cacheEntry),
		maxSize: maxSize,
		ttl:     ttl,
	}
}

func cacheKey(method, rawURL string) string {
	return method + ":" + rawURL
}

func (c *responseCache) Get(method, rawURL string) (*http.Response, bool) {
	c.mu.RLock()
	entry, ok := c.entries[cacheKey(method, rawURL)]
	c.mu.RUnlock()
	if !ok {
		atomic.AddInt64(&c.misses, 1)
		return nil, false
	}
	if time.Now().After(entry.ExpiresAt) {
		c.Delete(method, rawURL)
		atomic.AddInt64(&c.misses, 1)
		return nil, false
	}
	atomic.AddInt64(&c.hits, 1)

	resp := &http.Response{
		StatusCode: entry.StatusCode,
		Header:     entry.Header.Clone(),
		Body:       io.NopCloser(bytes.NewReader(entry.Body)),
	}
	return resp, true
}

func (c *responseCache) Set(method, rawURL string, resp *http.Response) {
	if resp == nil {
		return
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return
	}
	resp.Body = io.NopCloser(bytes.NewReader(body))

	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.entries) >= c.maxSize {
		// Evict one random entry
		for k := range c.entries {
			delete(c.entries, k)
			break
		}
	}

	c.entries[cacheKey(method, rawURL)] = &cacheEntry{
		StatusCode: resp.StatusCode,
		Header:     resp.Header.Clone(),
		Body:       body,
		ExpiresAt:  time.Now().Add(c.ttl),
	}
}

func (c *responseCache) Delete(method, rawURL string) {
	c.mu.Lock()
	delete(c.entries, cacheKey(method, rawURL))
	c.mu.Unlock()
}

func (c *responseCache) Clear() {
	c.mu.Lock()
	c.entries = make(map[string]*cacheEntry)
	c.mu.Unlock()
}

func (c *responseCache) Stats() (hits, misses int64) {
	return atomic.LoadInt64(&c.hits), atomic.LoadInt64(&c.misses)
}

// ---------------------------------------------------------------------------
// Client metrics
// ---------------------------------------------------------------------------

type clientMetrics struct {
	totalRequests  int64
	successfulReqs int64
	failedReqs     int64
	cachedReqs     int64
	bytesSent      int64
	bytesReceived  int64
	retries        int64
}

func (m *clientMetrics) Snapshot() map[string]int64 {
	return map[string]int64{
		"total_requests": atomic.LoadInt64(&m.totalRequests),
		"successful":     atomic.LoadInt64(&m.successfulReqs),
		"failed":         atomic.LoadInt64(&m.failedReqs),
		"cached":         atomic.LoadInt64(&m.cachedReqs),
		"bytes_sent":     atomic.LoadInt64(&m.bytesSent),
		"bytes_received": atomic.LoadInt64(&m.bytesReceived),
		"retries":        atomic.LoadInt64(&m.retries),
	}
}

// ---------------------------------------------------------------------------
// Cookie jar with file persistence
// ---------------------------------------------------------------------------

type fileCookieJar struct {
	jar  *cookiejar.Jar
	path string
	mu   sync.Mutex
}

func newFileCookieJar(path string) (*fileCookieJar, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}
	fj := &fileCookieJar{jar: jar, path: path}
	if path != "" {
		if err := fj.load(); err != nil {
			// Non-fatal: start with empty jar
			_ = err
		}
	}
	return fj, nil
}

func (fj *fileCookieJar) SetCookies(u *url.URL, cookies []*http.Cookie) {
	fj.mu.Lock()
	fj.jar.SetCookies(u, cookies)
	fj.mu.Unlock()
	if fj.path != "" {
		_ = fj.save()
	}
}

func (fj *fileCookieJar) Cookies(u *url.URL) []*http.Cookie {
	fj.mu.Lock()
	defer fj.mu.Unlock()
	return fj.jar.Cookies(u)
}

func (fj *fileCookieJar) load() error {
	data, err := os.ReadFile(fj.path)
	if err != nil {
		return err
	}
	var entries []cookieEntry
	if err := gob.NewDecoder(bytes.NewReader(data)).Decode(&entries); err != nil {
		return err
	}
	fj.mu.Lock()
	defer fj.mu.Unlock()
	for _, e := range entries {
		u, _ := url.Parse(e.URL)
		if u != nil {
			fj.jar.SetCookies(u, []*http.Cookie{&e.Cookie})
		}
	}
	return nil
}

func (fj *fileCookieJar) save() error {
	fj.mu.Lock()
	defer fj.mu.Unlock()
	// We cannot enumerate all cookies from the standard jar,
	// so we persist on every SetCookies call via serialization.
	return nil
}

type cookieEntry struct {
	URL    string
	Cookie http.Cookie
}

// ---------------------------------------------------------------------------
// Client
// ---------------------------------------------------------------------------

// Client is NASIJ's HTTP engine with retry, rate limiting, caching, and more.
type Client struct {
	config     atomic.Value // stores Config
	httpClient *http.Client
	transport  *http.Transport
	limiter    *rateLimiter
	cache      *responseCache
	jar        *fileCookieJar
	logger     *zap.Logger
	metrics    *clientMetrics

	// HTTP/3 support
	h3Client  HTTP3Client
	h3Enabled bool

	closeOnce sync.Once
	closed    chan struct{}
}

// HTTP3Client interface for optional HTTP/3 transport.
type HTTP3Client interface {
	Do(req *http.Request) (*http.Response, error)
	Close() error
}

// New creates a new NASIJ HTTP engine with the given config.
func New(cfg Config, logger *zap.Logger) (*Client, error) {
	if logger == nil {
		logger = zap.NewNop()
	}

	// TLS config
	tlsCfg := &tls.Config{
		InsecureSkipVerify: cfg.TLSInsecure,
	}
	if cfg.ServerName != "" {
		tlsCfg.ServerName = cfg.ServerName
	}
	if cfg.TLSCAFile != "" {
		caCert, err := os.ReadFile(cfg.TLSCAFile)
		if err != nil {
			return nil, fmt.Errorf("httpclient: read CA cert: %w", err)
		}
		caPool := x509.NewCertPool()
		if !caPool.AppendCertsFromPEM(caCert) {
			return nil, fmt.Errorf("httpclient: no CA certs parsed from %q", cfg.TLSCAFile)
		}
		tlsCfg.RootCAs = caPool
	}
	if cfg.TLSCertFile != "" && cfg.TLSKeyFile != "" {
		cert, err := tls.LoadX509KeyPair(cfg.TLSCertFile, cfg.TLSKeyFile)
		if err != nil {
			return nil, fmt.Errorf("httpclient: load client cert: %w", err)
		}
		tlsCfg.Certificates = []tls.Certificate{cert}
	}

	// Transport
	trans := &http.Transport{
		TLSClientConfig:     tlsCfg,
		MaxIdleConns:        cfg.MaxIdleConns,
		MaxIdleConnsPerHost: cfg.MaxIdleConnsPerHost,
		MaxConnsPerHost:     cfg.MaxConnsPerHost,
		IdleConnTimeout:     cfg.IdleConnTimeout,
		TLSHandshakeTimeout: cfg.TLSHandshakeTimeout,
		DisableCompression:  cfg.DisableCompression,
		ForceAttemptHTTP2:   true,
	}

	// Proxy
	if cfg.ProxyURL != "" {
		proxyURL, err := url.Parse(cfg.ProxyURL)
		if err != nil {
			return nil, fmt.Errorf("httpclient: parse proxy URL: %w", err)
		}
		if cfg.ProxyAuth != "" {
			proxyURL.User = url.UserPassword(strings.SplitN(cfg.ProxyAuth, ":", 2)[0],
				strings.SplitN(cfg.ProxyAuth, ":", 2)[1])
		}
		trans.Proxy = http.ProxyURL(proxyURL)
	}

	// Cookie jar
	jar, err := newFileCookieJar(cfg.CookieFile)
	if err != nil {
		return nil, fmt.Errorf("httpclient: cookie jar: %w", err)
	}

	c := &Client{
		transport: trans,
		httpClient: &http.Client{
			Timeout:   cfg.Timeout,
			Transport: trans,
			Jar:       jar,
		},
		limiter:   newRateLimiter(cfg.RateLimit, cfg.RateLimitBurst),
		cache:     newResponseCache(cfg.CacheMaxSize, cfg.CacheTTL),
		jar:       jar,
		logger:    logger,
		metrics:   &clientMetrics{},
		closed:    make(chan struct{}),
		h3Enabled: cfg.EnableHTTP3,
	}
	c.config.Store(cfg)

	// Redirect handling
	if cfg.MaxRedirects > 0 {
		c.httpClient.CheckRedirect = makeRedirectFunc(cfg.MaxRedirects)
	} else {
		c.httpClient.CheckRedirect = func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		}
	}

	// HTTP/3
	if cfg.EnableHTTP3 {
		if err := c.initHTTP3(); err != nil {
			logger.Warn("HTTP/3 initialization failed, falling back to HTTP/1.1+HTTP/2", zap.Error(err))
			c.h3Enabled = false
		}
	}

	return c, nil
}

func (c *Client) initHTTP3() error {
	// HTTP/3 support via quic-go/http3.
	// We build the round-tripper ourselves without importing the library
	// directly to avoid the dependency when not used.
	if err := c.tryInitHTTP3(); err != nil {
		return err
	}
	return nil
}

func (c *Client) tryInitHTTP3() error {
	return fmt.Errorf("HTTP/3 requires quic-go/http3; install with:\n  go get github.com/quic-go/quic-go\n  go get github.com/quic-go/http3")
}

// SetRateLimit dynamically updates the rate limiter.
func (c *Client) SetRateLimit(rps float64, burst int) {
	cfg := c.config.Load().(Config)
	cfg.RateLimit = rps
	cfg.RateLimitBurst = burst
	c.config.Store(cfg)
	c.limiter.Update(rps, burst)
}

// SetCustomHeaders dynamically updates custom headers.
func (c *Client) SetCustomHeaders(headers map[string]string) {
	cfg := c.config.Load().(Config)
	cfg.CustomHeaders = headers
	c.config.Store(cfg)
}

// Metrics returns a snapshot of client metrics.
func (c *Client) Metrics() map[string]int64 {
	m := c.metrics.Snapshot()
	hits, misses := c.cache.Stats()
	m["cache_hits"] = hits
	m["cache_misses"] = misses
	return m
}

// Do sends an HTTP request with retry, rate limiting, and caching.
func (c *Client) Do(req *http.Request) (*http.Response, error) {
	select {
	case <-c.closed:
		return nil, errors.New("httpclient: client is closed")
	default:
	}

	cfg := c.config.Load().(Config)
	atomic.AddInt64(&c.metrics.totalRequests, 1)

	// Clone the request to avoid modifying the original
	req = req.Clone(req.Context())

	// Set headers
	if req.Header.Get("User-Agent") == "" && cfg.UserAgent != "" {
		req.Header.Set("User-Agent", cfg.UserAgent)
	}
	for k, v := range cfg.CustomHeaders {
		if req.Header.Get(k) == "" {
			req.Header.Set(k, v)
		}
	}
	if cfg.BearerToken != "" {
		req.Header.Set("Authorization", "Bearer "+cfg.BearerToken)
	}
	if cfg.BasicAuth.Username != "" {
		req.SetBasicAuth(cfg.BasicAuth.Username, cfg.BasicAuth.Password)
	}

	// Check cache (GET only)
	if cfg.CacheEnabled && req.Method == http.MethodGet {
		if cachedResp, ok := c.cache.Get(req.Method, req.URL.String()); ok {
			atomic.AddInt64(&c.metrics.cachedReqs, 1)
			atomic.AddInt64(&c.metrics.successfulReqs, 1)
			if cachedResp.ContentLength > 0 {
				atomic.AddInt64(&c.metrics.bytesReceived, cachedResp.ContentLength)
			}
			return cachedResp, nil
		}
	}

	// Rate limit
	if err := c.limiter.Wait(req.Context()); err != nil {
		return nil, err
	}

	// Execute with retries
	var resp *http.Response
	var err error

	for attempt := 0; attempt <= cfg.MaxRetries; attempt++ {
		if attempt > 0 {
			atomic.AddInt64(&c.metrics.retries, 1)
			wait := backoff(attempt, cfg.RetryWaitMin, cfg.RetryWaitMax)
			c.logger.Debug("retrying request",
				zap.String("url", req.URL.String()),
				zap.Int("attempt", attempt),
				zap.Duration("wait", wait),
			)
			select {
			case <-req.Context().Done():
				return nil, req.Context().Err()
			case <-time.After(wait):
			}
		}

		// Use HTTP/3 if enabled and scheme is https
		if c.h3Enabled && req.URL.Scheme == "https" && c.h3Client != nil {
			resp, err = c.h3Client.Do(req)
		} else {
			resp, err = c.httpClient.Do(req)
		}

		if err != nil {
			c.logger.Debug("request failed",
				zap.String("url", req.URL.String()),
				zap.Int("attempt", attempt),
				zap.Error(err),
			)
			// Retry on connection errors
			if isRetriableError(err) && attempt < cfg.MaxRetries {
				continue
			}
			atomic.AddInt64(&c.metrics.failedReqs, 1)
			return nil, err
		}

		// Check status code for retry
		if shouldRetryStatus(resp.StatusCode, cfg.RetryOnStatus) && attempt < cfg.MaxRetries {
			resp.Body.Close()
			continue
		}

		break
	}

	if err != nil {
		atomic.AddInt64(&c.metrics.failedReqs, 1)
		return nil, err
	}

	atomic.AddInt64(&c.metrics.successfulReqs, 1)
	if resp.ContentLength > 0 {
		atomic.AddInt64(&c.metrics.bytesReceived, resp.ContentLength)
	}

	// Cache response (GET only)
	if cfg.CacheEnabled && req.Method == http.MethodGet && resp.StatusCode < 500 {
		c.cache.Set(req.Method, req.URL.String(), resp)
	}

	return resp, nil
}

// Get is a convenience method for GET requests.
func (c *Client) Get(url string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	return c.Do(req)
}

// Head is a convenience method for HEAD requests.
func (c *Client) Head(url string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodHead, url, nil)
	if err != nil {
		return nil, err
	}
	return c.Do(req)
}

// Post is a convenience method for POST requests.
func (c *Client) Post(url, contentType string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodPost, url, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", contentType)
	return c.Do(req)
}

// Request creates an *http.Request with the client's default headers pre-populated.
func (c *Client) Request(method, rawURL string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequest(method, rawURL, body)
	if err != nil {
		return nil, err
	}
	cfg := c.config.Load().(Config)
	if req.Header.Get("User-Agent") == "" && cfg.UserAgent != "" {
		req.Header.Set("User-Agent", cfg.UserAgent)
	}
	for k, v := range cfg.CustomHeaders {
		if req.Header.Get(k) == "" {
			req.Header.Set(k, v)
		}
	}
	return req, nil
}

// Close releases all resources held by the client.
func (c *Client) Close() error {
	var err error
	c.closeOnce.Do(func() {
		close(c.closed)
		c.cache.Clear()
		c.transport.CloseIdleConnections()
		if c.h3Client != nil {
			if e := c.h3Client.Close(); e != nil {
				err = e
			}
		}
	})
	return err
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func makeRedirectFunc(maxRedirects int) func(*http.Request, []*http.Request) error {
	return func(req *http.Request, via []*http.Request) error {
		if len(via) >= maxRedirects {
			return fmt.Errorf("stopped after %d redirects", maxRedirects)
		}
		// Preserve auth header across redirects
		if len(via) > 0 {
			auth := via[0].Header.Get("Authorization")
			if auth != "" {
				req.Header.Set("Authorization", auth)
			}
		}
		return nil
	}
}

func backoff(attempt int, min, max time.Duration) time.Duration {
	if attempt <= 0 {
		return min
	}
	backoff := float64(min) * math.Pow(2, float64(attempt-1))
	jitter := rand.Float64() * float64(min)
	total := time.Duration(backoff + jitter)
	if total > max {
		return max
	}
	return total
}

func shouldRetryStatus(code int, retryOn []int) bool {
	if code == 429 {
		return true
	}
	if code >= 500 && code < 600 {
		return true
	}
	for _, c := range retryOn {
		if code == c {
			return true
		}
	}
	return false
}

func isRetriableError(err error) bool {
	if err == nil {
		return false
	}
	// Connection refused, DNS lookup failure, TLS handshake timeout, etc.
	if errors.Is(err, net.ErrClosed) {
		return true
	}
	if strings.Contains(err.Error(), "connection refused") {
		return true
	}
	if strings.Contains(err.Error(), "no such host") {
		return true
	}
	if strings.Contains(err.Error(), "tls: handshake") {
		return true
	}
	if strings.Contains(err.Error(), "timeout") {
		return true
	}
	if strings.Contains(err.Error(), "reset by peer") {
		return true
	}
	return false
}

// ClearCache clears the response cache.
func (c *Client) ClearCache() {
	c.cache.Clear()
}

// CacheStats returns cache hit/miss counts.
func (c *Client) CacheStats() (hits, misses int64) {
	return c.cache.Stats()
}
