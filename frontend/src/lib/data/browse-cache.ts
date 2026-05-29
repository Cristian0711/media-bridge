import {
  fetchBrowseGlobalLists,
  fetchBrowseList,
  fetchBrowseServiceLists,
  fetchBrowseServices,
  type BrowseListMeta,
  type BrowsePage,
  type BrowseService,
} from '$lib/browse/api';
import { fetchWithCache, getCached, invalidatePrefix, isFresh } from '$lib/data/list-cache';
import { preloadPosterUrls } from '$lib/utils/poster-preload';
import type { SearchResult } from '$lib/types/search';

/** Align with backend browse cache (24h). */
export const DISCOVER_CACHE_STALE_MS = 24 * 60 * 60 * 1000;

const SERVICES_KEY = 'discover:services';
const GLOBAL_LISTS_KEY = 'discover:global-lists';

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

function preloadBrowsePosters(results: SearchResult[]): void {
  preloadPosterUrls(posterUrlsFromResults(results));
}

export async function loadBrowseServicesCached(options?: {
  force?: boolean;
}): Promise<BrowseService[]> {
  return fetchWithCache(SERVICES_KEY, fetchBrowseServices, options);
}

export async function loadBrowseGlobalListsCached(options?: {
  force?: boolean;
}): Promise<BrowseListMeta[]> {
  return fetchWithCache(GLOBAL_LISTS_KEY, fetchBrowseGlobalLists, options);
}

export async function loadBrowseServiceListsCached(
  serviceId: string,
  options?: { force?: boolean },
): Promise<BrowseListMeta[]> {
  const key = discoverServiceListsCacheKey(serviceId);
  return fetchWithCache(key, () => fetchBrowseServiceLists(serviceId), options);
}

export async function loadBrowseListCached(
  listId: string,
  page = 1,
  options?: { force?: boolean },
): Promise<BrowsePage> {
  const key = discoverListCacheKey(listId, page);
  return fetchWithCache(key, () => fetchBrowseList(listId, page), options);
}

export function isDiscoverFresh(key: string): boolean {
  return isFresh(key, DISCOVER_CACHE_STALE_MS);
}

/** Warm discover data once per session (services, lists, all row page-1). */
export function prefetchDiscover(): void {
  if (typeof window === 'undefined') return;

  const run = async () => {
    try {
      const services = await loadBrowseServicesCached();
      const globalLists = await loadBrowseGlobalListsCached();

      const listIds: string[] = globalLists.map((l) => l.id);

      for (const svc of services) {
        const meta = await loadBrowseServiceListsCached(svc.id);
        for (const list of meta) {
          listIds.push(list.id);
        }
      }

      await Promise.all(
        [...new Set(listIds)].map(async (id) => {
          const page = await loadBrowseListCached(id, 1);
          preloadBrowsePosters(page.results);
        }),
      );
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
