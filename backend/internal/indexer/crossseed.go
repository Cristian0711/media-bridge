package indexer

import (
	"strconv"
	"strings"
)

// crossSeedKey normalizes a release name so the same release listed on different
// indexers collapses to a single key. It lowercases and drops every non
// alphanumeric character. Used as the fallback grouping key when a release group
// cannot be parsed from the name.
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

var videoExtensions = map[string]bool{
	"mkv": true, "mp4": true, "avi": true, "ts": true,
	"m2ts": true, "wmv": true, "mov": true, "m4v": true,
}

// releaseGroup extracts the scene/p2p release group tag (the token after the
// final dash) from a release name, or "" when there isn't a clean one.
//
// The full name is unreliable for matching the same release across indexers
// (one writes "DD+5 1", another "DDP5.1"; one appends ".mkv"; one adds "Hybrid";
// dots vs spaces). The group tag is stable, so it anchors cross-seed grouping.
// Internal dashes in descriptors ("DTS-HD", "WEB-DL", "Blu-ray") are rejected by
// requiring the trailing token to be a single dotless word.
func releaseGroup(name string) string {
	s := strings.TrimSpace(name)
	// Strip a trailing video-file extension so "...-GROUP.mkv" yields "GROUP".
	if dot := strings.LastIndexByte(s, '.'); dot >= 0 && dot < len(s)-1 {
		if videoExtensions[strings.ToLower(s[dot+1:])] {
			s = s[:dot]
		}
	}
	// Drop trailing ")" / spaces from names like "(... x265 English - GRP)".
	s = strings.TrimRight(s, " )")

	dash := strings.LastIndexByte(s, '-')
	if dash < 0 || dash == len(s)-1 {
		return ""
	}
	tail := strings.TrimSpace(s[dash+1:])
	// A real group is a single dotless token. A tail containing "." means the
	// dash belonged to a descriptor (e.g. "DTS-HD.MA.5.1"), not a group.
	if tail == "" || strings.Contains(tail, ".") {
		return ""
	}
	// Some indexers append a stray suffix after a space ("WLM TL"); take the
	// first whitespace-delimited token as the group.
	if sp := strings.IndexAny(tail, " \t"); sp >= 0 {
		tail = tail[:sp]
	}
	return crossSeedKey(tail)
}

// crossSeedSizeGranularity buckets sizes so byte-level drift between trackers
// (differing overhead accounting) does not split an otherwise-identical release,
// while genuinely different encodes of the same group stay far apart.
const crossSeedSizeGranularity = 1 << 20 // 1 MiB

// crossSeedGroupKey identifies content that can be cross-seeded. Same release
// group AND same size (to MiB) is treated as the same content even when the
// descriptive parts of the name differ across indexers. When no group can be
// parsed it falls back to the normalized full name.
func crossSeedGroupKey(name string, size int64) string {
	if g := releaseGroup(name); g != "" {
		return "g|" + g + "|" + strconv.FormatInt(size/crossSeedSizeGranularity, 10)
	}
	if n := crossSeedKey(name); n != "" {
		return "n|" + n
	}
	return ""
}

// showCrossSeedKey extends the group+size key with the parsed season/episode so
// different seasons or episodes of the same release group and size are not
// treated as the same content.
func showCrossSeedKey(s Show) string {
	base := crossSeedGroupKey(s.Name, s.Size)
	if base == "" {
		return ""
	}
	return base + "|s" + strconv.Itoa(s.Season) + "e" + strconv.Itoa(s.Episode)
}

// countByKey returns, for each item, the number of distinct indexers sharing its
// cross-seed key. Items with an empty key (unidentifiable) count as 1. Duplicate
// listings on the same indexer are not double counted.
func countByKey(keys, indexers []string) []int {
	byKey := make(map[string]map[string]struct{})
	for i, k := range keys {
		if k == "" {
			continue
		}
		set := byKey[k]
		if set == nil {
			set = make(map[string]struct{})
			byKey[k] = set
		}
		set[strings.ToLower(strings.TrimSpace(indexers[i]))] = struct{}{}
	}
	out := make([]int, len(keys))
	for i, k := range keys {
		if k == "" || byKey[k] == nil {
			out[i] = 1
			continue
		}
		out[i] = len(byKey[k])
	}
	return out
}

// annotateMovieCrossSeed sets CrossSeedCount on every movie based on all movies
// provided. Call this on the full result set before freeleech filtering.
func annotateMovieCrossSeed(movies []Movie) {
	keys := make([]string, len(movies))
	indexers := make([]string, len(movies))
	for i, m := range movies {
		keys[i] = crossSeedGroupKey(m.Name, m.Size)
		indexers[i] = m.IndexerName
	}
	counts := countByKey(keys, indexers)
	for i := range movies {
		movies[i].CrossSeedCount = counts[i]
	}
}

// annotateShowCrossSeed is the show-shaped counterpart of annotateMovieCrossSeed.
func annotateShowCrossSeed(shows []Show) {
	keys := make([]string, len(shows))
	indexers := make([]string, len(shows))
	for i, s := range shows {
		keys[i] = showCrossSeedKey(s)
		indexers[i] = s.IndexerName
	}
	counts := countByKey(keys, indexers)
	for i := range shows {
		shows[i].CrossSeedCount = counts[i]
	}
}
