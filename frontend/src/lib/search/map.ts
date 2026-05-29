import type { MediaItem, MediaType } from '$lib/types/media';
import type { SearchResult } from '$lib/types/search';

export function toMediaItem(result: SearchResult): { item: MediaItem; mediaType: MediaType } | null {
  if (result.type === 'movie' && result.movie) {
    return {
      mediaType: 'movies',
      item: {
        title: result.movie.title,
        year: result.movie.year,
        ids: result.movie.ids,
        images: result.movie.images,
      },
    };
  }

  if (result.type === 'show' && result.show) {
    return {
      mediaType: 'shows',
      item: {
        title: result.show.title,
        year: result.show.year,
        ids: result.show.ids,
        images: result.show.images,
      },
    };
  }

  return null;
}

export function posterUrl(poster: string[] | undefined): string {
  const path = poster?.[0];
  if (!path) {
    return 'https://via.placeholder.com/56x80/1a1a2e/eee?text=No+Image';
  }
  if (path.startsWith('http://') || path.startsWith('https://')) {
    return path;
  }
  return `https://${path}`;
}
