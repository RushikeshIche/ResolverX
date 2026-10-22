package metrics

import (
	"sync"
	"time"
)

// QueryLogEntry represents a single DNS resolution event.
type QueryLogEntry struct {
	Timestamp time.Time     `json:"timestamp"`
	ClientIP  string        `json:"client_ip"`
	Domain    string        `json:"domain"`
	QType     string        `json:"qtype"`
	Source    string        `json:"source"` // "local", "cache", "upstream", "blocked", "error"
	RCode     string        `json:"rcode"`  // "NOERROR", "NXDOMAIN", "SERVFAIL", etc.
	Duration  time.Duration `json:"duration"`
}

// Stats holds snapshot counts for dashboard display.
type Stats struct {
	TotalQueries    uint64  `json:"total_queries"`
	LocalHits       uint64  `json:"local_hits"`
	CacheHits       uint64  `json:"cache_hits"`
	UpstreamQueries uint64  `json:"upstream_queries"`
	Errors          uint64  `json:"errors"`
	HitRatio        float64 `json:"hit_ratio"` // percentage of local + cache hits
}

// Collector provides thread-safe aggregation of metrics and live query logs.
type Collector struct {
	mu              sync.RWMutex
	totalQueries    uint64
	localHits       uint64
	cacheHits       uint64
	upstreamQueries uint64
	errors          uint64
	maxLogEntries   int
	recentLogs      []QueryLogEntry
	listeners       []func(entry QueryLogEntry)
}

// NewCollector creates a metrics collector with bounded query log history.
func NewCollector(maxLogEntries int) *Collector {
	if maxLogEntries <= 0 {
		maxLogEntries = 100
	}
	return &Collector{
		maxLogEntries: maxLogEntries,
		recentLogs:    make([]QueryLogEntry, 0, maxLogEntries),
		listeners:     make([]func(QueryLogEntry), 0),
	}
}

// RecordQuery tracks a resolution event and appends to the log ring buffer.
func (c *Collector) RecordQuery(entry QueryLogEntry) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.totalQueries++
	switch entry.Source {
	case "local":
		c.localHits++
	case "cache":
		c.cacheHits++
	case "upstream":
		c.upstreamQueries++
	case "error":
		c.errors++
	}

	// Append to bounded log slice
	if len(c.recentLogs) >= c.maxLogEntries {
		c.recentLogs = c.recentLogs[1:]
	}
	c.recentLogs = append(c.recentLogs, entry)

	for _, l := range c.listeners {
		if l != nil {
			go l(entry)
		}
	}
}

// OnLogEntry registers a live stream listener for query events.
func (c *Collector) OnLogEntry(listener func(QueryLogEntry)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.listeners = append(c.listeners, listener)
}

// GetStats returns an atomic snapshot of current resolver metrics.
func (c *Collector) GetStats() Stats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var ratio float64
	if c.totalQueries > 0 {
		ratio = float64(c.localHits+c.cacheHits) / float64(c.totalQueries) * 100.0
	}

	return Stats{
		TotalQueries:    c.totalQueries,
		LocalHits:       c.localHits,
		CacheHits:       c.cacheHits,
		UpstreamQueries: c.upstreamQueries,
		Errors:          c.errors,
		HitRatio:        ratio,
	}
}

// GetRecentLogs returns a copy of the recent query log entries.
func (c *Collector) GetRecentLogs() []QueryLogEntry {
	c.mu.RLock()
	defer c.mu.RUnlock()

	copied := make([]QueryLogEntry, len(c.recentLogs))
	copy(copied, c.recentLogs)
	return copied
}

// Reset clears metrics and query history.
func (c *Collector) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.totalQueries = 0
	c.localHits = 0
	c.cacheHits = 0
	c.upstreamQueries = 0
	c.errors = 0
	c.recentLogs = make([]QueryLogEntry, 0, c.maxLogEntries)
}
