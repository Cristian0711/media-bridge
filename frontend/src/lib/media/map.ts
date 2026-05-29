import type { ApiMedia, MediaLibraryItem } from '$lib/types/media-library';

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

/** Formats torrent size in gigabytes (binary GB). */
export function formatSizeGB(bytes?: number): string | null {
  if (!bytes || bytes <= 0) return null;
  const gb = bytes / 1024 ** 3;
  if (gb >= 100) return `${Math.round(gb)} GB`;
  if (gb >= 10) return `${gb.toFixed(1)} GB`;
  return `${gb.toFixed(2)} GB`;
}

export function posterUrl(url?: string | null): string | undefined {
  if (!url) return undefined;
  if (url.startsWith('http://') || url.startsWith('https://')) return url;
  if (url.startsWith('//')) return `https:${url}`;
  return `https://${url}`;
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

export function formatRelativeTime(dateString: string): string {
  try {
    const date = new Date(dateString);
    if (Number.isNaN(date.getTime())) return 'Unknown';

    const diffMs = Date.now() - date.getTime();
    const diffMins = Math.floor(diffMs / 60000);
    const diffHours = Math.floor(diffMs / 3600000);
    const diffDays = Math.floor(diffMs / 86400000);

    if (diffMins < 1) return 'Just now';
    if (diffMins < 60) return `${diffMins}m ago`;
    if (diffHours < 24) return `${diffHours}h ago`;
    if (diffDays < 7) return `${diffDays}d ago`;

    return date.toLocaleDateString('en-US', { month: 'short', day: 'numeric', year: 'numeric' });
  } catch {
    return 'Unknown';
  }
}
