import { normalizePosterUrl, posterAtWidth, POSTER_THUMB_WIDTH, type PosterWidth } from '$lib/utils/poster-url';
import type { PaginatedMediaResponse } from '$lib/types/media-library';
import type { PaginatedRequestsResponse } from '$lib/types/request';

const loaded = new Set<string>();
const loading = new Set<string>();

export function isPosterPreloaded(url: string): boolean {
  return loaded.has(url);
}

/** Options shared by the poster preloaders. `width` MUST match the rendition the
 *  matching `<img>` requests, or the preloaded entry won't be a cache hit. */
type PreloadOptions = { width?: PosterWidth; limit?: number };

export function preloadPosterUrls(
  urls: Iterable<string | undefined | null>,
  options: PreloadOptions = {},
): void {
  if (typeof window === 'undefined') return;

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
