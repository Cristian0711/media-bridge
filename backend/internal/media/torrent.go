package media

import (
	"github.com/Cristian0711/media-bridge/backend/internal/qbittorrent"
)

// TorrentHashFromRow returns the normalized qBittorrent hash stored on the movie or show entry.
func TorrentHashFromRow(row *Media) string {
	if row == nil {
		return ""
	}
	var raw string
	switch row.Type {
	case MediaTypeMovie:
		if row.Movie != nil && row.Movie.TorrentHash != nil {
			raw = *row.Movie.TorrentHash
		}
	case MediaTypeShowFull, MediaTypeShowSeason, MediaTypeShowEpisode:
		if row.ShowEntry != nil && row.ShowEntry.TorrentHash != nil {
			raw = *row.ShowEntry.TorrentHash
		}
	}
	return qbittorrent.NormalizeHash(raw)
}
