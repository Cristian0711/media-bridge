import type { MediaItem } from '$lib/types/media';
import type { IndexerSearchParams } from '$lib/indexer/api';

export function indexerParamsFor(
  item: MediaItem,
  extra?: Partial<IndexerSearchParams>,
): IndexerSearchParams {
  if (!item.ids.imdb) {
    throw new Error('Missing IMDb ID');
  }
  return {
    imdb_id: item.ids.imdb,
    ...extra,
  };
}

export function posterFromItem(item: MediaItem): string | undefined {
  const path = item.images.poster?.[0];
  if (!path) return undefined;
  if (path.startsWith('http')) return path;
  if (path.startsWith('//')) return `https:${path}`;
  return `https://${path}`;
}
