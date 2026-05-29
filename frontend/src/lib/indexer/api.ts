import { callApi } from '$lib/api/client';
import type { MovieSearchResponse, ShowSearchResponse } from '$lib/types/indexer';

export type IndexerSearchParams = {
  imdb_id: string;
  season?: number;
  episode?: number;
  quality?: string;
};

function qs(params: IndexerSearchParams): string {
  const search = new URLSearchParams();
  search.set('imdb_id', params.imdb_id);
  if (params.season) search.set('season', String(params.season));
  if (params.episode) search.set('episode', String(params.episode));
  if (params.quality) search.set('quality', params.quality);
  return `?${search.toString()}`;
}

export async function searchMovies(params: IndexerSearchParams): Promise<MovieSearchResponse> {
  return callApi<MovieSearchResponse>(`/indexer/search/movies${qs(params)}`);
}

export async function searchShows(params: IndexerSearchParams): Promise<ShowSearchResponse> {
  return callApi<ShowSearchResponse>(`/indexer/search/shows${qs(params)}`);
}

export async function findBestMovie(params: IndexerSearchParams): Promise<{ movie: import('$lib/types/indexer').IndexerMovie }> {
  return callApi(`/indexer/search/movies/best${qs(params)}`);
}

export async function findBestShow(params: IndexerSearchParams): Promise<{ show: import('$lib/types/indexer').IndexerShow }> {
  return callApi(`/indexer/search/shows/best${qs(params)}`);
}
