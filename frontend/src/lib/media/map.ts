import type { ApiMedia, MediaLibraryItem } from '$lib/types/media-library';

export { formatSizeGB } from '$lib/utils/format-size';
export { formatRelativeTime } from '$lib/utils/format-time';
export { normalizePosterUrl as posterUrl } from '$lib/utils/poster-url';

export function toLibraryItem(row: ApiMedia): MediaLibraryItem {
  let poster_url: string | undefined;
  let imdb_id: string | undefined;
  let tmdb_id: string | undefined;
  let tvdb_id: string | undefined;
  let season: number | undefined;
  let episode: number | undefined;

  if (row.type === 'movie' && row.movie) {
    poster_url = row.movie.poster_url ?? undefined;
    imdb_id = row.movie.imdb_id || undefined;
    tmdb_id = row.movie.tmdb_id || undefined;
  } else if (row.show_entry) {
    season = row.show_entry.season ?? undefined;
    episode = row.show_entry.episode ?? undefined;
    if (row.show_entry.show) {
      poster_url = row.show_entry.show.poster_url ?? undefined;
      imdb_id = row.show_entry.show.imdb_id || undefined;
      tvdb_id = row.show_entry.show.tvdb_id || undefined;
    }
  }

  return {
    id: row.id,
    title: row.name,
    type: row.type,
    poster_url,
    imdb_id,
    tmdb_id,
    tvdb_id,
    season,
    episode,
    username: row.username,
    quality: row.quality,
    indexer: row.indexer,
    size_bytes: row.size_bytes && row.size_bytes > 0 ? row.size_bytes : undefined,
    created_at: row.created_at,
  };
}

export function mediaTypeLabel(type: MediaLibraryItem['type']): string {
  switch (type) {
    case 'movie':
      return 'Movie';
    case 'show_episode':
      return 'Episode';
    case 'show_season':
      return 'Season';
    case 'show_full':
      return 'Full Series';
    default:
      return type;
  }
}

export function mediaDetail(item: MediaLibraryItem): string {
  if (item.type === 'show_episode' && item.season != null && item.episode != null) {
    return `S${String(item.season).padStart(2, '0')}E${String(item.episode).padStart(2, '0')}`;
  }
  if (item.type === 'show_season' && item.season != null) {
    return `Season ${item.season}`;
  }
  return '';
}
