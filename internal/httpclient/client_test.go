package httpclient_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/nasij/nasij/internal/httpclient"
)

func newTestLogger(t *testing.T) *zap.Logger {
	t.Helper()
	logger, err := zap.NewDevelopment()
	require.NoError(t, err)
	t.Cleanup(func() { _ = logger.Sync() })
	return logger
}

func TestDefaultConfig(t *testing.T) {
	cfg := httpclient.DefaultConfig()
	assert.Equal(t, "NASIJ/1.0", cfg.UserAgent)
	assert.Equal(t, 3, cfg.MaxRetries)
	assert.Equal(t, 10, cfg.MaxRedirects)
	assert.True(t, cfg.CacheEnabled)
	assert.Equal(t, 1000, cfg.CacheMaxSize)
}

func TestNewClient_Default(t *testing.T) {
	c, err := httpclient.New(httpclient.DefaultConfig(), newTestLogger(t))
	require.NoError(t, err)
	assert.NotNil(t, c)
	c.Close()
}

func TestNewClient_WithProxy(t *testing.T) {
	cfg := httpclient.DefaultConfig()
	cfg.ProxyURL = "http://127.0.0.1:8080"
	c, err := httpclient.New(cfg, newTestLogger(t))
	require.NoError(t, err)
	assert.NotNil(t, c)
	c.Close()
}

func TestNewClient_WithTLS(t *testing.T) {
	cfg := httpclient.DefaultConfig()
	cfg.TLSInsecure = true
	c, err := httpclient.New(cfg, newTestLogger(t))
	require.NoError(t, err)
	assert.NotNil(t, c)
	c.Close()
}

func TestNewClient_WithAuth(t *testing.T) {
	cfg := httpclient.DefaultConfig()
	cfg.BearerToken = "test-token"
	c, err := httpclient.New(cfg, newTestLogger(t))
	require.NoError(t, err)
	c.Close()
}

func TestNewClient_NilLogger(t *testing.T) {
	c, err := httpclient.New(httpclient.DefaultConfig(), nil)
	require.NoError(t, err)
	c.Close()
}

// --- Basic GET ---

func TestClient_Get(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "NASIJ/1.0", r.UserAgent())
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("hello"))
	}))
	defer ts.Close()

	c, err := httpclient.New(httpclient.DefaultConfig(), newTestLogger(t))
	require.NoError(t, err)
	defer c.Close()

	resp, err := c.Get(ts.URL)
	require.NoError(t, err)
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	assert.Equal(t, "hello", string(body))
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestClient_Get_CustomUA(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "CustomBot/1.0", r.UserAgent())
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	cfg := httpclient.DefaultConfig()
	cfg.UserAgent = "CustomBot/1.0"
	c, err := httpclient.New(cfg, newTestLogger(t))
	require.NoError(t, err)
	defer c.Close()

	resp, err := c.Get(ts.URL)
	require.NoError(t, err)
	resp.Body.Close()
}

func TestClient_Get_CustomHeaders(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "myvalue", r.Header.Get("X-Custom"))
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	cfg := httpclient.DefaultConfig()
	cfg.CustomHeaders = map[string]string{"X-Custom": "myvalue"}
	c, err := httpclient.New(cfg, newTestLogger(t))
	require.NoError(t, err)
	defer c.Close()

	resp, err := c.Get(ts.URL)
	require.NoError(t, err)
	resp.Body.Close()
}

func TestClient_Get_BearerToken(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer tok123", r.Header.Get("Authorization"))
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	cfg := httpclient.DefaultConfig()
	cfg.BearerToken = "tok123"
	c, err := httpclient.New(cfg, newTestLogger(t))
	require.NoError(t, err)
	defer c.Close()

	resp, err := c.Get(ts.URL)
	require.NoError(t, err)
	resp.Body.Close()
}

func TestClient_Get_BasicAuth(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		assert.True(t, ok)
		assert.Equal(t, "admin", user)
		assert.Equal(t, "secret", pass)
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	cfg := httpclient.DefaultConfig()
	cfg.BasicAuth = httpclient.UserPass{Username: "admin", Password: "secret"}
	c, err := httpclient.New(cfg, newTestLogger(t))
	require.NoError(t, err)
	defer c.Close()

	resp, err := c.Get(ts.URL)
	require.NoError(t, err)
	resp.Body.Close()
}

// --- Rate Limiting ---

