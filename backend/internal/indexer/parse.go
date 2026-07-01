package indexer

import (
	"fmt"
	"regexp"
)

var seasonEpisodePattern = regexp.MustCompile(`(?i)s(\d{1,2})(?:e(\d{1,2}))?`)

// parseSeasonEpisode extracts SxxEyy from a torrent name (used when mapping
// Prowlarr releases to Show rows in items.go). It returns:
//   - season, episode: the parsed numbers (season may be 0 for specials, S00);
//   - completeSeason: true when a season tag is present with no episode, i.e. a
//     full-season pack;
//   - ok: whether a season tag was matched at all. This distinguishes a real
//     season 0 (specials) from "could not parse", which both have season == 0 —
//     callers must use ok, not season > 0, to decide if parsing succeeded.
func parseSeasonEpisode(name string) (season, episode int, completeSeason, ok bool) {
	m := seasonEpisodePattern.FindStringSubmatch(name)
	if len(m) < 2 {
		return 0, 0, false, false
	}
	_, _ = fmt.Sscanf(m[1], "%d", &season)
	if len(m) > 2 && m[2] != "" {
		_, _ = fmt.Sscanf(m[2], "%d", &episode)
		return season, episode, false, true
	}
	return season, 0, true, true
}
