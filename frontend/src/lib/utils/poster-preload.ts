import { normalizePosterUrl } from '$lib/utils/poster-url';
import type { PaginatedMediaResponse } from '$lib/types/media-library';
import type { PaginatedRequestsResponse } from '$lib/types/request';

const loaded = new Set<string>();
const loading = new Set<string>();

export function isPosterPreloaded(url: string): boolean {
  return loaded.has(url);
}

export function preloadPosterUrls(urls: Iterable<string | undefined | null>, limit = 24): void {
  if (typeof window === 'undefined') return;

  let count = 0;
  for (const raw of urls) {
    if (count >= limit) break;
    const url = normalizePosterUrl(raw);
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

export function preloadPostersFromMediaResponse(res: PaginatedMediaResponse, limit = 24): void {
  const urls: string[] = [];
  for (const row of res.media) {
    if (row.type === 'movie' && row.movie?.poster_url) {
      urls.push(row.movie.poster_url);
    } else if (row.show_entry?.show?.poster_url) {
      urls.push(row.show_entry.show.poster_url);
    }
  }
  preloadPosterUrls(urls, limit);
}

export function preloadPostersFromRequestsResponse(res: PaginatedRequestsResponse, limit = 24): void {
  preloadPosterUrls(
    (res.requests ?? []).map((r) => r.poster_url),
    limit,
  );
}

export function clearPosterPreloadState(): void {
  loaded.clear();
  loading.clear();
}
