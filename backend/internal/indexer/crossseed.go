package indexer

import (
	"regexp"
	"strings"
)

// Cross-seed detection, ported from the cross-seed project's name/metadata
// matching (src/decide.ts assessCandidate + helpers).
//
// Our search is scoped to a single IMDb id, so every result is the same title —
// exactly the situation cross-seed is in when assessing torznab candidates. That
// means we don't compare titles; we only decide which results are the same
// *release* (and therefore cross-seedable) using the same signals cross-seed
// uses: release group, resolution, source, release version, and a fuzzy size
// tolerance. Each check is permissive: it passes unless it can prove a mismatch.
//
// The relation is applied pairwise (it is not a clean equivalence relation, e.g.
// group prefix-matching), so CrossSeedCount for a row is "how many distinct
// indexers carry a release matching this one" (including its own).

// fuzzySizeThreshold mirrors cross-seed's default (±2%).
const fuzzySizeThreshold = 0.02

var (
	// Strict resolution, matching cross-seed's RES_STRICT_REGEX.
	resStrictRe = regexp.MustCompile(`(?i)(2160|1080|720)[pi]`)
	// REPACK/PROPER/vN, matching cross-seed's REPACK_PROPER_REGEX.
	repackProperRe = regexp.MustCompile(`(?i)\b(?:REPACK|PROPER|\dv\d)\d?\b`)
	videoExtRe     = regexp.MustCompile(`(?i)\.(?:mkv|mp4|avi|ts|m2ts|m4v|wmv|mov|flv|divx|xvid)$`)
	nonAlnumRe     = regexp.MustCompile(`[^a-z0-9]+`)
)

// sourceMatchers ports cross-seed's SOURCE_REGEXES (streaming service tags). The
// HBO negative lookahead is dropped (RE2 has none); the result label is HBO
// either way, so classification is unaffected.
var sourceMatchers = []struct {
	name string
	re   *regexp.Regexp
}{
	{"AMZN", regexp.MustCompile(`(?i)\b(?:amzn|amazon(?:hd)?)\b[ ._-]web[ ._-]?(?:dl|rip)?\b`)},
	{"DSNP", regexp.MustCompile(`(?i)\b(?:dsnp|dsny|disney)\b`)},
	{"NF", regexp.MustCompile(`(?i)\b(?:nf|netflix(?:u?hd)?)\b`)},
	{"HULU", regexp.MustCompile(`(?i)\bhulu\b`)},
	{"ATVP", regexp.MustCompile(`(?i)\b(?:atvp|aptv)\b`)},
	{"HBO", regexp.MustCompile(`(?i)\bhbo\b|\b(?:hmax|hbom|hbo[ ._-]max)\b`)},
	{"PCOK", regexp.MustCompile(`(?i)\bpcok\b`)},
	{"PMTP", regexp.MustCompile(`(?i)\b(?:pmtp|paramount plus)\b`)},
}

func parseSource(name string) string {
	for _, s := range sourceMatchers {
		if s.re.MatchString(name) {
			return s.name
		}
	}
	return ""
}

func parseResolution(name string) string {
	if m := resStrictRe.FindStringSubmatch(name); m != nil {
		return m[1]
	}
	return ""
}

// badGroupTokens are descriptor tokens that must not be mistaken for a release
// group when they trail the final dash of an untagged release. This captures the
// intent of cross-seed's BAD_GROUP_PARSE_REGEX in a RE2-friendly way.
var badGroupTokens = map[string]bool{
	"x264": true, "x265": true, "h264": true, "h265": true, "hevc": true,
	"avc": true, "xvid": true, "divx": true, "remux": true, "bluray": true,
	"web": true, "webdl": true, "webrip": true, "bdrip": true, "brrip": true,
	"hdrip": true, "dts": true, "ddp": true, "dd": true, "aac": true,
	"eac3": true, "ac3": true, "atmos": true, "truehd": true, "flac": true,
	"hdr": true, "hdr10": true, "dv": true, "dovi": true, "uhd": true,
	"sdr": true, "ma": true, "hd": true, "dl": true, "proper": true, "repack": true,
}

