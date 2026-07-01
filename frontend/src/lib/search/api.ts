import { callApi, callApiRaw } from '$lib/api/client';
import type { MediaItem, MediaType } from '$lib/types/media';
import type { SearchResult } from '$lib/types/search';

export type SearchPage = {
  results: SearchResult[];
  page: number;
  totalPages: number;
};

/** Movies: imdb_id + tmdb_id */
export type MovieExternalIds = {
  imdb_id: string;
  tmdb_id: number;
};

/** Shows: imdb_id required; tvdb_id when TMDB has it */
export type ShowExternalIds = {
  imdb_id: string;
  tvdb_id?: number;
  tmdb_id?: number;
};

async function fetchExternalIdsRaw(
  type: 'movie' | 'show',
  tmdbId: number,
): Promise<MovieExternalIds | ShowExternalIds> {
  const params = new URLSearchParams({ type, tmdb_id: String(tmdbId) });
  return callApi<MovieExternalIds | ShowExternalIds>(
    `/search/external-ids?${params.toString()}`,
  );
}

export async function fetchMovieExternalIds(tmdbId: number): Promise<MovieExternalIds> {
  const ids = await fetchExternalIdsRaw('movie', tmdbId);
  if (!('tmdb_id' in ids) || !ids.imdb_id) {
    throw new Error('Missing movie external IDs');
  }
  return ids as MovieExternalIds;
}

export async function fetchShowExternalIds(tmdbId: number): Promise<ShowExternalIds> {
  const ids = await fetchExternalIdsRaw('show', tmdbId);
  if (!ids.imdb_id) {
    throw new Error('Missing IMDb ID for show');
  }
  return ids as ShowExternalIds;
}

/** Fetches external IDs from TMDB when missing — for download and indexer search only. */
export async function resolveItemForIndexer(
  item: MediaItem,
  mediaType: MediaType,
): Promise<MediaItem> {
  const tmdb = item.ids.tmdb;
  if (!tmdb) {
    throw new Error('Missing TMDB ID');
  }

  if (mediaType === 'movies') {
    if (item.ids.imdb && item.ids.tmdb) {
      return item;
    }
    const ids = await fetchMovieExternalIds(tmdb);
    return {
      ...item,
      ids: {
        ...item.ids,
        imdb: ids.imdb_id,
        tmdb: ids.tmdb_id,
      },
    };
  }

  if (item.ids.imdb) {
    return item;
  }
  const ids = await fetchShowExternalIds(tmdb);
  return {
    ...item,
    ids: {
      ...item.ids,
      imdb: ids.imdb_id,
      ...(ids.tvdb_id != null && ids.tvdb_id > 0 ? { tvdb: ids.tvdb_id } : {}),
      tmdb: ids.tmdb_id ?? tmdb,
    },
  };
}

export async function searchMedia(query: string, page = 1): Promise<SearchPage> {
  const params = new URLSearchParams({ query, page: String(page) });
  const { data, headers } = await callApiRaw<SearchResult[] | null>(
    `/search?${params.toString()}`,
  );

  const pageNum = parseInt(headers.get('X-Search-Page') ?? String(page), 10) || page;
  const totalPages = parseInt(headers.get('X-Search-Total-Pages') ?? '1', 10) || 1;

  return {
    results: data ?? [],
    page: pageNum,
    totalPages,
  };
}
