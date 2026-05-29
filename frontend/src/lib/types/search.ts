export type SearchMoviePayload = {
  title: string;
  year: number;
  ids: {
    trakt?: number;
    slug?: string;
    imdb?: string;
    tmdb?: number;
  };
  images: {
    poster: string[];
  };
};

export type SearchShowPayload = {
  title: string;
  year: number;
  ids: {
    trakt?: number;
    slug?: string;
    imdb?: string;
    tvdb?: number;
    tmdb?: number;
  };
  images: {
    poster: string[];
  };
};

export type SearchResult = {
  type: 'movie' | 'show';
  score: number;
  movie?: SearchMoviePayload;
  show?: SearchShowPayload;
};
