package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/Cristian0711/media-bridge/backend/shared/logger"
)

// defaultQBittorrentPassword is the placeholder used when QBITTORRENT_PASSWORD
// is unset. Shipping it as a working default is a misconfiguration, so its use
// is warned about loudly at startup.
const defaultQBittorrentPassword = "changeme"

type AppConfig struct {
	Port        string
	DatabaseURL string
	JWTSecret   string

	MoviesPath    string
	ShowsPath     string
	DownloadsPath string

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
	Prowlarr ProwlarrConfig
}

type ProwlarrConfig struct {
	Enabled bool
	URL     string
	APIKey  string
}

func Load() (*AppConfig, error) {
	cfg := &AppConfig{
		Port:          get("PORT", "8080"),
		DatabaseURL:   os.Getenv("DATABASE_URL"),
		JWTSecret:     os.Getenv("JWT_SECRET"),
		MoviesPath:    get("MOVIES_PATH", "/mnt/plexmedia/movies"),
		ShowsPath:     get("SHOWS_PATH", "/mnt/plexmedia/shows"),
		DownloadsPath: get("DOWNLOADS_PATH", "/mnt/plexmedia/downloads"),
		QueueWorkers: QueueWorkersConfig{
			Requests: getInt("REQUESTS_QUEUE_WORKERS", 1),
			Download: getInt("DOWNLOAD_QUEUE_WORKERS", 2),
			Hardlink: getInt("HARDLINK_QUEUE_WORKERS", 2),
			Remove:   getInt("REMOVE_QUEUE_WORKERS", 1),
		},
		QBittorrent: QBittorrentConfig{
			URL:      get("QBITTORRENT_URL", "http://192.168.0.65:8090"),
			Username: get("QBITTORRENT_USERNAME", "admin"),
			Password: get("QBITTORRENT_PASSWORD", defaultQBittorrentPassword),
		},
		Indexer: IndexerConfig{
			Prowlarr: ProwlarrConfig{
				Enabled: getBool("PROWLARR_ENABLED", true),
				URL:     get("PROWLARR_URL", "http://127.0.0.1:9696"),
				APIKey:  get("PROWLARR_API_KEY", ""),
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

	if cfg.QBittorrent.Password == defaultQBittorrentPassword {
		logger.Component("config").Warn(
			"QBITTORRENT_PASSWORD is unset; using the insecure built-in default — set it in the environment",
		)
	}
	if cfg.Indexer.Prowlarr.Enabled && cfg.Indexer.Prowlarr.APIKey == "" {
		logger.Component("config").Warn(
			"Prowlarr is enabled but PROWLARR_API_KEY is empty; indexer requests will be unauthenticated",
		)
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
