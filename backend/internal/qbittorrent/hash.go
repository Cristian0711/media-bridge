package qbittorrent

import "strings"

// NormalizeHash lowercases a torrent info hash for consistent map and DB lookups.
func NormalizeHash(hash string) string {
	return strings.ToLower(strings.TrimSpace(hash))
}