func alnum(s string) string {
	return nonAlnumRe.ReplaceAllString(strings.ToLower(s), "")
}

// releaseGroup extracts the trailing -GROUP tag, or "" when there isn't a clean
// one. Go's RE2 cannot express cross-seed's lookbehind RELEASE_GROUP_REGEX, so
// this is a pragmatic equivalent with the same rejection intent: strip a video
// extension, take the token after the final dash, and reject it if it is part of
// a descriptor ("DTS-HD.MA.5.1") or a known non-group token ("x264").
func releaseGroup(name string) string {
	s := videoExtRe.ReplaceAllString(strings.TrimSpace(name), "")
	s = strings.TrimRight(s, " )")
	dash := strings.LastIndexByte(s, '-')
	if dash < 0 || dash == len(s)-1 {
		return ""
	}
	tail := strings.TrimSpace(s[dash+1:])
	if tail == "" || strings.Contains(tail, ".") {
		return ""
	}
	if sp := strings.IndexAny(tail, " \t"); sp >= 0 {
		tail = tail[:sp]
	}
	g := alnum(tail)
	if g == "" || badGroupTokens[g] {
		return ""
	}
	return g
}

func resolutionMatches(a, b string) bool {
	ra, rb := parseResolution(a), parseResolution(b)
	return ra == "" || rb == "" || ra == rb
}

func sourceMatches(a, b string) bool {
	sa, sb := parseSource(a), parseSource(b)
	return sa == "" || sb == "" || sa == sb
}

func groupMatches(a, b string) bool {
	ga, gb := releaseGroup(a), releaseGroup(b)
	if ga == "" || gb == "" {
		return true // permissive when a group can't be parsed
	}
	return strings.HasPrefix(ga, gb) || strings.HasPrefix(gb, ga)
}

func versionMatches(a, b string) bool {
	return repackProperRe.MatchString(a) == repackProperRe.MatchString(b)
}

// fuzzySizeMatches reports whether two sizes are within fuzzySizeThreshold of the
// larger (symmetric variant of cross-seed's searchee-relative bounds).
func fuzzySizeMatches(a, b int64) bool {
	if a <= 0 || b <= 0 {
		return a == b
	}
	diff := a - b
	if diff < 0 {
		diff = -diff
	}
	base := a
	if b > base {
		base = b
	}
	return float64(diff) <= fuzzySizeThreshold*float64(base)
}

// sameRelease is the pairwise "these can be cross-seeded" predicate.
func sameRelease(nameA string, sizeA int64, nameB string, sizeB int64) bool {
	return resolutionMatches(nameA, nameB) &&
		sourceMatches(nameA, nameB) &&
		groupMatches(nameA, nameB) &&
		versionMatches(nameA, nameB) &&
		fuzzySizeMatches(sizeA, sizeB)
}

func normIndexer(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

// annotateMovieCrossSeed sets CrossSeedCount on every movie to the number of
// distinct indexers carrying a matching release. Call on the full result set
// before freeleech filtering so the count reflects every indexer.
func annotateMovieCrossSeed(movies []Movie) {
	for i := range movies {
		seen := map[string]struct{}{normIndexer(movies[i].IndexerName): {}}
		for j := range movies {
			if i == j {
				continue
			}
			if sameRelease(movies[i].Name, movies[i].Size, movies[j].Name, movies[j].Size) {
				seen[normIndexer(movies[j].IndexerName)] = struct{}{}
			}
		}
		movies[i].CrossSeedCount = len(seen)
	}
}

// annotateShowCrossSeed is the show-shaped counterpart. Shows additionally must
// share the parsed season and episode to be considered the same release.
func annotateShowCrossSeed(shows []Show) {
	for i := range shows {
		seen := map[string]struct{}{normIndexer(shows[i].IndexerName): {}}
		for j := range shows {
			if i == j {
				continue
			}
			if shows[i].Season == shows[j].Season &&
				shows[i].Episode == shows[j].Episode &&
				sameRelease(shows[i].Name, shows[i].Size, shows[j].Name, shows[j].Size) {
				seen[normIndexer(shows[j].IndexerName)] = struct{}{}
			}
		}
		shows[i].CrossSeedCount = len(seen)
	}
}
