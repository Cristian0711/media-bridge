import { callApi } from '$lib/api/client';
import type { MediaItem, MediaType } from '$lib/types/media';

export type AvailabilityItem = {
  type: 'movie' | 'show';
  imdb_id?: string;
  tmdb_id?: string;
  tvdb_id?: string;
  season?: number;
};

export type AvailabilityResult = {
  available: boolean;
  qualities: string[];
};

/** Builds an availability item from a search/discover media item. */
export function availabilityItem(
  item: MediaItem,
  mediaType: MediaType,
  season?: number,
): AvailabilityItem {
  return {
    type: mediaType === 'movies' ? 'movie' : 'show',
    imdb_id: item.ids.imdb || undefined,
    tmdb_id: item.ids.tmdb != null ? String(item.ids.tmdb) : undefined,
    tvdb_id: item.ids.tvdb != null ? String(item.ids.tvdb) : undefined,
    season,
  };
}

/**
 * One-shot fetch of the qualities already in the library for a single title
 * (season-scoped for shows). Used by the download/search dialogs; card
 * availability is delivered inline on search/browse responses instead.
 */
export async function fetchOwnedQualities(it: AvailabilityItem): Promise<string[]> {
  if (!it.imdb_id && !it.tmdb_id && !it.tvdb_id) return [];
  try {
    const res = await callApi<{ results: AvailabilityResult[] }>('/media/availability', {
      method: 'POST',
      body: JSON.stringify({ items: [it] }),
    });
    return res.results?.[0]?.qualities ?? [];
  } catch {
    return [];
  }
}
