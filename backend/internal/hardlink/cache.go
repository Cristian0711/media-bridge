package hardlink

import (
	"context"
	"sync"
	"time"

	"github.com/Cristian0711/media-bridge/backend/internal/media"
)

const defaultProgressCacheTTL = 3 * time.Second

type cachingService struct {
	inner Service
	ttl   time.Duration

	mu    sync.Mutex
	cache map[uint]cachedProgress
}

type cachedProgress struct {
	progress  *Progress
	expiresAt time.Time
}

func withProgressCache(inner Service, ttl time.Duration) Service {
	if ttl <= 0 {
		ttl = defaultProgressCacheTTL
	}
	return &cachingService{
		inner: inner,
		ttl:   ttl,
		cache: make(map[uint]cachedProgress),
	}
}

func (c *cachingService) Hardlink(ctx context.Context, mediaID uint) error {
	err := c.inner.Hardlink(ctx, mediaID)
	c.invalidate(mediaID)
	return err
}

func (c *cachingService) IsComplete(ctx context.Context, mediaID uint) (bool, error) {
	p, err := c.Progress(ctx, mediaID)
	if err != nil {
		return false, err
	}
	return p.Complete, nil
}

func (c *cachingService) Progress(ctx context.Context, mediaID uint) (*Progress, error) {
	now := time.Now()

	c.mu.Lock()
	if entry, ok := c.cache[mediaID]; ok && now.Before(entry.expiresAt) {
		p := entry.progress
		c.mu.Unlock()
		return p.clone(), nil
	}
	c.mu.Unlock()

	p, err := c.inner.Progress(ctx, mediaID)
	if err != nil {
		return nil, err
	}

	c.store(mediaID, p, now)
	return p.clone(), nil
}

func (c *cachingService) ProgressForMedia(ctx context.Context, row *media.Media) (*Progress, error) {
	if row == nil {
		return &Progress{}, nil
	}
	now := time.Now()
	id := row.ID

	c.mu.Lock()
	if entry, ok := c.cache[id]; ok && now.Before(entry.expiresAt) {
		p := entry.progress
		c.mu.Unlock()
		return p.clone(), nil
	}
	c.mu.Unlock()

	p, err := c.inner.ProgressForMedia(ctx, row)
	if err != nil {
		return nil, err
	}

	c.store(id, p, now)
	return p.clone(), nil
}

func (c *cachingService) store(mediaID uint, p *Progress, now time.Time) {
	c.mu.Lock()
	c.cache[mediaID] = cachedProgress{progress: p, expiresAt: now.Add(c.ttl)}
	c.mu.Unlock()
}

func (c *cachingService) invalidate(mediaID uint) {
	c.mu.Lock()
	delete(c.cache, mediaID)
	c.mu.Unlock()
}
