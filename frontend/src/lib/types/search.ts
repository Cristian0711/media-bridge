type SearchIds = {
  trakt?: number;
  slug?: string;
  imdb?: string;
  tmdb?: number;
};

type SearchPayload = {
  title: string;
  year: number;
  images: {
    poster: string[];
  };
};

export type SearchMoviePayload = SearchPayload & {
  ids: SearchIds;
};

export type SearchShowPayload = SearchPayload & {
  ids: SearchIds & {
    tvdb?: number;
  };
};

export type SearchResult = {
  type: 'movie' | 'show';
  score: number;
  movie?: SearchMoviePayload;
  show?: SearchShowPayload;
  /** Whether this title already exists in the server library. */
  available?: boolean;
};
