package search

import (
	"context"
	"fmt"
	"sync"
)

// BrowseListRow is one horizontal discover row with page-1 results.
type BrowseListRow struct {
	ID         string   `json:"id"`
	Title      string   `json:"title"`
	Page       int      `json:"page"`
	TotalPages int      `json:"total_pages"`
	Results    []Result `json:"results"`
}

// BrowseCatalog is a full set of discover rows (movies + series) for one scope.
type BrowseCatalog struct {
	Lists []BrowseListRow `json:"lists"`
}

func serviceCatalogCacheKey(serviceID string) string {
	return fmt.Sprintf("catalog:service:%s", serviceID)
}

const globalCatalogCacheKey = "catalog:global"

// BrowseServiceCatalog returns all list metadata and page-1 results for one streaming service.
func (s *Service) BrowseServiceCatalog(ctx context.Context, serviceID string) (*BrowseCatalog, error) {
	if !isKnownService(serviceID) {
		return nil, fmt.Errorf("unknown service: %s", serviceID)
	}
	key := serviceCatalogCacheKey(serviceID)
	if cached, ok := s.browseCache.getCatalog(key); ok {
		return cached, nil
	}
	catalog, err := s.buildCatalog(ctx, serviceID)
	if err != nil {
		return nil, err
	}
	s.browseCache.setCatalog(key, catalog)
	return catalog, nil
}

// BrowseGlobalCatalog returns global discover rows (e.g. trending) with page-1 results.
func (s *Service) BrowseGlobalCatalog(ctx context.Context) (*BrowseCatalog, error) {
	if cached, ok := s.browseCache.getCatalog(globalCatalogCacheKey); ok {
		return cached, nil
	}
	metas := s.BrowseGlobalLists()
	catalog, err := s.buildCatalogFromMetas(ctx, metas)
	if err != nil {
		return nil, err
	}
	s.browseCache.setCatalog(globalCatalogCacheKey, catalog)
	return catalog, nil
}

func (s *Service) buildCatalog(ctx context.Context, serviceID string) (*BrowseCatalog, error) {
	metas, err := s.BrowseServiceLists(serviceID)
	if err != nil {
		return nil, err
	}
	return s.buildCatalogFromMetas(ctx, metas)
}

func (s *Service) buildCatalogFromMetas(ctx context.Context, metas []BrowseListMeta) (*BrowseCatalog, error) {
	if len(metas) == 0 {
		return &BrowseCatalog{Lists: []BrowseListRow{}}, nil
	}

	rows := make([]BrowseListRow, len(metas))
	var wg sync.WaitGroup
	sem := make(chan struct{}, browseCatalogConcurrent)
	errCh := make(chan error, len(metas))

	for i, meta := range metas {
		wg.Add(1)
		go func(i int, meta BrowseListMeta) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			page, err := s.Browse(ctx, meta.ID, 1)
			if err != nil {
				errCh <- err
				return
			}
			results := page.Results
			if results == nil {
				results = []Result{}
			}
			rows[i] = BrowseListRow{
				ID:         meta.ID,
				Title:      meta.Title,
				Page:       page.Page,
				TotalPages: page.TotalPages,
				Results:    results,
			}
		}(i, meta)
	}

	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			return nil, err
		}
	}

	return &BrowseCatalog{Lists: rows}, nil
}

const browseCatalogConcurrent = 4

func (s *Service) warmServiceCatalog(ctx context.Context, serviceID string) error {
	_, err := s.BrowseServiceCatalog(ctx, serviceID)
	return err
}

func (s *Service) warmGlobalCatalog(ctx context.Context) error {
	_, err := s.BrowseGlobalCatalog(ctx)
	return err
}
