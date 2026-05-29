package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

type AppConfig struct {
	Port        string
	DatabaseURL string
	JWTSecret   string

	MoviesPath string
	ShowsPath  string

	QueueWorkers QueueWorkersConfig

	QBittorrent QBittorrentConfig
	Indexer     IndexerConfig
	TMDB        TMDBConfig
}

// QueueWorkersConfig controls concurrent processing-queue workers per pipeline (6.2).
type QueueWorkersConfig struct {
	Requests int
	Download int
	Hardlink int
	Remove   int
}

type TMDBConfig struct {
	URL    string
	APIKey string
}

type QBittorrentConfig struct {
	URL      string
	Username string
	Password string
}

type IndexerConfig struct {
	FileList     FileListConfig
	TorrentLeech TorrentLeechConfig
}

type FileListConfig struct {
	Enabled  bool
	Username string
	PassKey  string
	UUID     string
	PassID   string
	SessID   string
}

type TorrentLeechConfig struct {
	Enabled     bool
	PHPSESSID   string
	TLUID       string
	TLPASS      string
	LastBrowse1 string
	LastBrowse2 string
}

func Load() (*AppConfig, error) {
	cfg := &AppConfig{
		Port:        get("PORT", "8080"),
		DatabaseURL: os.Getenv("DATABASE_URL"),
		JWTSecret:   os.Getenv("JWT_SECRET"),
		MoviesPath:  get("MOVIES_PATH", "/mnt/plexmedia/movies"),
		ShowsPath:   get("SHOWS_PATH", "/mnt/plexmedia/shows"),
		QueueWorkers: QueueWorkersConfig{
			Requests: getInt("REQUESTS_QUEUE_WORKERS", 1),
			Download: getInt("DOWNLOAD_QUEUE_WORKERS", 2),
			Hardlink: getInt("HARDLINK_QUEUE_WORKERS", 2),
			Remove:   getInt("REMOVE_QUEUE_WORKERS", 1),
		},
		QBittorrent: QBittorrentConfig{
			URL:      get("QBITTORRENT_URL", "http://192.168.0.65:8090"),
			Username: get("QBITTORRENT_USERNAME", "admin"),
			Password: get("QBITTORRENT_PASSWORD", "changeme"),
		},
		Indexer: IndexerConfig{
			FileList: FileListConfig{
				Enabled:  getBool("FILELIST_ENABLED", true),
				Username: get("FILELIST_USERNAME", ""),
				PassKey:  get("FILELIST_PASS_KEY", ""),
				UUID:     get("FILELIST_UUID", ""),
				PassID:   get("FILELIST_PASS_ID", ""),
				SessID:   get("FILELIST_SESS_ID", ""),
			},
			TorrentLeech: TorrentLeechConfig{
				Enabled:     getBool("TORRENTLEECH_ENABLED", true),
				PHPSESSID:   get("TORRENTLEECH_PHPSESSID", ""),
				TLUID:       get("TORRENTLEECH_TLUID", ""),
				TLPASS:      get("TORRENTLEECH_TLPASS", ""),
				LastBrowse1: get("TORRENTLEECH_LASTBROWSE1", ""),
				LastBrowse2: get("TORRENTLEECH_LASTBROWSE2", ""),
			},
		},
		TMDB: TMDBConfig{
			URL:    get("TMDB_URL", "https://api.themoviedb.org/3"),
			APIKey: os.Getenv("TMDB_API_KEY"),
		},
	}

	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("required env var not set: DATABASE_URL")
	}
	if cfg.JWTSecret == "" {
		return nil, fmt.Errorf("required env var not set: JWT_SECRET")
	}
	if cfg.TMDB.APIKey == "" {
		return nil, fmt.Errorf("required env var not set: TMDB_API_KEY")
	}
	return cfg, nil
}

func get(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getBool(key string, fallback bool) bool {
	v := strings.TrimSpace(strings.ToLower(os.Getenv(key)))
	if v == "" {
		return fallback
	}
	return v != "false" && v != "0" && v != "no"
}

func getInt(key string, fallback int) int {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 1 {
		return fallback
	}
	return n
}
