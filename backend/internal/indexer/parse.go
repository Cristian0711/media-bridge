package indexer

import (
	"fmt"
	"regexp"
)

var seasonEpisodePattern = regexp.MustCompile(`(?i)s(\d{1,2})(?:e(\d{1,2}))?`)

func parseMovieMetadata(movie *Movie) {
	movie.Quality = parseQuality(movie.Name)
}

func parseShowMetadata(show *Show) bool {
	show.Quality = parseQuality(show.Name)
	matches := seasonEpisodePattern.FindStringSubmatch(show.Name)
	if len(matches) <= 1 {
		show.Season = 0
		show.Episode = 0
		show.Complete = false
		return false
	}
	_, _ = fmt.Sscanf(matches[1], "%d", &show.Season)
	if len(matches) > 2 && matches[2] != "" {
		_, _ = fmt.Sscanf(matches[2], "%d", &show.Episode)
		show.Complete = false
	} else {
		show.Episode = 0
		show.Complete = true
	}
	return true
}

func parseSeasonEpisode(name string) (int, int, bool) {
	m := seasonEpisodePattern.FindStringSubmatch(name)
	if len(m) < 2 {
		return 0, 0, false
	}
	var s, e int
	_, _ = fmt.Sscanf(m[1], "%d", &s)
	if len(m) > 2 && m[2] != "" {
		_, _ = fmt.Sscanf(m[2], "%d", &e)
		return s, e, false
	}
	return s, 0, true
}