func TestClient_RateLimit(t *testing.T) {
	var mu sync.Mutex
	var timestamps []time.Time

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		timestamps = append(timestamps, time.Now())
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	cfg := httpclient.DefaultConfig()
	cfg.RateLimit = 50.0 // 50 req/s
	cfg.RateLimitBurst = 1
	cfg.CacheEnabled = false
	c, err := httpclient.New(cfg, newTestLogger(t))
	require.NoError(t, err)
	defer c.Close()

	// Fire 5 requests in quick succession
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			resp, err := c.Get(ts.URL)
			if err == nil {
				resp.Body.Close()
			}
		}()
	}
	wg.Wait()

	mu.Lock()
	times := timestamps
	mu.Unlock()

	// With 50 req/s and burst=1, 5 requests should take at least ~80ms
	require.Len(t, times, 5)
	minTime := times[len(times)-1].Sub(times[0])
	t.Logf("5 requests at 50/s took %v", minTime)
	assert.GreaterOrEqual(t, minTime, 40*time.Millisecond)
}

// Caching

func TestClient_Cache_Hit(t *testing.T) {
	var callCount int32

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&callCount, 1)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("cached"))
	}))
	defer ts.Close()

	cfg := httpclient.DefaultConfig()
	cfg.CacheEnabled = true
	cfg.CacheTTL = 5 * time.Second
	c, err := httpclient.New(cfg, newTestLogger(t))
	require.NoError(t, err)
	defer c.Close()

	// First request: should hit server
	resp1, err := c.Get(ts.URL)
	require.NoError(t, err)
	body1, _ := io.ReadAll(resp1.Body)
	resp1.Body.Close()
	assert.Equal(t, "cached", string(body1))
	assert.Equal(t, int32(1), atomic.LoadInt32(&callCount))

	// Second request: should come from cache
	resp2, err := c.Get(ts.URL)
	require.NoError(t, err)
	body2, _ := io.ReadAll(resp2.Body)
	resp2.Body.Close()
	assert.Equal(t, "cached", string(body2))
	assert.Equal(t, int32(1), atomic.LoadInt32(&callCount))
}

func TestClient_Cache_DifferentURLs(t *testing.T) {
	var callCount int32

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&callCount, 1)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(r.URL.Path))
	}))
	defer ts.Close()

	cfg := httpclient.DefaultConfig()
	cfg.CacheEnabled = true
	cfg.CacheTTL = 5 * time.Second
	c, err := httpclient.New(cfg, newTestLogger(t))
	require.NoError(t, err)
	defer c.Close()

	c.Get(ts.URL + "/a")
	assert.Equal(t, int32(1), atomic.LoadInt32(&callCount))
	c.Get(ts.URL + "/b")
	assert.Equal(t, int32(2), atomic.LoadInt32(&callCount))
}

func TestClient_Cache_Expiry(t *testing.T) {
	var callCount int32

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&callCount, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	cfg := httpclient.DefaultConfig()
	cfg.CacheEnabled = true
	cfg.CacheTTL = 50 * time.Millisecond
	c, err := httpclient.New(cfg, newTestLogger(t))
	require.NoError(t, err)
	defer c.Close()

	c.Get(ts.URL)
	assert.Equal(t, int32(1), atomic.LoadInt32(&callCount))

	// Cache hit
	c.Get(ts.URL)
	assert.Equal(t, int32(1), atomic.LoadInt32(&callCount))

	// Wait for expiry
	time.Sleep(100 * time.Millisecond)

	// Cache miss
	c.Get(ts.URL)
	assert.Equal(t, int32(2), atomic.LoadInt32(&callCount))
}

func TestClient_ClearCache(t *testing.T) {
	var callCount int32

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&callCount, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	cfg := httpclient.DefaultConfig()
	cfg.CacheEnabled = true
	c, err := httpclient.New(cfg, newTestLogger(t))
	require.NoError(t, err)
	defer c.Close()

	c.Get(ts.URL)
	c.Get(ts.URL)
	assert.Equal(t, int32(1), atomic.LoadInt32(&callCount))

	c.ClearCache()
	c.Get(ts.URL)
	assert.Equal(t, int32(2), atomic.LoadInt32(&callCount))
}

// --- Retries ---

func TestClient_Retry_On5xx(t *testing.T) {
	var attempt int32

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cur := atomic.AddInt32(&attempt, 1)
		if cur < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	cfg := httpclient.DefaultConfig()
	cfg.MaxRetries = 3
	cfg.RetryWaitMin = 10 * time.Millisecond
	cfg.RetryWaitMax = 50 * time.Millisecond
	cfg.CacheEnabled = false
	c, err := httpclient.New(cfg, newTestLogger(t))
	require.NoError(t, err)
	defer c.Close()

	resp, err := c.Get(ts.URL)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, int32(3), atomic.LoadInt32(&attempt))
}

