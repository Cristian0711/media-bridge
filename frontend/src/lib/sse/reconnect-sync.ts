import { prefetchMediaList } from '$lib/data/media-list-cache';
import { prefetchRequestsList } from '$lib/data/requests-list-cache';
import { invalidateMediaListCache } from '$lib/data/media-list-cache';
import { invalidateRequestsListCache } from '$lib/data/requests-list-cache';
import { bumpMediaListVersion, bumpRequestsListVersion } from '$lib/sse/live-updates';

/**
 * After SSE was offline (background tab, network blip), lists may be stale.
 * Invalidate caches, notify mounted pages, and warm the first page of each tab.
 */
export function syncListsAfterSseReconnect(): void {
  invalidateMediaListCache();
  invalidateRequestsListCache();
  bumpMediaListVersion();
  bumpRequestsListVersion();

  prefetchMediaList('yours', 1);
  prefetchMediaList('all', 1);
  prefetchRequestsList('yours', 1);
  prefetchRequestsList('all', 1);
}
