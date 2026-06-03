package indexer

import (
	"fmt"
	"regexp"
)

var seasonEpisodePattern = regexp.MustCompile(`(?i)s(\d{1,2})(?:e(\d{1,2}))?`)

// parseSeasonEpisode extracts SxxEyy from a torrent name (used when mapping
// Prowlarr releases to Show rows in items.go).
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
