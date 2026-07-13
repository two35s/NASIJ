package runtime

import (
	"fmt"
	"sync"

	"github.com/mxschmitt/playwright-go"
)

type Collector struct {
	mu      sync.Mutex
	pw      *playwright.Playwright
	browser playwright.Browser
	closed  bool
}

func New() (*Collector, error) {
	pw, err := playwright.Run(
		&playwright.RunOptions{
			SkipInstallBrowsers: false,
			Browsers:            []string{"chromium"},
			Verbose:             false,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("runtime: start playwright: %w", err)
	}

	browser, err := pw.Chromium.Launch(
		playwright.BrowserTypeLaunchOptions{
			Headless: playwright.Bool(true),
		},
	)
	if err != nil {
		pw.Stop()
		return nil, fmt.Errorf("runtime: launch chromium: %w", err)
	}

	return &Collector{pw: pw, browser: browser}, nil
}

func (c *Collector) CollectURL(url string) (*Result, error) {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil, fmt.Errorf("runtime: collector is closed")
	}
	c.mu.Unlock()

	result := &Result{URL: url}

	bctx, err := c.browser.NewContext()
	if err != nil {
		return nil, fmt.Errorf("runtime: create context: %w", err)
	}
	defer bctx.Close()

	page, err := bctx.NewPage()
	if err != nil {
		return nil, fmt.Errorf("runtime: create page: %w", err)
	}

	page.OnRequest(func(req playwright.Request) {
		rec := RequestRecord{
			URL:          req.URL(),
			Method:       req.Method(),
			ResourceType: req.ResourceType(),
		}
		result.Requests = append(result.Requests, rec)
	})

	page.OnResponse(func(resp playwright.Response) {
		reqURL := resp.Request().URL()
		for i := range result.Requests {
			if result.Requests[i].URL == reqURL && result.Requests[i].StatusCode == 0 {
				result.Requests[i].StatusCode = resp.Status()
				result.Requests[i].Headers = resp.Headers()
				break
			}
		}
	})

	page.OnWebSocket(func(ws playwright.WebSocket) {
		result.WebSockets = append(result.WebSockets, WebSocketRecord{URL: ws.URL()})
	})

	if _, err = page.Goto(url, playwright.PageGotoOptions{
		WaitUntil: playwright.WaitUntilStateLoad,
		Timeout:   playwright.Float(30000),
	}); err != nil {
		return result, nil
	}

	if cookies, err := bctx.Cookies(); err == nil {
		for _, c := range cookies {
			result.Cookies = append(result.Cookies, CookieRecord{
				Name: c.Name, Value: c.Value, Domain: c.Domain,
				Path: c.Path, Secure: c.Secure, HttpOnly: c.HttpOnly,
			})
		}
	}

	if ls, err := page.Evaluate(`Object.entries(localStorage).map(([k,v])=>({key:k,value:v}))`); err == nil {
		if arr, ok := ls.([]any); ok {
			for _, item := range arr {
				if m, ok := item.(map[string]any); ok {
					result.LocalStorage = append(result.LocalStorage, StorageRecord{
						Key: fmt.Sprint(m["key"]), Value: fmt.Sprint(m["value"]),
					})
				}
			}
		}
	}

	if ss, err := page.Evaluate(`Object.entries(sessionStorage).map(([k,v])=>({key:k,value:v}))`); err == nil {
		if arr, ok := ss.([]any); ok {
			for _, item := range arr {
				if m, ok := item.(map[string]any); ok {
					result.SessionStorage = append(result.SessionStorage, StorageRecord{
						Key: fmt.Sprint(m["key"]), Value: fmt.Sprint(m["value"]),
					})
				}
			}
		}
	}

	if idb, err := page.Evaluate(indexedDBJS); err == nil {
		if arr, ok := idb.([]any); ok {
			for _, item := range arr {
				if m, ok := item.(map[string]any); ok {
					rec := IndexedDBRecord{Database: fmt.Sprint(m["database"])}
					if v, ok := m["version"]; ok {
						switch vv := v.(type) {
						case float64:
							rec.Version = int64(vv)
						case int64:
							rec.Version = vv
						}
					}
					if stores, ok := m["stores"].([]any); ok {
						for _, s := range stores {
							if sm, ok := s.(map[string]any); ok {
								store := IndexedDBStore{Name: fmt.Sprint(sm["name"])}
								if records, ok := sm["records"].([]any); ok {
									store.Records = records
								}
								rec.Stores = append(rec.Stores, store)
							}
						}
					}
					result.IndexedDB = append(result.IndexedDB, rec)
				}
			}
		}
	}

	if sw, err := page.Evaluate(serviceWorkerJS); err == nil {
		if arr, ok := sw.([]any); ok {
			for _, item := range arr {
				if m, ok := item.(map[string]any); ok {
					result.ServiceWorkers = append(result.ServiceWorkers, ServiceWorkerRecord{
						URL: fmt.Sprint(m["url"]), Scope: fmt.Sprint(m["scope"]),
						Active: m["active"] == true,
					})
				}
			}
		}
	}

	return result, nil
}

func (c *Collector) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil
	}
	c.closed = true

	var errs []error
	if c.browser != nil {
		if err := c.browser.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if c.pw != nil {
		if err := c.pw.Stop(); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("runtime: close: %v", errs)
	}
	return nil
}

const indexedDBJS = `(() => {
  try {
    if (!indexedDB || typeof indexedDB.databases !== 'function') return [];
    return indexedDB.databases().then(dbs => Promise.all(dbs.map(info =>
      new Promise((res, rej) => {
        const r = indexedDB.open(info.name);
        r.onsuccess = () => {
          const db = r.result;
          const stores = [];
          for (const name of db.objectStoreNames) {
            const tx = db.transaction(name, 'readonly');
            const store = tx.objectStore(name);
            stores.push(new Promise((res2, rej2) => {
              const r2 = store.getAll();
              r2.onsuccess = () => res2({name, records: r2.result});
              r2.onerror = () => rej2(r2.error);
            }));
          }
          Promise.all(stores).then(s => { res({database: info.name, version: info.version, stores: s}); db.close(); });
        };
        r.onerror = () => { rej(r.error); };
      })
    )));
  } catch(e) { return []; }
})()`

const serviceWorkerJS = `(() => {
  try {
    if (!('serviceWorker' in navigator)) return [];
    return navigator.serviceWorker.getRegistrations().then(regs =>
      regs.map(r => ({
        url: (r.active && r.active.scriptURL) || (r.installing && r.installing.scriptURL) || (r.waiting && r.waiting.scriptURL) || '',
        scope: r.scope, active: !!r.active
      }))
    );
  } catch(e) { return []; }
})()`
