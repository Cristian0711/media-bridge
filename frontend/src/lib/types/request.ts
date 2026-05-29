export type RequestRow = {
  id: number;
  type: string;
  status: string;
  name: string;
  user_id: number;
  username: string;
  request_id: string;
  media_id?: number;
  imdb_id?: string;
  tmdb_id?: string;
  tvdb_id?: string;
  season?: number;
  episode?: number;
  poster_url?: string;
  torrent_url?: string;
  torrent_name?: string;
  indexer: string;
  quality: string;
  created_at: string;
  updated_at: string;
};

export type PaginatedRequestsResponse = {
  requests: RequestRow[];
  page: number;
  page_size: number;
  total_count: number;
  total_pages: number;
};
