package media

import (
	"strconv"
	"time"

	"gorm.io/gorm"
)

// MediaType represents the type of media entry.
type MediaType string

const (
	MediaTypeMovie       MediaType = "movie"
	MediaTypeShowFull    MediaType = "show_full"
	MediaTypeShowSeason  MediaType = "show_season"
	MediaTypeShowEpisode MediaType = "show_episode"
)

// Media represents a media entry in the system (SSOT).
type Media struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	Type      MediaType      `gorm:"type:varchar(20);not null;index" json:"type"`
	Name      string         `gorm:"not null;index" json:"name"`
	Path        string         `gorm:"type:text;not null" json:"path"`
	LibraryPath string         `gorm:"type:text" json:"library_path,omitempty"`
	Indexer     string         `gorm:"type:varchar(100);not null;index" json:"indexer"`
	Quality   string         `gorm:"type:varchar(100);not null;index" json:"quality"`
	SizeBytes int64          `gorm:"default:0" json:"size_bytes,omitempty"`
	UserID    uint           `gorm:"not null;index" json:"user_id"`
	Username  string         `gorm:"type:varchar(100);not null;index" json:"username"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	MovieID     *uint      `json:"movie_id,omitempty"`
	ShowEntryID *uint      `json:"show_entry_id,omitempty"`
	Movie       *Movie     `gorm:"foreignKey:MovieID" json:"movie,omitempty"`
	ShowEntry   *ShowEntry `gorm:"foreignKey:ShowEntryID" json:"show_entry,omitempty"`
}

// Movie stores movie-specific information.
type Movie struct {
	ID          uint       `gorm:"primaryKey" json:"id"`
	IMDBID      string     `gorm:"type:varchar(20);index" json:"imdb_id,omitempty"`
	TMDBID      string     `gorm:"type:varchar(20);index" json:"tmdb_id,omitempty"`
	Year        *int       `json:"year,omitempty"`
	PosterURL   *string    `gorm:"type:text" json:"poster_url,omitempty"`
	TorrentHash *string    `gorm:"type:varchar(100)" json:"torrent_hash,omitempty"`
	TorrentURL  *string    `gorm:"type:text" json:"torrent_url,omitempty"`
	TorrentName *string    `gorm:"type:varchar(255)" json:"torrent_name,omitempty"`
	SavePath    *string    `gorm:"type:text" json:"save_path,omitempty"`
	StartedAt   *time.Time `json:"started_at,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}

// Show stores show metadata.
type Show struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Name      string    `gorm:"not null;index" json:"name"`
	IMDBID    string    `gorm:"type:varchar(20);index" json:"imdb_id,omitempty"`
	TVDBID    string    `gorm:"type:varchar(20);index" json:"tvdb_id,omitempty"`
	PosterURL *string   `gorm:"type:text" json:"poster_url,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ShowEntry represents what we actually have for a show.
type ShowEntry struct {
	ID     uint  `gorm:"primaryKey" json:"id"`
	ShowID uint  `gorm:"not null;index" json:"show_id"`
	Show   *Show `gorm:"foreignKey:ShowID" json:"show,omitempty"`

	Season  *int `gorm:"index" json:"season,omitempty"`
	Episode *int `json:"episode,omitempty"`

	TorrentHash *string    `gorm:"type:varchar(100)" json:"torrent_hash,omitempty"`
	TorrentURL  *string    `gorm:"type:text" json:"torrent_url,omitempty"`
	TorrentName *string    `gorm:"type:varchar(255)" json:"torrent_name,omitempty"`
	SavePath    *string    `gorm:"type:text" json:"save_path,omitempty"`
	StartedAt   *time.Time `json:"started_at,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`

	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// GetMediaIdentifier returns a unique identifier for the media.
func (m *Media) GetMediaIdentifier() string {
	if m.Type == MediaTypeMovie && m.Movie != nil {
		if m.Movie.IMDBID != "" {
			return "movie:" + m.Movie.IMDBID
		}
		if m.Movie.TMDBID != "" {
			return "movie:tmdb:" + m.Movie.TMDBID
		}
	}
	return ""
}

// GetShowEntryType returns the type of show entry.
func (se *ShowEntry) GetShowEntryType() MediaType {
	if se.Season == nil && se.Episode == nil {
		return MediaTypeShowFull
	}
	if se.Season != nil && se.Episode == nil {
		return MediaTypeShowSeason
	}
	return MediaTypeShowEpisode
}

// GetIdentifier returns a unique identifier for the show entry.
func (se *ShowEntry) GetIdentifier() string {
	entryType := se.GetShowEntryType()
	switch entryType {
	case MediaTypeShowFull:
		return "show_full"
	case MediaTypeShowSeason:
		return "show_season:" + strconv.Itoa(*se.Season)
	case MediaTypeShowEpisode:
		return "show_episode:" + strconv.Itoa(*se.Season) + ":" + strconv.Itoa(*se.Episode)
	}
	return ""
}
