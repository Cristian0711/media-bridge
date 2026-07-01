import { callApi } from '$lib/api/client';

type RequestAck = {
  status: string;
  message: string;
};

export async function removeMovie(mediaId: number): Promise<RequestAck> {
  return callApi<RequestAck>('/requests/movies/remove', {
    method: 'POST',
    body: JSON.stringify({ media_id: mediaId }),
  });
}

export async function removeShow(mediaId: number): Promise<RequestAck> {
  return callApi<RequestAck>('/requests/shows/remove', {
    method: 'POST',
    body: JSON.stringify({ media_id: mediaId }),
  });
}

export type MovieDownloadBody = {
  name: string;
  imdb_id: string;
  tmdb_id?: string;
  poster_url?: string;
  torrent_url: string;
  torrent_name: string;
  indexer: string;
  quality: string;
};

export type ShowDownloadBody = {
  name: string;
  season: number;
  episode?: number;
  imdb_id?: string;
  tmdb_id?: string;
  tvdb_id?: string;
  poster_url?: string;
  torrent_url: string;
  torrent_name: string;
  indexer: string;
  quality: string;
};

export async function downloadMovie(body: MovieDownloadBody): Promise<RequestAck> {
  return callApi<RequestAck>('/requests/movies/download', {
    method: 'POST',
    body: JSON.stringify(body),
  });
}

export async function downloadShow(body: ShowDownloadBody): Promise<RequestAck> {
  return callApi<RequestAck>('/requests/shows/download', {
    method: 'POST',
    body: JSON.stringify(body),
  });
}
