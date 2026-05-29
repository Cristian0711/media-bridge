package search

import (
	"testing"
	"time"
)

func TestBrowseCacheServicesTTL(t *testing.T) {
	t.Parallel()
	c := newBrowseCache()
	c.setServices([]BrowseService{{ID: "netflix", Name: "Netflix"}})

	got, ok := c.getServices()
	if !ok || len(got) != 1 || got[0].ID != "netflix" {
		t.Fatalf("expected cached service, got ok=%v len=%d", ok, len(got))
	}

	c.mu.Lock()
	c.servicesExpires = time.Now().Add(-time.Second)
	c.mu.Unlock()

	if _, ok := c.getServices(); ok {
		t.Fatal("expected expired services cache miss")
	}
}

func TestBrowseCacheListPage(t *testing.T) {
	t.Parallel()
	c := newBrowseCache()
	page := &SearchPage{
		Results:    []Result{{Type: "movie", Score: 1}},
		Page:       1,
		TotalPages: 5,
	}
	c.setListPage("trending", 1, page)

	got, ok := c.getListPage("trending", 1)
	if !ok || got.Page != 1 || len(got.Results) != 1 {
		t.Fatalf("unexpected cache hit: %+v ok=%v", got, ok)
	}

	got.Results[0].Score = 99
	if cached, _ := c.getListPage("trending", 1); cached.Results[0].Score == 99 {
		t.Fatal("expected cached copy isolation")
	}
}

func TestAllBrowseWarmListIDs(t *testing.T) {
	t.Parallel()
	ids := allBrowseWarmListIDs()
	if len(ids) != 1+len(browseServices)*len(serviceListKinds) {
		t.Fatalf("unexpected warm list count: %d", len(ids))
	}
	seen := make(map[string]bool)
	for _, id := range ids {
		if seen[id] {
			t.Fatalf("duplicate warm id: %s", id)
		}
		seen[id] = true
	}
	if !seen["trending"] || !seen["netflix:movies"] {
		t.Fatalf("missing expected ids: %v", ids)
	}
}
