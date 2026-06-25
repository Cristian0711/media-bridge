import type { MediaItem, MediaType } from '$lib/types/media';
import type { SearchResult } from '$lib/types/search';
import { normalizePosterUrl } from '$lib/utils/poster-url';

const POSTER_PLACEHOLDER = 'https://via.placeholder.com/56x80/1a1a2e/eee?text=No+Image';

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
  return normalizePosterUrl(poster?.[0]) ?? POSTER_PLACEHOLDER;
}
