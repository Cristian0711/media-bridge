import { SvelteMap } from 'svelte/reactivity';
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

/** Reactive cache of availability results keyed by {@link availabilityKey}. */
const cache = new SvelteMap<string, AvailabilityResult>();
const inflight = new Set<string>();

let pending: AvailabilityItem[] = [];
let flushScheduled = false;

export function availabilityKey(it: AvailabilityItem): string {
  return [it.type, it.imdb_id ?? '', it.tmdb_id ?? '', it.tvdb_id ?? '', it.season ?? ''].join(':');
}

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

/** Reads a cached availability result (reactive); undefined until loaded. */
export function getAvailability(it: AvailabilityItem): AvailabilityResult | undefined {
  return cache.get(availabilityKey(it));
}

/**
 * Registers an item for availability lookup. Calls within the same tick are
 * coalesced into a single batched request, so a page full of cards costs one
 * round-trip. No-ops for items already cached or in flight.
 */
export function requestAvailability(it: AvailabilityItem): void {
  if (!it.imdb_id && !it.tmdb_id && !it.tvdb_id) return;
  const key = availabilityKey(it);
  if (cache.has(key) || inflight.has(key)) return;
  pending.push(it);
  if (!flushScheduled) {
    flushScheduled = true;
    queueMicrotask(flushPending);
  }
}

async function flushPending(): Promise<void> {
  flushScheduled = false;
  const batch: AvailabilityItem[] = [];
  const seen = new Set<string>();
  for (const it of pending) {
    const key = availabilityKey(it);
    if (seen.has(key) || cache.has(key) || inflight.has(key)) continue;
    seen.add(key);
    inflight.add(key);
    batch.push(it);
  }
  pending = [];
  if (batch.length === 0) return;

  try {
    const res = await callApi<{ results: AvailabilityResult[] }>('/media/availability', {
      method: 'POST',
      body: JSON.stringify({ items: batch }),
    });
    batch.forEach((it, i) => {
      const result = res.results?.[i];
      if (result) cache.set(availabilityKey(it), result);
    });
  } catch {
    // Best-effort: leave uncached so a later mount can retry.
  } finally {
    batch.forEach((it) => inflight.delete(availabilityKey(it)));
  }
}

/** One-shot fetch of the qualities already in the library for a single title. */
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
