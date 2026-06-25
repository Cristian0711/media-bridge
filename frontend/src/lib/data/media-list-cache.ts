import * as mediaApi from '$lib/media/api';
import { fetchWithCache, getCached, invalidatePrefix, isFresh } from '$lib/data/list-cache';
import { preloadPostersFromMediaResponse } from '$lib/utils/poster-preload';
import type { PaginatedMediaResponse } from '$lib/types/media-library';
import type { LibraryView } from '$lib/data/library-view';

export const MEDIA_LIST_PAGE_SIZE = 20;

export function mediaListCacheKey(
  view: LibraryView,
  page: number,
  query = '',
): string {
  const mode = query.trim() ? 'search' : 'list';
  const scope = view === 'yours' ? 'my' : 'all';
  const q = query.trim() ? `:${encodeURIComponent(query.trim())}` : '';
  return `media:${mode}:${scope}:p${page}${q}`;
}

function fetchMediaPage(
  view: LibraryView,
  page: number,
  query: string,
): Promise<PaginatedMediaResponse> {
  const params = { page, pageSize: MEDIA_LIST_PAGE_SIZE };
  const q = query.trim();
  if (q) {
    return view === 'yours'
      ? mediaApi.searchMyMedia(q, params)
      : mediaApi.searchAllMedia(q, params);
  }
  return view === 'yours' ? mediaApi.getMyMedia(params) : mediaApi.getAllMedia(params);
}

export function prefetchMediaList(view: LibraryView, page = 1, query = ''): void {
  if (page !== 1 || query.trim()) return;
  const key = mediaListCacheKey(view, page, query);
  const cached = getCached<PaginatedMediaResponse>(key);
  if (cached) {
    preloadPostersFromMediaResponse(cached);
    return;
  }
  if (isFresh(key)) return;
  void fetchWithCache(key, () => fetchMediaPage(view, page, query))
    .then((res) => preloadPostersFromMediaResponse(res))
    .catch(() => {});
}

export async function loadMediaListCached(
  view: LibraryView,
  page: number,
  query: string,
  options?: { force?: boolean },
): Promise<PaginatedMediaResponse> {
  const key = mediaListCacheKey(view, page, query);
  return fetchWithCache(key, () => fetchMediaPage(view, page, query), options);
}

export function invalidateMediaListCache(): void {
  invalidatePrefix('media:');
}

export { isFresh as isMediaListFresh } from '$lib/data/list-cache';
