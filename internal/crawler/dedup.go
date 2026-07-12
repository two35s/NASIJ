package crawler

import "sync"

// DedupSet provides thread-safe URL deduplication.
type DedupSet struct {
	mu    sync.RWMutex
	urls  map[string]struct{}
	count int
}

func newDedupSet() *DedupSet {
	return NewDedupSet()
}

func NewDedupSet() *DedupSet {
	return &DedupSet{
		urls: make(map[string]struct{}),
	}
}

// Add returns true if the URL was added (not seen before), false if duplicate.
func (d *DedupSet) Add(rawurl string) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	if _, ok := d.urls[rawurl]; ok {
		return false
	}
	d.urls[rawurl] = struct{}{}
	d.count++
	return true
}

// Has returns true if the URL has already been seen.
func (d *DedupSet) Has(rawurl string) bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	_, ok := d.urls[rawurl]
	return ok
}

// Len returns the number of unique URLs seen.
func (d *DedupSet) Len() int {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.count
}

// Clear resets the dedup set.
func (d *DedupSet) Clear() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.urls = make(map[string]struct{})
	d.count = 0
}
