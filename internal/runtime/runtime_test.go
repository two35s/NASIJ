package runtime

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResultZeroValue(t *testing.T) {
	r := Result{}
	assert.Empty(t, r.Requests)
	assert.Empty(t, r.WebSockets)
	assert.Empty(t, r.EventSources)
	assert.Empty(t, r.Cookies)
	assert.Empty(t, r.LocalStorage)
	assert.Empty(t, r.SessionStorage)
	assert.Empty(t, r.IndexedDB)
	assert.Empty(t, r.ServiceWorkers)
	assert.Empty(t, r.URL)
}

func TestRequestRecord(t *testing.T) {
	r := RequestRecord{
		URL:          "https://example.com/api",
		Method:       "POST",
		ResourceType: "fetch",
		StatusCode:   200,
		Headers:      map[string]string{"content-type": "application/json"},
	}
	assert.Equal(t, "https://example.com/api", r.URL)
	assert.Equal(t, "POST", r.Method)
	assert.Equal(t, "fetch", r.ResourceType)
	assert.Equal(t, 200, r.StatusCode)
	assert.Equal(t, "application/json", r.Headers["content-type"])
}

func TestCookieRecord(t *testing.T) {
	c := CookieRecord{
		Name: "session", Value: "abc123",
		Domain: "example.com", Path: "/",
		Secure: true, HttpOnly: true,
	}
	assert.Equal(t, "session", c.Name)
	assert.Equal(t, "abc123", c.Value)
	assert.True(t, c.Secure)
	assert.True(t, c.HttpOnly)
}

func TestStorageRecord(t *testing.T) {
	s := StorageRecord{Key: "theme", Value: "dark"}
	assert.Equal(t, "theme", s.Key)
	assert.Equal(t, "dark", s.Value)
}

func TestIndexedDBRecord(t *testing.T) {
	rec := IndexedDBRecord{
		Database: "testdb",
		Version:  1,
		Stores: []IndexedDBStore{
			{Name: "users", Records: []any{map[string]any{"id": 1, "name": "alice"}}},
		},
	}
	assert.Equal(t, "testdb", rec.Database)
	assert.Equal(t, int64(1), rec.Version)
	assert.Len(t, rec.Stores, 1)
	assert.Equal(t, "users", rec.Stores[0].Name)
}

func TestServiceWorkerRecord(t *testing.T) {
	sw := ServiceWorkerRecord{
		URL: "https://example.com/sw.js",
		Scope: "/",
		Active: true,
	}
	assert.Equal(t, "https://example.com/sw.js", sw.URL)
	assert.True(t, sw.Active)
}

func TestWebSocketRecord(t *testing.T) {
	ws := WebSocketRecord{URL: "wss://example.com/socket"}
	assert.Equal(t, "wss://example.com/socket", ws.URL)
}

func TestEventSourceRecord(t *testing.T) {
	es := EventSourceRecord{URL: "https://example.com/events"}
	assert.Equal(t, "https://example.com/events", es.URL)
}

func TestCollectorClosed(t *testing.T) {
	c := &Collector{closed: true}
	_, err := c.CollectURL("http://example.com")
	assert.ErrorContains(t, err, "closed")
}

func TestNewAndClose(t *testing.T) {
	if os.Getenv("PLAYWRIGHT_BROWSERS_PATH") == "" && os.Getenv("CI") == "" {
		t.Skip("skipping; set PLAYWRIGHT_BROWSERS_PATH or CI to run Playwright tests")
	}

	c, err := New()
	require.NoError(t, err)
	require.NotNil(t, c)
	assert.False(t, c.closed)

	err = c.Close()
	assert.NoError(t, err)
	assert.True(t, c.closed)

	err = c.Close()
	assert.NoError(t, err)
}

func TestCollectURL(t *testing.T) {
	if os.Getenv("PLAYWRIGHT_BROWSERS_PATH") == "" && os.Getenv("CI") == "" {
		t.Skip("skipping; set PLAYWRIGHT_BROWSERS_PATH or CI to run Playwright tests")
	}

	c, err := New()
	require.NoError(t, err)
	defer c.Close()

	result, err := c.CollectURL("https://example.com")
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, "https://example.com", result.URL)

	assert.NotEmpty(t, result.Requests)

	foundDoc := false
	for _, req := range result.Requests {
		if req.ResourceType == "document" {
			foundDoc = true
		}
		assert.NotEmpty(t, req.URL)
		assert.NotEmpty(t, req.Method)
	}
	assert.True(t, foundDoc, "expected at least one document request")
}

func TestCollectStorageViaEvaluate(t *testing.T) {
	if os.Getenv("PLAYWRIGHT_BROWSERS_PATH") == "" && os.Getenv("CI") == "" {
		t.Skip("skipping; set PLAYWRIGHT_BROWSERS_PATH or CI to run Playwright tests")
	}

	c, err := New()
	require.NoError(t, err)
	defer c.Close()

	bctx, err := c.browser.NewContext()
	require.NoError(t, err)
	defer bctx.Close()

	page, err := bctx.NewPage()
	require.NoError(t, err)

	_, err = page.Goto("https://example.com")
	require.NoError(t, err)

	_, err = page.Evaluate(`localStorage.setItem('theme', 'dark')`)
	require.NoError(t, err)
	_, err = page.Evaluate(`localStorage.setItem('lang', 'en')`)
	require.NoError(t, err)
	_, err = page.Evaluate(`sessionStorage.setItem('token', 'abc')`)
	require.NoError(t, err)

	ls, err := page.Evaluate(`Object.entries(localStorage).map(([k,v])=>({key:k,value:v}))`)
	require.NoError(t, err)

	arr, ok := ls.([]any)
	require.True(t, ok)
	assert.Len(t, arr, 2)

	lsMap := make(map[string]string)
	for _, item := range arr {
		m := item.(map[string]any)
		lsMap[fmt.Sprint(m["key"])] = fmt.Sprint(m["value"])
	}
	assert.Equal(t, "dark", lsMap["theme"])
	assert.Equal(t, "en", lsMap["lang"])

	ss, err := page.Evaluate(`Object.entries(sessionStorage).map(([k,v])=>({key:k,value:v}))`)
	require.NoError(t, err)

	ssArr, ok := ss.([]any)
	require.True(t, ok)
	assert.Len(t, ssArr, 1)
	ssMap := make(map[string]string)
	for _, item := range ssArr {
		m := item.(map[string]any)
		ssMap[fmt.Sprint(m["key"])] = fmt.Sprint(m["value"])
	}
	assert.Equal(t, "abc", ssMap["token"])
}
