package media

import "github.com/Cristian0711/media-bridge/backend/internal/sse"

// ToSSEPayload converts a loaded media row into the wire format for SSE clients.
func ToSSEPayload(row *Media) *sse.MediaPayload {
	if row == nil {
		return nil
	}
	out := &sse.MediaPayload{
		ID:        row.ID,
		Type:      string(row.Type),
		Name:      row.Name,
		Path:      row.Path,
		Indexer:   row.Indexer,
		Quality:   row.Quality,
		SizeBytes: row.SizeBytes,
		UserID:    row.UserID,
		Username:  row.Username,
		CreatedAt: row.CreatedAt,
		UpdatedAt: row.UpdatedAt,
		MovieID:   row.MovieID,
	}
	if row.Movie != nil {
		out.Movie = &sse.MoviePayload{
			IMDBID:    row.Movie.IMDBID,
			TMDBID:    row.Movie.TMDBID,
			Year:      row.Movie.Year,
			PosterURL: row.Movie.PosterURL,
		}
	}
	if row.ShowEntry != nil {
		entry := &sse.ShowEntryPayload{
			Season:  row.ShowEntry.Season,
			Episode: row.ShowEntry.Episode,
		}
		if row.ShowEntry.Show != nil {
			entry.Show = &sse.ShowPayload{
				IMDBID:    row.ShowEntry.Show.IMDBID,
				TVDBID:    row.ShowEntry.Show.TVDBID,
				PosterURL: row.ShowEntry.Show.PosterURL,
			}
		}
		out.ShowEntry = entry
	}
	return out
}
