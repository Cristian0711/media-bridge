package search

import (
	"fmt"
	"sync"
	"time"
)

const browseCacheTTL = 24 * time.Hour

type browseCache struct {
	mu sync.RWMutex

	services        []BrowseService
	servicesExpires time.Time

	pages map[string]cachedBrowsePage
}

type cachedBrowsePage struct {
	page      SearchPage
	expiresAt time.Time
}

func newBrowseCache() *browseCache {
	return &browseCache{
		pages: make(map[string]cachedBrowsePage),
	}
}

func browsePageCacheKey(listID string, page int) string {
	return fmt.Sprintf("%s:%d", listID, page)
}

func (c *browseCache) getServices() ([]BrowseService, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.services == nil || time.Now().After(c.servicesExpires) {
		return nil, false
	}
	return cloneBrowseServices(c.services), true
}

func (c *browseCache) setServices(services []BrowseService) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.services = cloneBrowseServices(services)
	c.servicesExpires = time.Now().Add(browseCacheTTL)
}

func (c *browseCache) getListPage(listID string, page int) (*SearchPage, bool) {
	key := browsePageCacheKey(listID, page)
	c.mu.RLock()
	defer c.mu.RUnlock()
	entry, ok := c.pages[key]
	if !ok || time.Now().After(entry.expiresAt) {
		return nil, false
	}
	return cloneSearchPage(&entry.page), true
}

func (c *browseCache) setListPage(listID string, page int, result *SearchPage) {
	if result == nil {
		return
	}
	key := browsePageCacheKey(listID, page)
	c.mu.Lock()
	defer c.mu.Unlock()
	c.pages[key] = cachedBrowsePage{
		page:      *cloneSearchPage(result),
		expiresAt: time.Now().Add(browseCacheTTL),
	}
}

func cloneBrowseServices(in []BrowseService) []BrowseService {
	if in == nil {
		return nil
	}
	out := make([]BrowseService, len(in))
	copy(out, in)
	return out
}

func cloneSearchPage(in *SearchPage) *SearchPage {
	if in == nil {
		return nil
	}
	out := *in
	if in.Results != nil {
		out.Results = append([]Result{}, in.Results...)
	}
	return &out
}
