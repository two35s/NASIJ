package runtime

type Result struct {
	URL            string                `json:"url"`
	Requests       []RequestRecord       `json:"requests"`
	WebSockets     []WebSocketRecord     `json:"web_sockets"`
	EventSources   []EventSourceRecord   `json:"event_sources"`
	Cookies        []CookieRecord        `json:"cookies"`
	LocalStorage   []StorageRecord       `json:"local_storage"`
	SessionStorage []StorageRecord       `json:"session_storage"`
	IndexedDB      []IndexedDBRecord     `json:"indexed_db"`
	ServiceWorkers []ServiceWorkerRecord `json:"service_workers"`
}

type RequestRecord struct {
	URL          string            `json:"url"`
	Method       string            `json:"method"`
	ResourceType string            `json:"resource_type"`
	StatusCode   int               `json:"status_code"`
	Headers      map[string]string `json:"headers"`
}

type WebSocketRecord struct {
	URL string `json:"url"`
}

type EventSourceRecord struct {
	URL string `json:"url"`
}

type CookieRecord struct {
	Name     string `json:"name"`
	Value    string `json:"value"`
	Domain   string `json:"domain"`
	Path     string `json:"path"`
	Secure   bool   `json:"secure"`
	HttpOnly bool   `json:"http_only"`
}

type StorageRecord struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type IndexedDBRecord struct {
	Database string           `json:"database"`
	Version  int64            `json:"version"`
	Stores   []IndexedDBStore `json:"stores"`
}

type IndexedDBStore struct {
	Name    string `json:"name"`
	Records []any  `json:"records"`
}

type ServiceWorkerRecord struct {
	URL    string `json:"url"`
	Scope  string `json:"scope"`
	Active bool   `json:"active"`
}
