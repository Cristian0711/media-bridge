package indexer

import (
	"regexp"
	"strings"
)

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

func qualityPriority(quality string) int {
	q := canonicalQuality(quality)
	for _, item := range qualities {
		if item.Label == q {
			return item.Priority
		}
	}
	return 0
}

// ParseTorrentQuality is the exported entry point used by indexer providers.
func ParseTorrentQuality(name string) string {
	return parseQuality(name)
}
