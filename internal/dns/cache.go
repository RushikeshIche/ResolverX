package dns

import (
	"container/list"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/miekg/dns"
)

type cacheItem struct {
	key       string
	msg       *dns.Msg
	expiresAt time.Time
}

// Cache provides a thread-safe, TTL-aware, LRU-evicting in-memory DNS response cache.
type Cache struct {
	mu         sync.RWMutex
	maxEntries int
	items      map[string]*list.Element
	lruList    *list.List
}

// NewCache initializes a new response cache with a bounded maximum capacity.
func NewCache(maxEntries int) *Cache {
	if maxEntries <= 0 {
		maxEntries = 1000
	}
	return &Cache{
		maxEntries: maxEntries,
		items:      make(map[string]*list.Element),
		lruList:    list.New(),
	}
}

func cacheKey(name string, qtype, qclass uint16) string {
	norm := strings.ToLower(strings.TrimSuffix(name, ".")) + "."
	return fmt.Sprintf("%s:%d:%d", norm, qtype, qclass)
}

// Get looks up a cached response message, decrementing RR TTLs according to elapsed time.
// Returns nil, false if not found or if the entry has expired.
func (c *Cache) Get(qName string, qtype, qclass uint16) (*dns.Msg, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	key := cacheKey(qName, qtype, qclass)
	elem, found := c.items[key]
	if !found {
		return nil, false
	}

	item := elem.Value.(*cacheItem)
	now := time.Now()

	// If entry has expired, purge and return miss
	if now.After(item.expiresAt) {
		c.removeElement(elem)
		return nil, false
	}

	// Move to front of LRU list
	c.lruList.MoveToFront(elem)

	remainingSeconds := uint32(item.expiresAt.Sub(now).Seconds())
	if remainingSeconds == 0 {
		c.removeElement(elem)
		return nil, false
	}

	// Return a deep copy with adjusted TTL
	copied := item.msg.Copy()
	adjustRRHeaderTTL(copied.Answer, remainingSeconds)
	adjustRRHeaderTTL(copied.Ns, remainingSeconds)
	adjustRRHeaderTTL(copied.Extra, remainingSeconds)

	return copied, true
}

func adjustRRHeaderTTL(rrs []dns.RR, remainingTTL uint32) {
	for _, rr := range rrs {
		h := rr.Header()
		if h.Ttl > remainingTTL {
			h.Ttl = remainingTTL
		}
	}
}

// Set stores a DNS response in the cache if eligible (min TTL > 0 and contains answers or NOERROR/NXDOMAIN).
func (c *Cache) Set(msg *dns.Msg) {
	if msg == nil || len(msg.Question) == 0 {
		return
	}

	// Calculate minimum TTL across answers
	minTTL := uint32(0)
	if len(msg.Answer) > 0 {
		minTTL = msg.Answer[0].Header().Ttl
		for _, rr := range msg.Answer[1:] {
			if rr.Header().Ttl < minTTL {
				minTTL = rr.Header().Ttl
			}
		}
	} else if msg.Rcode == dns.RcodeNameError || msg.Rcode == dns.RcodeSuccess {
		// Cap negative/empty cache entries to at most 60 seconds
		minTTL = 60
	}

	if minTTL == 0 {
		return
	}

	q := msg.Question[0]
	key := cacheKey(q.Name, q.Qtype, q.Qclass)

	c.mu.Lock()
	defer c.mu.Unlock()

	// If already present, update
	if elem, found := c.items[key]; found {
		item := elem.Value.(*cacheItem)
		item.msg = msg.Copy()
		item.expiresAt = time.Now().Add(time.Duration(minTTL) * time.Second)
		c.lruList.MoveToFront(elem)
		return
	}

	// Evict LRU entry if capacity exceeded
	for c.lruList.Len() >= c.maxEntries {
		oldest := c.lruList.Back()
		if oldest == nil {
			break
		}
		c.removeElement(oldest)
	}

	item := &cacheItem{
		key:       key,
		msg:       msg.Copy(),
		expiresAt: time.Now().Add(time.Duration(minTTL) * time.Second),
	}
	elem := c.lruList.PushFront(item)
	c.items[key] = elem
}

// Invalidate removes any cached entries matching the specified domain.
func (c *Cache) Invalidate(domain string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	norm := strings.ToLower(strings.TrimSuffix(domain, ".")) + "."

	for key, elem := range c.items {
		if strings.HasPrefix(key, norm+":") {
			c.removeElement(elem)
		}
	}
}

// Purge removes all entries from the cache.
func (c *Cache) Purge() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.items = make(map[string]*list.Element)
	c.lruList.Init()
}

// Size returns the count of items currently in cache.
func (c *Cache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.items)
}

func (c *Cache) removeElement(elem *list.Element) {
	c.lruList.Remove(elem)
	item := elem.Value.(*cacheItem)
	delete(c.items, item.key)
}
