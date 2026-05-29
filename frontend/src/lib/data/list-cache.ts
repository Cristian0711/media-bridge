/** Default age before a cached list is considered stale (background refetch). */
export const LIST_CACHE_STALE_MS = 60_000;

type CacheEntry<T> = {
  data: T;
  fetchedAt: number;
};

const cache = new Map<string, CacheEntry<unknown>>();
const inflight = new Map<string, Promise<unknown>>();

export function getCached<T>(key: string): T | undefined {
  return cache.get(key)?.data as T | undefined;
}

export function isFresh(key: string, maxAgeMs = LIST_CACHE_STALE_MS): boolean {
  const entry = cache.get(key);
  if (!entry) return false;
  return Date.now() - entry.fetchedAt < maxAgeMs;
}

export function setCached<T>(key: string, data: T): void {
  cache.set(key, { data, fetchedAt: Date.now() });
}

export async function fetchWithCache<T>(
  key: string,
  fetcher: () => Promise<T>,
  options?: { force?: boolean },
): Promise<T> {
  if (!options?.force) {
    const existing = inflight.get(key);
    if (existing) return existing as Promise<T>;
  } else {
    inflight.delete(key);
  }

  const promise = fetcher()
    .then((data) => {
      setCached(key, data);
      inflight.delete(key);
      return data;
    })
    .catch((err) => {
      inflight.delete(key);
      throw err;
    });

  inflight.set(key, promise);
  return promise;
}

/** Warm cache without surfacing errors to the UI. */
export function prefetch<T>(key: string, fetcher: () => Promise<T>): void {
  if (isFresh(key)) return;
  void fetchWithCache(key, fetcher).catch(() => {});
}

export function invalidatePrefix(prefix: string): void {
  for (const key of [...cache.keys()]) {
    if (key.startsWith(prefix)) cache.delete(key);
  }
  for (const key of [...inflight.keys()]) {
    if (key.startsWith(prefix)) inflight.delete(key);
  }
}

export function clearListCache(): void {
  cache.clear();
  inflight.clear();
}
