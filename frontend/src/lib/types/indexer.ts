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
  /** Distinct indexers carrying this release (pre-filter). >1 = cross-seedable. */
  cross_seed_count: number;
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

/** One configurable indexer in the admin panel. */
export type IndexerSetting = {
  id: string;
  name: string;
  enabled: boolean;
  /** When true, only freeleech torrents from this indexer are shown. */
  freeleech_only: boolean;
};

export type IndexerSettingsResponse = {
  indexers: IndexerSetting[];
};
