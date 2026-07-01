import { callApi } from '$lib/api/client';
import { buildQuery } from '$lib/api/query';
import type {
  IndexerSettingsResponse,
  MovieSearchResponse,
  ShowSearchResponse,
} from '$lib/types/indexer';

export type IndexerSearchParams = {
  imdb_id: string;
  season?: number;
  episode?: number;
  quality?: string;
};

function qs(params: IndexerSearchParams): string {
  return buildQuery({
    imdb_id: params.imdb_id,
    season: params.season || undefined,
    episode: params.episode || undefined,
    quality: params.quality || undefined,
  });
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

/** Admin: list indexers with their per-indexer freeleech configuration. */
export async function listIndexerSettings(): Promise<IndexerSettingsResponse> {
  return callApi<IndexerSettingsResponse>('/indexer/settings', { method: 'GET' });
}

/** Admin: set whether an indexer returns freeleech-only results. */
export async function updateIndexerSetting(
  indexerName: string,
  freeleechOnly: boolean,
): Promise<{ indexer_name: string; freeleech_only: boolean }> {
  return callApi('/indexer/settings', {
    method: 'PUT',
    body: JSON.stringify({ indexer_name: indexerName, freeleech_only: freeleechOnly }),
  });
}
