package torrentleech

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

type Config struct {
	PHPSESSID   string
	TLUid       string
	TLPass      string
	LastBrowse1 string
	LastBrowse2 string
}

type Client struct {
	baseURL    string
	httpClient *http.Client
	cookies    map[string]string
}

type Response struct {
	NumFound    int                    `json:"numFound"`
	TorrentList []Item                 `json:"torrentList"`
	Facets      map[string]interface{} `json:"facets,omitempty"`
	FacetsWOC   map[string]interface{} `json:"facetswoc,omitempty"`
	Order       string                 `json:"order,omitempty"`
	OrderBy     string                 `json:"orderBy,omitempty"`
	Page        int                    `json:"page,omitempty"`
	PerPage     int                    `json:"perPage,omitempty"`
}

type Item struct {
	Fid                string   `json:"fid"`
	Filename           string   `json:"filename"`
	Name               string   `json:"name"`
	CategoryID         int      `json:"categoryID"`
	Size               int64    `json:"size"`
	Completed          int      `json:"completed"`
	Seeders            int      `json:"seeders"`
	Leechers           int      `json:"leechers"`
	NumComments        int      `json:"numComments"`
	Tags               []string `json:"tags"`
	New                bool     `json:"new"`
	ImdbID             string   `json:"imdbID"`
	Rating             float64  `json:"rating"`
	Genres             string   `json:"genres"`
	TvMazeID           string   `json:"tvmazeID"`
	IgdbID             string   `json:"igdbID"`
	AnimeID            any      `json:"animeID"`
	AddedTimestamp     string   `json:"addedTimestamp"`
	DownloadMultiplier float64  `json:"download_multiplier"`
	CommentsDisabled   int      `json:"commentsDisabled"`
	Uploader           string   `json:"uploader,omitempty"`
}

func NewClient(cfg Config) *Client {
	cookies := make(map[string]string)
	if cfg.PHPSESSID != "" {
		cookies["PHPSESSID"] = cfg.PHPSESSID
	}
	if cfg.TLUid != "" {
		cookies["tluid"] = cfg.TLUid
	}
	if cfg.TLPass != "" {
		cookies["tlpass"] = cfg.TLPass
	}
	if cfg.LastBrowse1 != "" {
		cookies["lastBrowse1"] = cfg.LastBrowse1
	}
	if cfg.LastBrowse2 != "" {
		cookies["lastBrowse2"] = cfg.LastBrowse2
	}
	transport := &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
		DisableKeepAlives:   false,
	}
	return &Client{
		baseURL: "https://www.torrentleech.org",
		cookies: cookies,
		httpClient: &http.Client{
			Timeout:   10 * time.Second,
			Transport: transport,
		},
	}
}

func (c *Client) Search(ctx context.Context, query string) (*Response, error) {
	u := fmt.Sprintf("%s/torrents/browse/list/query/%s", c.baseURL, url.PathEscape(query))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	for n, v := range c.cookies {
		if v != "" {
			req.AddCookie(&http.Cookie{Name: n, Value: v})
		}
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("torrentleech API error: %s", resp.Status)
	}
	var result Response
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) DownloadTorrent(ctx context.Context, downloadURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	if err != nil {
		return "", err
	}
	for n, v := range c.cookies {
		if v != "" {
			req.AddCookie(&http.Cookie{Name: n, Value: v})
		}
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download failed with status %d", resp.StatusCode)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(data), nil
}
