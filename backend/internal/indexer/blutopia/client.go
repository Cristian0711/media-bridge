package blutopia

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const defaultBaseURL = "https://blutopia.cc"

type Config struct {
	BaseURL string
	APIKey  string
}

type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

type filterResponse struct {
	Data []torrentResource `json:"data"`
}

type torrentResource struct {
	ID         string            `json:"id"`
	Attributes torrentAttributes `json:"attributes"`
}

type torrentAttributes struct {
	Name            string  `json:"name"`
	Category        string  `json:"category"`
	CategoryID      int     `json:"category_id"`
	Type            string  `json:"type"`
	Resolution      string  `json:"resolution"`
	Size            int64   `json:"size"`
	Seeders         int     `json:"seeders"`
	Leechers        int     `json:"leechers"`
	TimesCompleted  int     `json:"times_completed"`
	IMDBID          *int    `json:"imdb_id"`
	TMDBID          *int    `json:"tmdb_id"`
	TVDBID          *int    `json:"tvdb_id"`
	Freeleech       string  `json:"freeleech"`
	CreatedAt       string  `json:"created_at"`
	DownloadLink    string  `json:"download_link"`
}

func NewClient(cfg Config) *Client {
	base := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if base == "" {
		base = defaultBaseURL
	}
	return &Client{
		baseURL: base,
		apiKey:  strings.TrimSpace(cfg.APIKey),
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        20,
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:     90 * time.Second,
			},
		},
	}
}

// FilterFreeleech returns freeleech torrents for the given IMDb id (movies and TV mixed).
func (c *Client) FilterFreeleech(ctx context.Context, imdbID string) ([]torrentResource, error) {
	numericID, err := normalizeIMDbNumeric(imdbID)
	if err != nil {
		return nil, err
	}

	u, err := url.Parse(c.baseURL + "/api/torrents/filter")
	if err != nil {
		return nil, err
	}
	q := u.Query()
	q.Set("imdbId", numericID)
	q.Set("free", "100")
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Media-Bridge/1.0")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("blutopia API error: %s — %s", resp.Status, truncateBody(body))
	}

	var parsed filterResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("decode blutopia response: %w", err)
	}

	out := make([]torrentResource, 0, len(parsed.Data))
	for _, item := range parsed.Data {
		if !isFreeleech(item.Attributes.Freeleech) {
			continue
		}
		out = append(out, item)
	}
	return out, nil
}

func (c *Client) DownloadTorrent(ctx context.Context, downloadURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	if err != nil {
		return "", err
	}
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
	req.Header.Set("User-Agent", "Media-Bridge/1.0")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("blutopia download failed: %s — %s", resp.Status, truncateBody(body))
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(data), nil
}

func normalizeIMDbNumeric(imdb string) (string, error) {
	s := strings.TrimSpace(imdb)
	s = strings.TrimPrefix(strings.TrimPrefix(s, "tt"), "TT")
	if s == "" {
		return "", fmt.Errorf("imdb_id is required")
	}
	if _, err := strconv.Atoi(s); err != nil {
		return "", fmt.Errorf("invalid imdb_id: %s", imdb)
	}
	return s, nil
}

func isFreeleech(v string) bool {
	v = strings.TrimSpace(strings.ToLower(v))
	if v == "" {
		return true // free=100 query already applied
	}
	return strings.Contains(v, "100")
}

func isMovieCategory(category string, categoryID int) bool {
	if categoryID == 1 {
		return true
	}
	return strings.EqualFold(strings.TrimSpace(category), "Movie")
}

func isShowCategory(category string, categoryID int) bool {
	if isMovieCategory(category, categoryID) {
		return false
	}
	lower := strings.ToLower(category)
	if strings.Contains(lower, "tv") || strings.Contains(lower, "show") || strings.Contains(lower, "episode") {
		return true
	}
	// Non-movie results from TV filter often use other category ids.
	return categoryID != 0 && categoryID != 1
}

func truncateBody(b []byte) string {
	const max = 200
	if len(b) <= max {
		return string(b)
	}
	return string(b[:max]) + "…"
}
