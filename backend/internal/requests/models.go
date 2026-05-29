package requests

import (
	"time"

	"gorm.io/gorm"
)

type Request struct {
	ID uint `gorm:"primarykey" json:"id"`

	Type   string `gorm:"not null;index" json:"type"`
	Status string `gorm:"not null;index" json:"status"`

	Name        string `gorm:"not null" json:"name"`
	UserID      uint   `gorm:"not null;index" json:"user_id"`
	Username    string `gorm:"not null;index" json:"username"`
	RequestID   string `gorm:"not null;index" json:"request_id"`
	MediaID     uint   `gorm:"index" json:"media_id,omitempty"`
	IMDBID      string `gorm:"index" json:"imdb_id,omitempty"`
	TMDBID      string `json:"tmdb_id,omitempty"`
	TVDBID      string `json:"tvdb_id,omitempty"`
	Season      int    `json:"season,omitempty"`
	Episode     int    `json:"episode,omitempty"`
	PosterURL   string `json:"poster_url,omitempty"`
	TorrentURL  string `json:"torrent_url,omitempty"`
	TorrentName string `json:"torrent_name,omitempty"`
	Indexer     string `gorm:"not null" json:"indexer"`
	Quality     string `gorm:"not null" json:"quality"`

	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}
