/** Fields shared by every indexer torrent result (movie or show). */
export type IndexerTorrent = {
  id: number;
  name: string;
  imdb: string;
  freeleech: number;
  size: number;
  category: string;
  seeders: number;
  leechers: number;
  times_completed: number;
  download_link: string;
  quality: string;
  indexer_name: string;
};

export type IndexerMovie = IndexerTorrent;

export type IndexerShow = IndexerTorrent & {
  season: number;
  episode: number;
  complete_season: boolean;
};

export type MovieSearchResponse = {
  movies: IndexerMovie[];
  total: number;
  by_indexer: Record<string, number>;
  available_qualities: string[];
};

export type ShowSearchResponse = {
  shows: IndexerShow[];
  unparsed?: IndexerShow[];
  total: number;
  by_indexer: Record<string, number>;
  available_qualities: string[];
  available_seasons: number[];
};
