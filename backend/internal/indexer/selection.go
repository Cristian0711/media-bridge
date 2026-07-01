package indexer

import (
	"fmt"
	"sort"
	"strings"
)

// isFileList reports whether the release came from the FileList indexer, which
// is exempt from the freeleech-only filter (non-freeleech results are shown).
func isFileList(indexerName string) bool {
	return strings.Contains(strings.ToLower(indexerName), "filelist")
}

// pickBestFreeleechMovie returns the highest-seeded freeleech torrent for the given quality.
func pickBestFreeleechMovie(movies []Movie, quality string) (Movie, error) {
	want := canonicalQuality(quality)
	if want == "" || want == "Unknown" {
		return Movie{}, fmt.Errorf("invalid quality")
	}
	var best *Movie
	for i := range movies {
		m := &movies[i]
		if m.Seeders <= 0 || m.Freeleech != 1 {
			continue
		}
		if canonicalQuality(m.Quality) != want {
			continue
		}
		if best == nil || m.Seeders > best.Seeders {
			best = m
		}
	}
	if best == nil {
		return Movie{}, fmt.Errorf("no freeleech torrent available for this quality")
	}
	return *best, nil
}

// pickBestFreeleechShow returns the highest-seeded freeleech torrent for the given quality.
func pickBestFreeleechShow(shows []Show, quality string) (Show, error) {
	want := canonicalQuality(quality)
	if want == "" || want == "Unknown" {
		return Show{}, fmt.Errorf("invalid quality")
	}
	var best *Show
	for i := range shows {
		s := &shows[i]
		if s.Seeders <= 0 || s.Freeleech != 1 {
			continue
		}
		if canonicalQuality(s.Quality) != want {
			continue
		}
		if best == nil || s.Seeders > best.Seeders {
			best = s
		}
	}
	if best == nil {
		return Show{}, fmt.Errorf("no freeleech torrent available for this quality")
	}
	return *best, nil
}

func filterAndSortMovies(movies []Movie, quality string) []Movie {
	filtered := make([]Movie, 0, len(movies))
	for _, m := range movies {
		if m.Seeders == 0 {
			continue
		}
		if m.Freeleech != 1 && !isFileList(m.IndexerName) {
			continue
		}
		if quality != "" && canonicalQuality(m.Quality) != canonicalQuality(quality) {
			continue
		}
		filtered = append(filtered, m)
	}
	sort.SliceStable(filtered, func(i, j int) bool {
		qi := qualityPriority(filtered[i].Quality)
		qj := qualityPriority(filtered[j].Quality)
		if qi == qj {
			return filtered[i].Seeders > filtered[j].Seeders
		}
		return qi > qj
	})
	return filtered
}

func filterAndSortShows(shows []Show, season, episode int, quality string) (parsed []Show, unparsed []Show) {
	for _, s := range shows {
		if s.Seeders == 0 {
			continue
		}
		if s.Freeleech != 1 && !isFileList(s.IndexerName) {
			continue
		}
		if quality != "" && canonicalQuality(s.Quality) != canonicalQuality(quality) {
			continue
		}
		if s.Parsed {
			// s.Season may legitimately be 0 here (S00 specials); bucket on
			// whether a season tag was matched, not on season > 0, so specials
			// are filterable instead of being dumped into "unparsed".
			if season > 0 && s.Season != season {
				continue
			}
			if episode > 0 && s.Episode != episode && !s.Complete {
				continue
			}
			parsed = append(parsed, s)
		} else {
			unparsed = append(unparsed, s)
		}
	}
	sortShows(parsed)
	sortShows(unparsed)
	return parsed, unparsed
}

func sortShows(shows []Show) {
	sort.SliceStable(shows, func(i, j int) bool {
		qi := qualityPriority(shows[i].Quality)
		qj := qualityPriority(shows[j].Quality)
		if qi == qj {
			return shows[i].Seeders > shows[j].Seeders
		}
		return qi > qj
	})
}
