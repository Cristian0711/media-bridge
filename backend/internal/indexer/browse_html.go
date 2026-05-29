package indexer

import (
	"bytes"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

func parseBrowseMovies(html string) []Movie {
	result := parseBrowseHTML(html, false)
	return result.movies
}

func parseBrowseShows(html string) []Show {
	result := parseBrowseHTML(html, true)
	return result.shows
}

// ParseBrowseMovies / ParseBrowseShows are the exported entry points used by the
// FileList provider to scrape its browse HTML.
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
