package search

// MovieSearchResult is a single search hit for movies.
type MovieSearchResult struct {
	Type  string  `json:"type"`
	Score float64 `json:"score"`
	Movie Movie   `json:"movie"`
}

// Movie holds metadata returned for a movie search result.
type Movie struct {
	Title  string      `json:"title"`
	Year   int         `json:"year"`
	IDs    MovieIDs    `json:"ids"`
	Images MovieImages `json:"images"`
}

// MovieIDs contains external identifiers for a movie.
type MovieIDs struct {
	Trakt int    `json:"trakt,omitempty"`
	Slug  string `json:"slug,omitempty"`
	IMDB  string `json:"imdb,omitempty"`
	TMDB  int    `json:"tmdb"`
}

// MovieImages contains image URLs for a movie.
type MovieImages struct {
	Poster []string `json:"poster"`
}

// ShowSearchResult is a single search hit for shows.
type ShowSearchResult struct {
	Type  string  `json:"type"`
	Score float64 `json:"score"`
	Show  Show    `json:"show"`
}

// Show holds metadata returned for a show search result.
type Show struct {
	Title  string     `json:"title"`
	Year   int        `json:"year"`
	IDs    ShowIDs    `json:"ids"`
	Images ShowImages `json:"images"`
}

// ShowIDs contains external identifiers for a show.
type ShowIDs struct {
	Trakt int    `json:"trakt,omitempty"`
	Slug  string `json:"slug,omitempty"`
	IMDB  string `json:"imdb,omitempty"`
	TVDB  int    `json:"tvdb,omitempty"`
	TMDB  int    `json:"tmdb"`
}

// ShowImages contains image URLs for a show.
type ShowImages struct {
	Poster []string `json:"poster"`
}

// Result is a combined movie or show hit with score, type, and poster images.
// Available reports whether the title already exists in the server library;
// it is filled in at the response boundary, not stored in the browse cache.
type Result struct {
	Type      string  `json:"type"`
	Score     float64 `json:"score"`
	Movie     *Movie  `json:"movie,omitempty"`
	Show      *Show   `json:"show,omitempty"`
	Available bool    `json:"available"`
}
