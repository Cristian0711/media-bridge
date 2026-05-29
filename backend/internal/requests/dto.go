package requests

import "github.com/Cristian0711/media-bridge/backend/internal/sse"

// MovieDownloadRequestBody defines expected JSON body for movie downloads.
type MovieDownloadRequestBody struct {
	Name        string `json:"name" binding:"required"`
	IMDBID      string `json:"imdb_id" binding:"required"`
	TMDBID      string `json:"tmdb_id"`
	PosterURL   string `json:"poster_url"`
	TorrentURL  string `json:"torrent_url" binding:"required"`
	TorrentName string `json:"torrent_name" binding:"required"`
	Indexer     string `json:"indexer" binding:"required"`
	Quality     string `json:"quality" binding:"required"`
}

// ShowDownloadRequestBody defines expected JSON body for show downloads.
type ShowDownloadRequestBody struct {
	Name        string `json:"name" binding:"required"`
	Season      int    `json:"season" binding:"required"`
	Episode     int    `json:"episode"`
	IMDBID      string `json:"imdb_id"`
	TVDBID      string `json:"tvdb_id"`
	PosterURL   string `json:"poster_url"`
	TorrentURL  string `json:"torrent_url" binding:"required"`
	TorrentName string `json:"torrent_name" binding:"required"`
	Indexer     string `json:"indexer" binding:"required"`
	Quality     string `json:"quality" binding:"required"`
}

// MovieRemoveRequestBody defines expected JSON body for movie removal.
type MovieRemoveRequestBody struct {
	MediaID uint `json:"media_id" binding:"required"`
}

// ShowRemoveRequestBody defines expected JSON body for show removal.
type ShowRemoveRequestBody struct {
	MediaID uint `json:"media_id" binding:"required"`
}

type RequestAck struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

type PaginatedRequestsResponse struct {
	Requests   []Request `json:"requests"`
	Page       int       `json:"page"`
	PageSize   int       `json:"page_size"`
	TotalCount int64     `json:"total_count"`
	TotalPages int       `json:"total_pages"`
}

type QueueEntryResponse struct {
	ID             string `json:"id"`
	QueueName      string `json:"queue_name"`
	Status         string `json:"status"`
	Attempts       int    `json:"attempts"`
	RequestEntryID uint   `json:"request_entry_id"`
	RequestID      string `json:"request_id"`
	UserID         uint   `json:"user_id"`
	Username       string `json:"username"`
	Type           string `json:"type"`
	CreatedAt      string `json:"created_at"`
}

type PaginatedQueueEntriesResponse struct {
	Entries    []QueueEntryResponse `json:"entries"`
	Page       int                  `json:"page"`
	PageSize   int                  `json:"page_size"`
	TotalCount int64                `json:"total_count"`
	TotalPages int                  `json:"total_pages"`
}

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
