import { ApiError } from '$lib/api/client';
import { getToken } from '$lib/auth/session';
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
  const headers = new Headers();
  const token = getToken();
  if (!token) {
    throw new ApiError(401, 'Not authenticated');
  }
  headers.set('Authorization', `Bearer ${token}`);

  const res = await fetch(`/api/v1/search/external-ids?${params.toString()}`, {
    headers,
    credentials: 'include',
  });
  const text = await res.text();
  let body: unknown = null;
  if (text) {
    try {
      body = JSON.parse(text);
    } catch {
      body = text;
    }
  }

  if (!res.ok) {
    const message =
      typeof body === 'object' && body !== null && 'error' in body
        ? String((body as { error: string }).error)
        : res.statusText || 'Request failed';
    throw new ApiError(res.status, message, body);
  }

  return body as MovieExternalIds | ShowExternalIds;
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
  const headers = new Headers();
  const token = getToken();
  if (!token) {
    throw new ApiError(401, 'Not authenticated');
  }
  headers.set('Authorization', `Bearer ${token}`);

  const res = await fetch(`/api/v1/search?${params.toString()}`, { headers, credentials: 'include' });
  const text = await res.text();
  let body: unknown = null;
  if (text) {
    try {
      body = JSON.parse(text);
    } catch {
      body = text;
    }
  }

  if (!res.ok) {
    const message =
      typeof body === 'object' && body !== null && 'error' in body
        ? String((body as { error: string }).error)
        : res.statusText || 'Request failed';
    throw new ApiError(res.status, message, body);
  }

  const pageNum = parseInt(res.headers.get('X-Search-Page') ?? String(page), 10) || page;
  const totalPages = parseInt(res.headers.get('X-Search-Total-Pages') ?? '1', 10) || 1;

  return {
    results: (body ?? []) as SearchResult[],
    page: pageNum,
    totalPages,
  };
}
