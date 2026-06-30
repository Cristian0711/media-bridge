import { normalizePosterUrl, posterAtWidth, POSTER_THUMB_WIDTH, type PosterWidth } from '$lib/utils/poster-url';
import type { PaginatedMediaResponse } from '$lib/types/media-library';
import type { PaginatedRequestsResponse } from '$lib/types/request';

const loaded = new Set<string>();
const loading = new Set<string>();

export function isPosterPreloaded(url: string): boolean {
  return loaded.has(url);
}

/**
 * Skip eager poster preloading on constrained connections. Preloading is a
 * bandwidth gamble — it's only a win when there's spare bandwidth to warm the
 * cache ahead of the `<img>`. Under Save-Data or a slow radio it backfires by
 * competing with the posters actually on screen, so we let those load on demand
 * (the `<img>` tags are still lazy) instead.
 */
function preloadDisabled(): boolean {
  if (typeof navigator === 'undefined') return false;
  const conn = (
    navigator as Navigator & {
      connection?: { saveData?: boolean; effectiveType?: string };
    }
  ).connection;
  if (!conn) return false;
  if (conn.saveData) return true;
  return conn.effectiveType === 'slow-2g' || conn.effectiveType === '2g';
}

/** Options shared by the poster preloaders. `width` MUST match the rendition the
 *  matching `<img>` requests, or the preloaded entry won't be a cache hit. */
type PreloadOptions = { width?: PosterWidth; limit?: number };

export function preloadPosterUrls(
  urls: Iterable<string | undefined | null>,
  options: PreloadOptions = {},
): void {
  if (typeof window === 'undefined' || preloadDisabled()) return;

  const { width, limit = 24 } = options;
  let count = 0;
  for (const raw of urls) {
    if (count >= limit) break;
    const url = width ? posterAtWidth(raw, width) : normalizePosterUrl(raw);
    if (!url || loaded.has(url) || loading.has(url)) continue;

    loading.add(url);
    const img = new Image();
    img.decoding = 'async';
    img.onload = () => {
      loaded.add(url);
      loading.delete(url);
    };
    img.onerror = () => loading.delete(url);
    img.src = url;
    count++;
  }
}

export function preloadPostersFromMediaResponse(
  res: PaginatedMediaResponse,
  options: PreloadOptions = { width: POSTER_THUMB_WIDTH },
): void {
  const urls: string[] = [];
  for (const row of res.media) {
    if (row.type === 'movie' && row.movie?.poster_url) {
      urls.push(row.movie.poster_url);
    } else if (row.show_entry?.show?.poster_url) {
      urls.push(row.show_entry.show.poster_url);
    }
  }
  preloadPosterUrls(urls, options);
}

export function preloadPostersFromRequestsResponse(
  res: PaginatedRequestsResponse,
  options: PreloadOptions = { width: POSTER_THUMB_WIDTH },
): void {
  preloadPosterUrls(
    (res.requests ?? []).map((r) => r.poster_url),
    options,
  );
}

export function clearPosterPreloadState(): void {
  loaded.clear();
  loading.clear();
}