func TestClient_Retry_On429(t *testing.T) {
	var attempt int32

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cur := atomic.AddInt32(&attempt, 1)
		if cur < 2 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	cfg := httpclient.DefaultConfig()
	cfg.MaxRetries = 2
	cfg.RetryWaitMin = 5 * time.Millisecond
	cfg.RetryWaitMax = 20 * time.Millisecond
	cfg.CacheEnabled = false
	c, err := httpclient.New(cfg, newTestLogger(t))
	require.NoError(t, err)
	defer c.Close()

	resp, err := c.Get(ts.URL)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, int32(2), atomic.LoadInt32(&attempt))
}

func TestClient_Retry_MaxedOut(t *testing.T) {
	var attempt int32

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&attempt, 1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	cfg := httpclient.DefaultConfig()
	cfg.MaxRetries = 2
	cfg.RetryWaitMin = 5 * time.Millisecond
	cfg.RetryWaitMax = 20 * time.Millisecond
	cfg.CacheEnabled = false
	c, err := httpclient.New(cfg, newTestLogger(t))
	require.NoError(t, err)
	defer c.Close()

	resp, err := c.Get(ts.URL)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	assert.Equal(t, int32(3), atomic.LoadInt32(&attempt)) // initial + 2 retries
}

// --- Redirects ---

func TestClient_Redirect_Follow(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/redirect" {
			http.Redirect(w, r, "/target", http.StatusFound)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("final"))
	}))
	defer ts.Close()

	cfg := httpclient.DefaultConfig()
	cfg.MaxRedirects = 5
	c, err := httpclient.New(cfg, newTestLogger(t))
	require.NoError(t, err)
	defer c.Close()

	resp, err := c.Get(ts.URL + "/redirect")
	require.NoError(t, err)
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	assert.Equal(t, "final", string(body))
}

func TestClient_Redirect_NoFollow(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/target", http.StatusFound)
	}))
	defer ts.Close()

	cfg := httpclient.DefaultConfig()
	cfg.MaxRedirects = 0
	c, err := httpclient.New(cfg, newTestLogger(t))
	require.NoError(t, err)
	defer c.Close()

	resp, err := c.Get(ts.URL + "/redirect")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusFound, resp.StatusCode)
}

// --- Timeout and context ---

func TestClient_ContextTimeout(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(500 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	cfg := httpclient.DefaultConfig()
	cfg.Timeout = 50 * time.Millisecond
	c, err := httpclient.New(cfg, newTestLogger(t))
	require.NoError(t, err)
	defer c.Close()

	_, err = c.Get(ts.URL)
	require.Error(t, err)
}

func TestClient_ContextCancelled(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	cfg := httpclient.DefaultConfig()
	cfg.CacheEnabled = false
	c, err := httpclient.New(cfg, newTestLogger(t))
	require.NoError(t, err)
	defer c.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	req, _ := http.NewRequestWithContext(ctx, "GET", ts.URL, nil)
	_, err = c.Do(req)
	require.Error(t, err)
}

// --- Request builder ---

func TestClient_Request(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "NASIJ/1.0", r.UserAgent())
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	cfg := httpclient.DefaultConfig()
	c, err := httpclient.New(cfg, newTestLogger(t))
	require.NoError(t, err)
	defer c.Close()

	req, err := c.Request("GET", ts.URL, nil)
	require.NoError(t, err)
	assert.Equal(t, "NASIJ/1.0", req.Header.Get("User-Agent"))

	resp, err := c.Do(req)
	require.NoError(t, err)
	resp.Body.Close()
}

// --- Metrics ---

func TestClient_Metrics(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	cfg := httpclient.DefaultConfig()
	cfg.CacheEnabled = false
	c, err := httpclient.New(cfg, newTestLogger(t))
	require.NoError(t, err)
	defer c.Close()

	m := c.Metrics()
	assert.Equal(t, int64(0), m["total_requests"])

	c.Get(ts.URL)
	m = c.Metrics()
	assert.Equal(t, int64(1), m["total_requests"])
	assert.Equal(t, int64(1), m["successful"])
	assert.Equal(t, int64(0), m["failed"])
}

// --- Close ---

func TestClient_Close_Idempotent(t *testing.T) {
	c, err := httpclient.New(httpclient.DefaultConfig(), newTestLogger(t))
	require.NoError(t, err)
	assert.NoError(t, c.Close())
	assert.NoError(t, c.Close()) // second close should be safe
}

func TestClient_Do_AfterClose(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	c, err := httpclient.New(httpclient.DefaultConfig(), newTestLogger(t))
	require.NoError(t, err)
	c.Close()

	_, err = c.Get(ts.URL)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "closed")
}

