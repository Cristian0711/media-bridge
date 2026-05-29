package sse

import "time"

// EventType is the SSE "event:" field and JSON "type" on the wire.
type EventType string

const (
	EventConnected            EventType = "connected"
	EventHeartbeat            EventType = "heartbeat"
	EventMediaAdded           EventType = "media.added"
	EventMediaRemoved         EventType = "media.removed"
	EventRequestCreated       EventType = "request.created"
	EventRequestStatusChanged EventType = "request.status_changed"
)

// Envelope is the JSON payload sent in every SSE data line.
// Payload types live in this package so sse does not import media/requests (avoids import cycles).
type Envelope struct {
	Type      EventType       `json:"type"`
	Timestamp int64           `json:"timestamp"`
	Media     *MediaPayload   `json:"media,omitempty"`
	Request   *RequestPayload `json:"request,omitempty"`
}

// MediaPayload mirrors the media API JSON shape for live updates.
type MediaPayload struct {
	ID        uint                `json:"id"`
	Type      string              `json:"type"`
	Name      string              `json:"name"`
	Path      string              `json:"path"`
	Indexer   string              `json:"indexer"`
	Quality   string              `json:"quality"`
	SizeBytes int64               `json:"size_bytes,omitempty"`
	UserID    uint                `json:"user_id"`
	Username  string              `json:"username"`
	CreatedAt time.Time           `json:"created_at"`
	UpdatedAt time.Time           `json:"updated_at"`
	MovieID   *uint               `json:"movie_id,omitempty"`
	ShowEntry *ShowEntryPayload   `json:"show_entry,omitempty"`
	Movie     *MoviePayload       `json:"movie,omitempty"`
}

type MoviePayload struct {
	IMDBID    string  `json:"imdb_id,omitempty"`
	TMDBID    string  `json:"tmdb_id,omitempty"`
	Year      *int    `json:"year,omitempty"`
	PosterURL *string `json:"poster_url,omitempty"`
}

type ShowEntryPayload struct {
	Season  *int         `json:"season,omitempty"`
	Episode *int         `json:"episode,omitempty"`
	Show    *ShowPayload `json:"show,omitempty"`
}

type ShowPayload struct {
	IMDBID    string  `json:"imdb_id,omitempty"`
	TVDBID    string  `json:"tvdb_id,omitempty"`
	PosterURL *string `json:"poster_url,omitempty"`
}

// RequestPayload mirrors the requests list API JSON for live updates.
type RequestPayload struct {
	ID          uint      `json:"id"`
	Type        string    `json:"type"`
	Status      string    `json:"status"`
	Name        string    `json:"name"`
	UserID      uint      `json:"user_id"`
	Username    string    `json:"username"`
	RequestID   string    `json:"request_id"`
	MediaID     uint      `json:"media_id,omitempty"`
	IMDBID      string    `json:"imdb_id,omitempty"`
	TMDBID      string    `json:"tmdb_id,omitempty"`
	TVDBID      string    `json:"tvdb_id,omitempty"`
	Season      int       `json:"season,omitempty"`
	Episode     int       `json:"episode,omitempty"`
	PosterURL   string    `json:"poster_url,omitempty"`
	TorrentURL  string    `json:"torrent_url,omitempty"`
	TorrentName string    `json:"torrent_name,omitempty"`
	Indexer     string    `json:"indexer"`
	Quality     string    `json:"quality"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// ConnectedPayload is sent once when a client attaches to the stream.
type ConnectedPayload struct {
	Type      EventType `json:"type"`
	ClientID  string    `json:"client_id"`
	Timestamp int64     `json:"timestamp"`
}

func unixNow() int64 {
	return time.Now().UTC().Unix()
}
