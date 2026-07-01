import { getAllRequests, getMyRequests } from '$lib/requests/list-api';
import { fetchWithCache, getCached, invalidatePrefix, isFresh } from '$lib/data/list-cache';
import { preloadPostersFromRequestsResponse } from '$lib/utils/poster-preload';
import type { PaginatedRequestsResponse } from '$lib/types/request';
import type { RequestsView } from '$lib/data/requests-view';

export const REQUESTS_LIST_PAGE_SIZE = 20;

export function requestsListCacheKey(view: RequestsView, page: number): string {
  const scope = view === 'yours' ? 'my' : 'all';
  return `requests:list:${scope}:p${page}`;
}

function fetchRequestsPage(view: RequestsView, page: number): Promise<PaginatedRequestsResponse> {
  const params = { page, pageSize: REQUESTS_LIST_PAGE_SIZE };
  return view === 'yours' ? getMyRequests(params) : getAllRequests(params);
}

export function prefetchRequestsList(view: RequestsView, page = 1): void {
  if (page !== 1) return;
  const key = requestsListCacheKey(view, page);
  const cached = getCached<PaginatedRequestsResponse>(key);
  if (cached) {
    preloadPostersFromRequestsResponse(cached);
    return;
  }
  if (isFresh(key)) return;
  void fetchWithCache(key, () => fetchRequestsPage(view, page))
    .then((res) => preloadPostersFromRequestsResponse(res))
    .catch(() => {});
}

export async function loadRequestsListCached(
  view: RequestsView,
  page: number,
  options?: { force?: boolean },
): Promise<PaginatedRequestsResponse> {
  const key = requestsListCacheKey(view, page);
  return fetchWithCache(key, () => fetchRequestsPage(view, page), options);
}

export function invalidateRequestsListCache(): void {
  invalidatePrefix('requests:');
}

export { isFresh as isRequestsListFresh };
