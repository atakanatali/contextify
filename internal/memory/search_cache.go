package memory

import (
	"encoding/json"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/atakanatali/contextify/internal/config"
)

type searchCacheEntry struct {
	results   []SearchResult
	expiresAt time.Time
}

type searchCache struct {
	enabled bool
	ttl     time.Duration
	max     int

	mu      sync.RWMutex
	entries map[string]searchCacheEntry
	order   []string
}

func newSearchCache(cfg config.SearchConfig) *searchCache {
	ttl := cfg.CacheTTL
	if ttl <= 0 {
		ttl = 30 * time.Second
	}
	maxEntries := cfg.CacheMaxEntries
	if maxEntries <= 0 {
		maxEntries = 500
	}

	return &searchCache{
		enabled: cfg.CacheEnabled,
		ttl:     ttl,
		max:     maxEntries,
		entries: make(map[string]searchCacheEntry),
		order:   make([]string, 0, maxEntries),
	}
}

func (c *searchCache) Enabled() bool {
	return c != nil && c.enabled
}

func (c *searchCache) Get(req SearchRequest) ([]SearchResult, bool) {
	if !c.Enabled() {
		return nil, false
	}
	key, err := cacheKey(req)
	if err != nil {
		return nil, false
	}

	now := time.Now()

	c.mu.RLock()
	entry, ok := c.entries[key]
	c.mu.RUnlock()
	if !ok {
		return nil, false
	}
	if now.After(entry.expiresAt) {
		c.mu.Lock()
		delete(c.entries, key)
		c.mu.Unlock()
		return nil, false
	}

	return cloneSearchResults(entry.results), true
}

func (c *searchCache) Set(req SearchRequest, results []SearchResult) {
	if !c.Enabled() {
		return
	}
	key, err := cacheKey(req)
	if err != nil {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.entries[key]; !exists {
		c.order = append(c.order, key)
	}

	c.entries[key] = searchCacheEntry{
		results:   cloneSearchResults(results),
		expiresAt: time.Now().Add(c.ttl),
	}

	for len(c.entries) > c.max && len(c.order) > 0 {
		oldest := c.order[0]
		c.order = c.order[1:]
		if _, exists := c.entries[oldest]; exists {
			delete(c.entries, oldest)
		}
	}
}

func (c *searchCache) InvalidateAll() {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = make(map[string]searchCacheEntry)
	c.order = c.order[:0]
}

func cacheKey(req SearchRequest) (string, error) {
	type keyPayload struct {
		Query         string       `json:"query"`
		Type          *MemoryType  `json:"type,omitempty"`
		Scope         *MemoryScope `json:"scope,omitempty"`
		ProjectID     *string      `json:"project_id,omitempty"`
		AgentSource   *string      `json:"agent_source,omitempty"`
		Tags          []string     `json:"tags,omitempty"`
		MinImportance *float32     `json:"min_importance,omitempty"`
		Limit         int          `json:"limit"`
		Offset        int          `json:"offset"`
	}

	tags := append([]string(nil), req.Tags...)
	sort.Strings(tags)

	payload := keyPayload{
		Query:         strings.ToLower(strings.TrimSpace(req.Query)),
		Type:          req.Type,
		Scope:         req.Scope,
		ProjectID:     req.ProjectID,
		AgentSource:   req.AgentSource,
		Tags:          tags,
		MinImportance: req.MinImportance,
		Limit:         req.Limit,
		Offset:        req.Offset,
	}

	b, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func cloneSearchResults(in []SearchResult) []SearchResult {
	out := make([]SearchResult, len(in))
	copy(out, in)
	return out
}
