/** Raw media row from GET /media/list and /media/list/my */
export type ApiMedia = {
  id: number;
  type: 'movie' | 'show_full' | 'show_season' | 'show_episode';
  name: string;
  path: string;
  indexer: string;
  quality: string;
  size_bytes?: number;
  user_id: number;
  username: string;
  created_at: string;
  updated_at: string;
  movie?: {
    imdb_id?: string;
    tmdb_id?: string;
    year?: number;
    poster_url?: string;
  };
  show_entry?: {
    season?: number;
    episode?: number;
    show?: {
      imdb_id?: string;
      tvdb_id?: string;
      poster_url?: string;
    };
  };
};

export type PaginatedMediaResponse = {
  media: ApiMedia[];
  page: number;
  page_size: number;
  total_count: number;
  total_size_bytes: number;
  total_pages: number;
};

export type MediaLibraryItem = {
  id: number;
  title: string;
  type: ApiMedia['type'];
  poster_url?: string;
  imdb_id?: string;
  tmdb_id?: string;
  tvdb_id?: string;
  season?: number;
  episode?: number;
  username: string;
  quality: string;
  indexer: string;
  size_bytes?: number;
  created_at: string;
};
