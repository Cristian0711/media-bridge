export type MediaType = 'movies' | 'shows';

export type MediaItem = {
  title: string;
  year: number;
  ids: {
    imdb?: string;
    tmdb?: number;
    tvdb?: number;
    trakt?: number;
    slug?: string;
  };
  images: {
    poster: string[];
  };
};
