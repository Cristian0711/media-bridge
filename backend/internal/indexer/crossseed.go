package indexer

import "strings"

// crossSeedKey normalizes a release name so the same release listed on different
// indexers collapses to a single key. It lowercases and drops every non
// alphanumeric character, so cosmetic differences ("BluRay" vs "Blu-ray",
// dots vs spaces, "TrueHD7 1" vs "TrueHD 7 1") do not split a group, while
// different release groups or encodings still produce distinct keys.
func crossSeedKey(name string) string {
	var b strings.Builder
	b.Grow(len(name))
	for _, r := range strings.ToLower(name) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// crossSeedCounts maps each release key to the number of distinct indexers that
// carry it, computed over every item passed in (i.e. before any filtering).
// Duplicate listings on the same indexer are not double counted.
func crossSeedCounts[T any](items []T, name func(T) string, indexer func(T) string) map[string]int {
	byKey := make(map[string]map[string]struct{}, len(items))
	for _, it := range items {
		key := crossSeedKey(name(it))
		if key == "" {
			continue
		}
		set := byKey[key]
		if set == nil {
			set = make(map[string]struct{})
			byKey[key] = set
		}
		set[strings.ToLower(strings.TrimSpace(indexer(it)))] = struct{}{}
	}
	counts := make(map[string]int, len(byKey))
	for key, set := range byKey {
		counts[key] = len(set)
	}
	return counts
}

// annotateMovieCrossSeed sets CrossSeedCount on every movie based on all movies
// provided. Call this on the full result set before freeleech filtering.
func annotateMovieCrossSeed(movies []Movie) {
	counts := crossSeedCounts(movies, func(m Movie) string { return m.Name }, func(m Movie) string { return m.IndexerName })
	for i := range movies {
		movies[i].CrossSeedCount = crossSeedCountFor(counts, movies[i].Name)
	}
}

// annotateShowCrossSeed is the show-shaped counterpart of annotateMovieCrossSeed.
func annotateShowCrossSeed(shows []Show) {
	counts := crossSeedCounts(shows, func(s Show) string { return s.Name }, func(s Show) string { return s.IndexerName })
	for i := range shows {
		shows[i].CrossSeedCount = crossSeedCountFor(counts, shows[i].Name)
	}
}

func crossSeedCountFor(counts map[string]int, name string) int {
	if c, ok := counts[crossSeedKey(name)]; ok {
		return c
	}
	return 1
}
