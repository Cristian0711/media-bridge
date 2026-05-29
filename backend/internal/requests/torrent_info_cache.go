package requests

import (
	"sync"
	"time"
)

const defaultTorrentInfoCacheTTL = 3 * time.Second

type torrentInfoCache struct {
	ttl   time.Duration
	mu    sync.Mutex
	items map[uint]cachedTorrentInfo
}

type cachedTorrentInfo struct {
	info      *RequestTorrentInfo
	expiresAt time.Time
}

func newTorrentInfoCache(ttl time.Duration) *torrentInfoCache {
	if ttl <= 0 {
		ttl = defaultTorrentInfoCacheTTL
	}
	return &torrentInfoCache{
		ttl:   ttl,
		items: make(map[uint]cachedTorrentInfo),
	}
}

func (c *torrentInfoCache) get(requestID uint) (*RequestTorrentInfo, bool) {
	now := time.Now()
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.items[requestID]
	if !ok || now.After(entry.expiresAt) {
		delete(c.items, requestID)
		return nil, false
	}
	return entry.info, true
}

func (c *torrentInfoCache) set(requestID uint, info *RequestTorrentInfo) {
	c.mu.Lock()
	c.items[requestID] = cachedTorrentInfo{
		info:      info,
		expiresAt: time.Now().Add(c.ttl),
	}
	c.mu.Unlock()
}

func (c *torrentInfoCache) invalidate(requestID uint) {
	c.mu.Lock()
	delete(c.items, requestID)
	c.mu.Unlock()
}
