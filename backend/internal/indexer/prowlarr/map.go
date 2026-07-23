package prowlarr

import (
	"fmt"
	"strconv"
	"strings"

	idx "github.com/Cristian0711/media-bridge/backend/internal/indexer"
)

const (
	categoryMovies = 2000
	categoryTV     = 5000
)

// ToIndexerItems maps Prowlarr releases into the shared IndexerItem shape used by
// the indexer service (and ultimately Movie/Show API responses).
func ToIndexerItems(releases []Release, imdbFallback string, tvOnly bool) []idx.IndexerItem {
	out := make([]idx.IndexerItem, 0, len(releases))
	for _, r := range releases {
		// Prowlarr already scoped the query by type (movie vs tvsearch), so the
		// category check is only a guard against an indexer returning the wrong
		// type. Some indexers (e.g. FileList via Prowlarr) return an empty
		// category list; those can't be classified, so trust the search type and
		// keep them rather than silently dropping the whole indexer's results.
		if len(r.Categories) > 0 {
			if tvOnly {
				if !isTVRelease(r) {
					continue
				}
			} else if !isMovieRelease(r) {
				continue
			}
		}
		out = append(out, releaseToItem(r, imdbFallback))
	}
	return out
}

func releaseToItem(r Release, imdbFallback string) idx.IndexerItem {
	imdb := formatIMDBID(r.IMDBID)
	if imdb == "" {
		imdb = normalizeIMDB(imdbFallback)
	}
	return idx.IndexerItem{
		ID:           releaseID(r.GUID),
		Name:         r.Title,
		ImdbID:       imdb,
		Size:         r.Size,
		Seeders:      r.Seeders,
		Leechers:     r.Leechers,
		Downloads:    r.Grabs,
		DownloadLink: r.DownloadURL,
		Freeleech:    hasFreeleech(r.IndexerFlags),
		Category:     primaryCategory(r.Categories),
		UploadDate:   r.PublishDate,
		IndexerName:  strings.TrimSpace(r.Indexer),
	}
}

func isMovieRelease(r Release) bool {
	for _, c := range r.Categories {
		if categoryTreeIsMovie(c) {
			return true
		}
	}
	return false
}

func isTVRelease(r Release) bool {
	for _, c := range r.Categories {
		if categoryTreeIsTV(c) {
			return true
		}
	}
	return false
}

func categoryTreeIsMovie(c Category) bool {
	if c.ID == categoryMovies {
		return true
	}
	name := strings.ToLower(c.Name)
	if strings.Contains(name, "movie") && !strings.Contains(name, "audio") {
		return true
	}
	for _, sub := range c.SubCategories {
		if categoryTreeIsMovie(sub) {
			return true
		}
	}
	return false
}

func categoryTreeIsTV(c Category) bool {
	if c.ID == categoryTV {
		return true
	}
	name := strings.ToLower(c.Name)
	if strings.HasPrefix(name, "tv") || strings.Contains(name, "television") {
		return true
	}
	for _, sub := range c.SubCategories {
		if categoryTreeIsTV(sub) {
			return true
		}
	}
	return false
}

func primaryCategory(cats []Category) string {
	if len(cats) == 0 {
		return ""
	}
	names := make([]string, 0, len(cats))
	for _, c := range cats {
		if c.Name != "" {
			names = append(names, c.Name)
		} else if c.ID > 0 {
			names = append(names, strconv.Itoa(c.ID))
		}
	}
	return strings.Join(names, " / ")
}

func hasFreeleech(flags []string) bool {
	for _, f := range flags {
		if strings.EqualFold(strings.TrimSpace(f), "freeleech") {
			return true
		}
	}
	return false
}

// releaseID extracts a stable numeric id for the API (Movie.id / Show.id).
func releaseID(guid string) string {
	guid = strings.TrimSpace(guid)
	if guid == "" {
		return ""
	}
	// e.g. FileList-957709
	if i := strings.LastIndex(guid, "-"); i >= 0 {
		tail := guid[i+1:]
		if _, err := strconv.ParseInt(tail, 10, 64); err == nil {
			return tail
		}
	}
	return guid
}

func formatIMDBID(n int) string {
	if n <= 0 {
		return ""
	}
	return fmt.Sprintf("tt%07d", n)
}

// NormalizeIMDB normalizes request imdb_id values to tt-prefixed form.
func NormalizeIMDB(id string) string {
	return normalizeIMDB(id)
}

func normalizeIMDB(id string) string {
	id = strings.TrimSpace(id)
	if id == "" {
		return ""
	}
	lower := strings.ToLower(id)
	lower = strings.TrimPrefix(lower, "imdb:")
	lower = strings.TrimPrefix(lower, "tt")
	lower = strings.TrimSpace(lower)
	if lower == "" {
		return ""
	}
	for _, c := range lower {
		if c < '0' || c > '9' {
			return id
		}
	}
	n, err := strconv.Atoi(lower)
	if err != nil || n <= 0 {
		return id
	}
	return fmt.Sprintf("tt%07d", n)
}

// MovieSearchQuery builds a Prowlarr movie search query string.
func MovieSearchQuery(imdbID string) string {
	return "{imdbid:" + normalizeIMDB(imdbID) + "}"
}

// TVSearchQuery builds a Prowlarr tvsearch query string.
func TVSearchQuery(imdbID string, season, episode int) string {
	q := "{imdbid:" + normalizeIMDB(imdbID) + "}"
	if season > 0 {
		q += "{season:" + strconv.Itoa(season) + "}"
	}
	if episode > 0 {
		q += "{episode:" + strconv.Itoa(episode) + "}"
	}
	return q
}

// MatchIndexerFilter returns true when names is empty or the release indexer name
// matches one of the requested names (case-insensitive substring).
func MatchIndexerFilter(indexerName string, names []string) bool {
	if len(names) == 0 {
		return true
	}
	lower := strings.ToLower(strings.TrimSpace(indexerName))
	for _, n := range names {
		n = strings.TrimSpace(n)
		if n == "" {
			continue
		}
		if strings.Contains(lower, strings.ToLower(n)) {
			return true
		}
	}
	return false
}

// ResolveIndexerIDs maps requested name fragments to Prowlarr indexer IDs.
func ResolveIndexerIDs(indexers []Indexer, names []string) []int {
	if len(names) == 0 {
		return nil
	}
	ids := make([]int, 0, len(names))
	seen := make(map[int]struct{})
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		// Allow numeric indexer id in the filter list.
		if n, err := strconv.Atoi(name); err == nil {
			if _, ok := seen[n]; !ok {
				seen[n] = struct{}{}
				ids = append(ids, n)
			}
			continue
		}
		lower := strings.ToLower(name)
		for _, idx := range indexers {
			if !idx.IsEnabled() {
				continue
			}
			in := strings.ToLower(idx.Name)
			if strings.Contains(in, lower) || strings.Contains(lower, in) {
				if _, ok := seen[idx.ID]; !ok {
					seen[idx.ID] = struct{}{}
					ids = append(ids, idx.ID)
				}
			}
		}
	}
	return ids
}