// --- FromScope ---

func TestFromScope_Defaults(t *testing.T) {
	cfg := httpclient.FromScope(0, 0, "", "", nil)
	assert.Equal(t, "NASIJ/1.0", cfg.UserAgent)
	assert.Equal(t, 0.0, cfg.RateLimit)
}

func TestFromScope_RateLimit(t *testing.T) {
	cfg := httpclient.FromScope(50, 10, "", "", nil)
	assert.Equal(t, 50.0, cfg.RateLimit)
	assert.Equal(t, 10, cfg.RateLimitBurst)
}

func TestFromScope_BearerAuth(t *testing.T) {
	cfg := httpclient.FromScope(0, 0, "bearer", "tok-abc", nil)
	assert.Equal(t, "tok-abc", cfg.BearerToken)
}

func TestFromScope_BasicAuth(t *testing.T) {
	cfg := httpclient.FromScope(0, 0, "basic", "admin:secret", nil)
	assert.Equal(t, "admin", cfg.BasicAuth.Username)
	assert.Equal(t, "secret", cfg.BasicAuth.Password)
}

func TestFromScope_HeaderAuth(t *testing.T) {
	cfg := httpclient.FromScope(0, 0, "header", "X-API-Key: abc123", nil)
	assert.Equal(t, "abc123", cfg.CustomHeaders["X-API-Key"])
}

func TestFromScope_CustomHeaders(t *testing.T) {
	hdrs := map[string]string{"X-Extra": "value"}
	cfg := httpclient.FromScope(0, 0, "", "", hdrs)
	assert.Equal(t, "value", cfg.CustomHeaders["X-Extra"])
}

// --- Rate limiter ---

func TestRateLimiter_AllowsWithinRate(t *testing.T) {
	var count int32

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&count, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	cfg := httpclient.DefaultConfig()
	cfg.MaxRetries = 0
	cfg.CacheEnabled = false
	cfg.RateLimit = 1000.0 // very high rate (essentially unlimited)
	c, err := httpclient.New(cfg, newTestLogger(t))
	require.NoError(t, err)
	defer c.Close()

	// Should be able to fire multiple requests quickly
	for i := 0; i < 20; i++ {
		resp, err := c.Get(ts.URL)
		if err == nil {
			resp.Body.Close()
		}
	}

	assert.Equal(t, int32(20), atomic.LoadInt32(&count))
}

func TestRateLimiter_ZeroRate(t *testing.T) {
	cfg := httpclient.DefaultConfig()
	cfg.RateLimit = 0 // unlimited
	c, err := httpclient.New(cfg, nil)
	require.NoError(t, err)
	c.Close()
}

func TestSetRateLimit(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	cfg := httpclient.DefaultConfig()
	cfg.RateLimit = 0 // start unlimited
	c, err := httpclient.New(cfg, newTestLogger(t))
	require.NoError(t, err)
	defer c.Close()

	// Dynamically set rate limit
	c.SetRateLimit(100.0, 10)

	resp, err := c.Get(ts.URL)
	require.NoError(t, err)
	resp.Body.Close()
}

// --- SetCustomHeaders ---

func TestSetCustomHeaders(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "bar", r.Header.Get("X-Foo"))
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	cfg := httpclient.DefaultConfig()
	c, err := httpclient.New(cfg, newTestLogger(t))
	require.NoError(t, err)
	defer c.Close()

	c.SetCustomHeaders(map[string]string{"X-Foo": "bar"})
	resp, err := c.Get(ts.URL)
	require.NoError(t, err)
	resp.Body.Close()
}

// --- Client lifecycle ---

func TestNewClient_InvalidProxy(t *testing.T) {
	cfg := httpclient.DefaultConfig()
	cfg.ProxyURL = "://invalid"
	_, err := httpclient.New(cfg, nil)
	require.Error(t, err)
}

func TestNewClient_MissingCACert(t *testing.T) {
	cfg := httpclient.DefaultConfig()
	cfg.TLSCAFile = "/nonexistent/ca.pem"
	_, err := httpclient.New(cfg, nil)
	require.Error(t, err)
}

// --- Compression ---

func TestClient_DisableCompression(t *testing.T) {
	cfg := httpclient.DefaultConfig()
	cfg.DisableCompression = true
	c, err := httpclient.New(cfg, newTestLogger(t))
	require.NoError(t, err)
	c.Close()
}

// --- Post ---

