package requests

import "github.com/Cristian0711/media-bridge/backend/internal/sse"

// ToSSEPayload converts a request row into the wire format for SSE clients.
func ToSSEPayload(row *Request) *sse.RequestPayload {
	if row == nil {
		return nil
	}
	return &sse.RequestPayload{
		ID:          row.ID,
		Type:        row.Type,
		Status:      row.Status,
		Name:        row.Name,
		UserID:      row.UserID,
		Username:    row.Username,
		RequestID:   row.RequestID,
		MediaID:     row.MediaID,
		IMDBID:      row.IMDBID,
		TMDBID:      row.TMDBID,
		TVDBID:      row.TVDBID,
		Season:      row.Season,
		Episode:     row.Episode,
		PosterURL:   row.PosterURL,
		TorrentURL:  row.TorrentURL,
		TorrentName: row.TorrentName,
		Indexer:     row.Indexer,
		Quality:     row.Quality,
		CreatedAt:   row.CreatedAt,
		UpdatedAt:   row.UpdatedAt,
	}
}
