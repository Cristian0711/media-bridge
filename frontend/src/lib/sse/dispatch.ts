import { invalidateDiscoverCache } from '$lib/data/browse-cache';
import { invalidateMediaListCache } from '$lib/data/media-list-cache';
import { invalidateRequestsListCache } from '$lib/data/requests-list-cache';
import { bumpMediaListVersion, bumpRequestsListVersion } from '$lib/sse/live-updates';
import type { AppSseEnvelope } from '$lib/sse/types';

/** Apply a server SSE envelope: invalidate caches and signal list pages to refresh. */
export function dispatchAppSseEvent(envelope: AppSseEnvelope | null): void {
  if (!envelope?.type) return;

  switch (envelope.type) {
    case 'media.added':
    case 'media.removed':
      invalidateMediaListCache();
      bumpMediaListVersion();
      // Library changed → discover availability flags are stale; drop the
      // cached catalog so the next visit refetches with fresh availability.
      invalidateDiscoverCache();
      break;
    case 'request.created':
    case 'request.status_changed':
      invalidateRequestsListCache();
      bumpRequestsListVersion();
      break;
    case 'connected':
    case 'heartbeat':
      break;
    default:
      break;
  }
}
