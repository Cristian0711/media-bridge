import { prefetchDiscover } from '$lib/data/browse-cache';
import { prefetchMediaList } from '$lib/data/media-list-cache';
import { prefetchRequestsList } from '$lib/data/requests-list-cache';

let prefetched = false;

/** Warm discover, library, and requests list data after landing on home. */
export function prefetchTabLists(): void {
  if (prefetched || typeof window === 'undefined') return;
  prefetched = true;

  const run = () => {
    prefetchDiscover();
    prefetchMediaList('yours', 1);
    prefetchMediaList('all', 1);
    prefetchRequestsList('yours', 1);
    prefetchRequestsList('all', 1);
  };

  if ('requestIdleCallback' in window) {
    requestIdleCallback(run, { timeout: 2500 });
  } else {
    setTimeout(run, 100);
  }
}
