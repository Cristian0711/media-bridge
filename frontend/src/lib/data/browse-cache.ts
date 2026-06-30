import {
  fetchBrowseGlobalCatalog,
  fetchBrowseServiceCatalog,
  fetchBrowseServices,
  type BrowseCatalog,
  type BrowseListRow,
  type BrowsePage,
  type BrowseService,
} from '$lib/browse/api';
import { fetchWithCache, getCached, invalidatePrefix, isFresh, setCached } from '$lib/data/list-cache';
import { preloadPosterUrls } from '$lib/utils/poster-preload';
import { POSTER_CARD_WIDTH } from '$lib/utils/poster-url';
import type { SearchResult } from '$lib/types/search';

/** Align with backend browse cache (24h). */
export const DISCOVER_CACHE_STALE_MS = 24 * 60 * 60 * 1000;

const SERVICES_KEY = 'discover:services';
const GLOBAL_CATALOG_KEY = 'discover:global-catalog';

export function discoverServiceCatalogCacheKey(serviceId: string): string {
  return `discover:catalog:${serviceId}`;
}

/** @deprecated Use discoverServiceCatalogCacheKey; kept for list-page cache keys */
export function discoverServiceListsCacheKey(serviceId: string): string {
  return `discover:service-lists:${serviceId}`;
}

export function discoverListCacheKey(listId: string, page = 1): string {
  return `discover:list:${listId}:p${page}`;
}

function posterUrlsFromResults(results: SearchResult[]): (string | undefined)[] {
  return results.map((r) => {
    if (r.type === 'movie') return r.movie?.images?.poster?.[0];
    return r.show?.images?.poster?.[0];
  });
}

function cacheCatalogRows(catalog: BrowseCatalog): void {
  for (const row of catalog.lists) {
    const page: BrowsePage = {
      results: row.results,
      page: row.page,
      totalPages: row.totalPages,
    };
    setCached(discoverListCacheKey(row.id, 1), page);
    preloadPosterUrls(posterUrlsFromResults(row.results), { width: POSTER_CARD_WIDTH });
  }
}

export async function loadBrowseServicesCached(options?: {
  force?: boolean;
}): Promise<BrowseService[]> {
  return fetchWithCache(SERVICES_KEY, fetchBrowseServices, options);
}

export async function loadBrowseGlobalCatalogCached(options?: {
  force?: boolean;
}): Promise<BrowseCatalog> {
  return fetchWithCache(
    GLOBAL_CATALOG_KEY,
    async () => {
      const catalog = await fetchBrowseGlobalCatalog();
      cacheCatalogRows(catalog);
      return catalog;
    },
    options,
  );
}

export async function loadBrowseServiceCatalogCached(
  serviceId: string,
  options?: { force?: boolean },
): Promise<BrowseCatalog> {
  const key = discoverServiceCatalogCacheKey(serviceId);
  return fetchWithCache(
    key,
    async () => {
      const catalog = await fetchBrowseServiceCatalog(serviceId);
      cacheCatalogRows(catalog);
      return catalog;
    },
    options,
  );
}

export function applyBrowseCatalogToRowState(
  catalog: BrowseCatalog,
): Record<string, { loading: boolean; error: string; results: SearchResult[] }> {
  const rowState: Record<string, { loading: boolean; error: string; results: SearchResult[] }> =
    {};
  for (const row of catalog.lists) {
    rowState[row.id] = { loading: false, error: '', results: row.results ?? [] };
  }
  return rowState;
}

export function listsFromCatalog(catalog: BrowseCatalog): BrowseListRow[] {
  return catalog.lists;
}

export function isDiscoverFresh(key: string): boolean {
  return isFresh(key, DISCOVER_CACHE_STALE_MS);
}

/** Warm discover: one catalog request per service + global. */
export function prefetchDiscover(): void {
  if (typeof window === 'undefined') return;

  const run = async () => {
    try {
      // Services must resolve first (we need their ids), but the global catalog
      // is independent — warm it alongside the per-service catalogs in parallel.
      const services = await loadBrowseServicesCached();
      await Promise.all([
        loadBrowseGlobalCatalogCached(),
        ...services.map((svc) => loadBrowseServiceCatalogCached(svc.id)),
      ]);
    } catch {
      // best-effort warm
    }
  };

  if ('requestIdleCallback' in window) {
    requestIdleCallback(() => void run(), { timeout: 4000 });
  } else {
    setTimeout(() => void run(), 150);
  }
}

export function invalidateDiscoverCache(): void {
  invalidatePrefix('discover:');
}