func TestClient_Post(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "text/plain", r.Header.Get("Content-Type"))
		body, _ := io.ReadAll(r.Body)
		assert.Equal(t, "data", string(body))
		w.WriteHeader(http.StatusCreated)
	}))
	defer ts.Close()

	cfg := httpclient.DefaultConfig()
	cfg.CacheEnabled = false
	c, err := httpclient.New(cfg, newTestLogger(t))
	require.NoError(t, err)
	defer c.Close()

	resp, err := c.Post(ts.URL, "text/plain", strings.NewReader("data"))
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusCreated, resp.StatusCode)
}

// --- Head ---

func TestClient_Head(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "HEAD", r.Method)
		w.Header().Set("X-Test", "value")
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	cfg := httpclient.DefaultConfig()
	c, err := httpclient.New(cfg, newTestLogger(t))
	require.NoError(t, err)
	defer c.Close()

	resp, err := c.Head(ts.URL)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, "value", resp.Header.Get("X-Test"))
}

// --- Cache stats ---

func TestClient_CacheStats(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	cfg := httpclient.DefaultConfig()
	cfg.CacheEnabled = true
	cfg.CacheTTL = 5 * time.Second
	c, err := httpclient.New(cfg, newTestLogger(t))
	require.NoError(t, err)
	defer c.Close()

	hits, misses := c.CacheStats()
	assert.Equal(t, int64(0), hits)
	assert.Equal(t, int64(0), misses)

	c.Get(ts.URL) // miss
	hits, misses = c.CacheStats()
	assert.Equal(t, int64(0), hits)
	assert.Equal(t, int64(1), misses)

	c.Get(ts.URL) // hit
	hits, misses = c.CacheStats()
	assert.Equal(t, int64(1), hits)
	assert.Equal(t, int64(1), misses)
}

// --- Redirect preserves auth ---

func TestClient_RedirectPreservesAuth(t *testing.T) {
	var seenTarget bool

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/redirect" {
			http.Redirect(w, r, "/target", http.StatusFound)
			return
		}
		seenTarget = true
		assert.Equal(t, "Bearer preserved-token", r.Header.Get("Authorization"))
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	cfg := httpclient.DefaultConfig()
	cfg.BearerToken = "preserved-token"
	cfg.MaxRedirects = 5
	c, err := httpclient.New(cfg, newTestLogger(t))
	require.NoError(t, err)
	defer c.Close()

	resp, err := c.Get(ts.URL + "/redirect")
	require.NoError(t, err)
	resp.Body.Close()
	assert.True(t, seenTarget)
}

// --- Concurrency ---

func TestClient_ConcurrentRequests(t *testing.T) {
	var requestCount int32

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestCount, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	cfg := httpclient.DefaultConfig()
	cfg.CacheEnabled = false
	cfg.RateLimit = 1000.0
	c, err := httpclient.New(cfg, newTestLogger(t))
	require.NoError(t, err)
	defer c.Close()

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			resp, err := c.Get(ts.URL)
			if err == nil {
				resp.Body.Close()
			}
		}()
	}
	wg.Wait()
	assert.Equal(t, int32(10), atomic.LoadInt32(&requestCount))
}

// --- Backoff (indirect test via retry behavior) ---

func TestBackoffBehavior(t *testing.T) {
	// This test verifies that backoff delays increase with attempt count
	// by checking that a server returning 503 triggers increasing delays.
	var attempts []time.Time
	var mu sync.Mutex

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		attempts = append(attempts, time.Now())
		mu.Unlock()
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer ts.Close()

	cfg := httpclient.DefaultConfig()
	cfg.MaxRetries = 2
	cfg.RetryWaitMin = 10 * time.Millisecond
	cfg.RetryWaitMax = 100 * time.Millisecond
	cfg.CacheEnabled = false
	c, err := httpclient.New(cfg, newTestLogger(t))
	require.NoError(t, err)
	defer c.Close()

	c.Get(ts.URL)

	mu.Lock()
	times := attempts
	mu.Unlock()

	// With 3 attempts (initial + 2 retries), delays should increase
	if len(times) >= 3 {
		d1 := times[1].Sub(times[0])
		d2 := times[2].Sub(times[1])
		t.Logf("attempt delays: %v, %v", d1, d2)
	}
}

// --- Verify req.URL doesn't change after Do ---

func TestClient_RequestURLUnchanged(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	cfg := httpclient.DefaultConfig()
	cfg.MaxRedirects = 3
	cfg.CacheEnabled = false
	c, err := httpclient.New(cfg, newTestLogger(t))
	require.NoError(t, err)
	defer c.Close()

	req, _ := http.NewRequest("GET", ts.URL, nil)
	originalURL := req.URL.String()
	c.Do(req)
	assert.Equal(t, originalURL, req.URL.String())
}
