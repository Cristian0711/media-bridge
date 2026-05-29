package indexer

import (
	"bytes"
	"context"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

var seasonEpisodePattern = regexp.MustCompile(`(?i)s(\d{1,2})(?:e(\d{1,2}))?`)

type qualityRank struct {
	Label    string
	Priority int
}

var qualities = []qualityRank{
	{Label: "1080p", Priority: 9},
	{Label: "1080p BluRay", Priority: 8},
	{Label: "1080p Remux", Priority: 7},
	{Label: "4K Remux", Priority: 6},
	{Label: "4K BluRay", Priority: 5},
	{Label: "4K", Priority: 4},
	{Label: "720p Remux", Priority: 3},
	{Label: "720p", Priority: 2},
	{Label: "Unknown", Priority: 1},
}

// canonicalQuality maps a torrent title or an API quality label to a standard quality name.
func canonicalQuality(quality string) string {
	q := strings.TrimSpace(quality)
	if q == "" {
		return ""
	}
	for _, item := range qualities {
		if strings.EqualFold(item.Label, q) {
			return item.Label
		}
	}
	return parseQuality(q)
}

func parseQuality(name string) string {
	if name == "" {
		return "Unknown"
	}
	n := strings.TrimSpace(strings.ReplaceAll(name, "_", " "))
	switch {
	case regexp.MustCompile(`(?i)\b(2160p|3840x2160|\[4k\]|\buhd\b|4k[-_. ](?:uhd|hevc|bd|h265)|(?:uhd|hevc|bd|h265)[-_. ]4k)\b`).MatchString(n):
		if regexp.MustCompile(`(?i)\b(remux)\b`).MatchString(n) {
			return "4K Remux"
		}
		if regexp.MustCompile(`(?i)\b(bluray|blu-ray)\b`).MatchString(n) {
			return "4K BluRay"
		}
		return "4K"
	case regexp.MustCompile(`(?i)\b(1080p|1920x1080|fhd|1080i|4kto1080p)\b`).MatchString(n):
		if regexp.MustCompile(`(?i)\b(remux)\b`).MatchString(n) {
			return "1080p Remux"
		}
		if regexp.MustCompile(`(?i)\b(bluray|blu-ray)\b`).MatchString(n) {
			return "1080p BluRay"
		}
		return "1080p"
	case regexp.MustCompile(`(?i)\b(720p|1280x720|960p)\b`).MatchString(n):
		if regexp.MustCompile(`(?i)\b(remux)\b`).MatchString(n) {
			return "720p Remux"
		}
		return "720p"
	default:
		return "Unknown"
	}
}

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
	fmt.Sscanf(matches[1], "%d", &show.Season)
	if len(matches) > 2 && matches[2] != "" {
		fmt.Sscanf(matches[2], "%d", &show.Episode)
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
	fmt.Sscanf(m[1], "%d", &s)
	if len(m) > 2 && m[2] != "" {
		fmt.Sscanf(m[2], "%d", &e)
		return s, e, false
	}
	return s, 0, true
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
		if quality != "" && canonicalQuality(s.Quality) != canonicalQuality(quality) {
			continue
		}
		if s.Season > 0 {
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

func qualityPriority(quality string) int {
	q := canonicalQuality(quality)
	for _, item := range qualities {
		if item.Label == q {
			return item.Priority
		}
	}
	return 0
}

func buildShowQuery(name string, season, episode int) string {
	if season > 0 && episode > 0 {
		return fmt.Sprintf("%s S%02dE%02d", name, season, episode)
	}
	if season > 0 {
		return fmt.Sprintf("%s S%02d", name, season)
	}
	return name
}

func BuildShowQuery(name string, season, episode int) string {
	return buildShowQuery(name, season, episode)
}

func ParseTorrentQuality(name string) string {
	return parseQuality(name)
}

func processMovieItems(_ context.Context, items []IndexerItem) []Movie {
	movies := make([]Movie, 0, len(items))
	for _, item := range items {
		movie := Movie{
			ID:           parseID(item.ID),
			Name:         item.Name,
			Imdb:         item.ImdbID,
			Freeleech:    boolToInt(item.Freeleech),
			Size:         item.Size,
			Category:     item.Category,
			Seeders:      item.Seeders,
			Leechers:     item.Leechers,
			Downloads:    item.Downloads,
			DownloadLink: item.DownloadLink,
			Quality:      parseQuality(item.Name),
			IndexerName:  item.IndexerName,
		}
		movies = append(movies, movie)
	}
	return movies
}

func processShowItems(_ context.Context, items []IndexerItem) []Show {
	shows := make([]Show, 0, len(items))
	for _, item := range items {
		season, episode, complete := parseSeasonEpisode(item.Name)
		show := Show{
			ID:           parseID(item.ID),
			Name:         item.Name,
			Imdb:         item.ImdbID,
			Freeleech:    boolToInt(item.Freeleech),
			Size:         item.Size,
			Category:     item.Category,
			Seeders:      item.Seeders,
			Leechers:     item.Leechers,
			Downloads:    item.Downloads,
			DownloadLink: item.DownloadLink,
			Quality:      parseQuality(item.Name),
			IndexerName:  item.IndexerName,
			Season:       season,
			Episode:      episode,
			Complete:     complete,
		}
		shows = append(shows, show)
	}
	return shows
}

func parseBrowseMovies(html string) []Movie {
	result := parseBrowseHTML(html, false)
	return result.movies
}

func parseBrowseShows(html string) []Show {
	result := parseBrowseHTML(html, true)
	return result.shows
}

func ParseBrowseMovies(html string) []Movie {
	return parseBrowseMovies(html)
}

func ParseBrowseShows(html string) []Show {
	return parseBrowseShows(html)
}

type browseParseResult struct {
	movies []Movie
	shows  []Show
}

func parseBrowseHTML(html string, shows bool) browseParseResult {
	if html == "" {
		return browseParseResult{}
	}
	doc, err := goquery.NewDocumentFromReader(bytes.NewBufferString(html))
	if err != nil {
		return browseParseResult{}
	}

	sizeRe := regexp.MustCompile(`(?i)([0-9]+(?:[\.,][0-9]+)?)\s*(TB|GB|MB|KB)`)
	parseSize := func(s string) int64 {
		m := sizeRe.FindStringSubmatch(strings.TrimSpace(s))
		if len(m) < 3 {
			return 0
		}
		numStr := strings.ReplaceAll(strings.ReplaceAll(m[1], ",", "."), " ", "")
		f, err := strconv.ParseFloat(numStr, 64)
		if err != nil {
			return 0
		}
		switch strings.ToUpper(m[2]) {
		case "TB":
			return int64(f * 1024 * 1024 * 1024 * 1024)
		case "GB":
			return int64(f * 1024 * 1024 * 1024)
		case "MB":
			return int64(f * 1024 * 1024)
		case "KB":
			return int64(f * 1024)
		default:
			return 0
		}
	}
	parseInt := func(s string) int {
		s = strings.TrimSpace(s)
		buf := make([]rune, 0, len(s))
		for _, r := range s {
			if r >= '0' && r <= '9' {
				buf = append(buf, r)
			}
		}
		v, _ := strconv.Atoi(string(buf))
		return v
	}

	result := browseParseResult{
		movies: make([]Movie, 0, 16),
		shows:  make([]Show, 0, 16),
	}

	doc.Find("div.torrentrow").Each(func(_ int, row *goquery.Selection) {
		cells := row.Find("div.torrenttable")
		if cells.Length() < 2 {
			return
		}
		category := strings.TrimSpace(cells.Eq(0).Find("img[alt]").AttrOr("alt", ""))
		titleAnchor := cells.Eq(1).Find("a[href^='details.php?id='][title]").First()
		href, _ := titleAnchor.Attr("href")
		title := strings.TrimSpace(titleAnchor.AttrOr("title", ""))
		if title == "" || href == "" {
			return
		}
		parsedURL, err := url.Parse(href)
		if err != nil {
			return
		}
		id := parsedURL.Query().Get("id")
		if id == "" {
			parts := strings.Split(href, "id=")
			if len(parts) == 2 {
				id = parts[1]
			}
		}
		if id == "" {
			return
		}

		freeleech := 0
		if cells.Eq(1).Find("img[src*='freeleech']").Length() > 0 {
			freeleech = 1
		}
		var sizeBytes int64
		var downloads, seeders, leechers int
		if cells.Length() >= 10 {
			sizeBytes = parseSize(cells.Eq(6).Text())
			downloads = parseInt(cells.Eq(7).Text())
			seeders = parseInt(cells.Eq(8).Text())
			leechers = parseInt(cells.Eq(9).Text())
		}

		if shows {
			item := Show{
				Name:         title,
				Freeleech:    freeleech,
				Size:         sizeBytes,
				Category:     category,
				Seeders:      seeders,
				Leechers:     leechers,
				Downloads:    downloads,
				DownloadLink: fmt.Sprintf("https://filelist.io/download.php?id=%s", id),
			}
			parseShowMetadata(&item)
			result.shows = append(result.shows, item)
			return
		}

		item := Movie{
			Name:         title,
			Freeleech:    freeleech,
			Size:         sizeBytes,
			Category:     category,
			Seeders:      seeders,
			Leechers:     leechers,
			Downloads:    downloads,
			DownloadLink: fmt.Sprintf("https://filelist.io/download.php?id=%s", id),
		}
		parseMovieMetadata(&item)
		result.movies = append(result.movies, item)
	})

	return result
}
