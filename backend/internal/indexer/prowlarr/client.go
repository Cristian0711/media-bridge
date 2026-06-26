package prowlarr

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

	"github.com/Cristian0711/media-bridge/backend/shared/logger"
	"go.uber.org/zap"
)

type Config struct {
	BaseURL string
	APIKey  string
}

type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
	log        *zap.Logger
}

func NewClient(cfg Config) *Client {
	base := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	return &Client{
		baseURL: base,
		apiKey:  strings.TrimSpace(cfg.APIKey),
		httpClient: &http.Client{
			Timeout: 90 * time.Second,
		},
		log: logger.Named("indexer.prowlarr"),
	}
}

func (c *Client) Search(ctx context.Context, query, searchType string, indexerIDs []int) ([]Release, error) {
	if c.baseURL == "" {
		return nil, fmt.Errorf("prowlarr base URL is not configured")
	}
	if c.apiKey == "" {
		return nil, fmt.Errorf("prowlarr API key is not configured")
	}
	if strings.TrimSpace(query) == "" {
		return nil, fmt.Errorf("search query is required")
	}

	params := url.Values{}
	params.Set("query", query)
	params.Set("type", searchType)
	params.Set("limit", "100")
	params.Set("offset", "0")
	if len(indexerIDs) > 0 {
		parts := make([]string, 0, len(indexerIDs))
		for _, id := range indexerIDs {
			parts = append(parts, strconv.Itoa(id))
		}
		params.Set("indexerIds", strings.Join(parts, ","))
	}

	u := c.baseURL + "/api/v1/search?" + params.Encode()
	c.log.Info("prowlarr search",
		zap.String("type", searchType),
		zap.String("query", query),
		zap.Int("indexer_filter_count", len(indexerIDs)))

	var releases []Release
	if err := c.getJSON(ctx, u, &releases); err != nil {
		return nil, err
	}
	c.log.Info("prowlarr search complete", zap.Int("results", len(releases)))
	return releases, nil
}

func (c *Client) ListIndexers(ctx context.Context) ([]Indexer, error) {
	if c.baseURL == "" {
		return nil, fmt.Errorf("prowlarr base URL is not configured")
	}
	u := c.baseURL + "/api/v1/indexer"
	var indexers []Indexer
	if err := c.getJSON(ctx, u, &indexers); err != nil {
		return nil, err
	}
	return indexers, nil
}

func (c *Client) DownloadTorrent(ctx context.Context, downloadURL string) (string, error) {
	if strings.TrimSpace(downloadURL) == "" {
		return "", fmt.Errorf("download URL is required")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	if err != nil {
		return "", err
	}
	if c.apiKey != "" && !strings.Contains(downloadURL, "apikey=") {
		req.Header.Set("X-Api-Key", c.apiKey)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("prowlarr download failed with status %d", resp.StatusCode)
	}
	// .torrent files are tiny; cap the read so a misbehaving or malicious
	// indexer proxy cannot stream an unbounded body and OOM the process. Read
	// one extra byte so we can detect (rather than silently truncate) overflow.
	const maxTorrentSize = 10 << 20 // 10 MiB
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxTorrentSize+1))
	if err != nil {
		return "", err
	}
	if len(data) > maxTorrentSize {
		return "", fmt.Errorf("prowlarr download exceeds %d bytes", maxTorrentSize)
	}
	return base64.StdEncoding.EncodeToString(data), nil
}

func (c *Client) getJSON(ctx context.Context, rawURL string, dest any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	if c.apiKey != "" {
		req.Header.Set("X-Api-Key", c.apiKey)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("prowlarr request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read prowlarr response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		snippet := string(body)
		if len(snippet) > 200 {
			snippet = snippet[:200]
		}
		return fmt.Errorf("prowlarr request failed with status %d: %s", resp.StatusCode, snippet)
	}
	if err := json.Unmarshal(body, dest); err != nil {
		return fmt.Errorf("decode prowlarr response: %w", err)
	}
	return nil
}

// IsProwlarrDownloadURL reports whether the URL is served by Prowlarr's download proxy.
func IsProwlarrDownloadURL(raw string) bool {
	lower := strings.ToLower(raw)
	return strings.Contains(lower, "/download") &&
		strings.Contains(lower, "apikey=") &&
		strings.Contains(lower, "link=")
}
