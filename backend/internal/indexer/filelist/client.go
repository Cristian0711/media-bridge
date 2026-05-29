package filelist

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/Cristian0711/media-bridge/backend/shared/logger"
	"go.uber.org/zap"
)

type Config struct {
	Username string
	Passkey  string
	UUID     string
	PassID   string
	SessID   string
}

type Client struct {
	baseURL    string
	username   string
	passkey    string
	uuid       string
	passID     string
	sessID     string
	httpClient *http.Client
	log        *zap.Logger
}

func NewClient(cfg Config) *Client {
	transport := &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
		DisableKeepAlives:   false,
	}
	return &Client{
		baseURL:  "https://filelist.io",
		username: cfg.Username,
		passkey:  cfg.Passkey,
		uuid:     cfg.UUID,
		passID:   cfg.PassID,
		sessID:   cfg.SessID,
		httpClient: &http.Client{
			Timeout:   10 * time.Second,
			Transport: transport,
		},
		log: logger.Named("indexer.filelist.client"),
	}
}

func (c *Client) Request(ctx context.Context, params map[string]string, result any) error {
	u := fmt.Sprintf("%s/api.php", c.baseURL)
	q := url.Values{}
	q.Set("username", c.username)
	q.Set("passkey", c.passkey)
	for k, v := range params {
		q.Set(k, v)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u+"?"+q.Encode(), nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		c.log.Warn("filelist api non-200",
			zap.Int("status", resp.StatusCode),
			zap.Any("params", params),
			zap.String("body", string(body)))
		return fmt.Errorf("filelist API error: %s", resp.Status)
	}
	if err := json.Unmarshal(body, result); err != nil {
		c.log.Warn("filelist api decode failed", zap.Any("params", params), zap.Error(err))
		return err
	}
	return nil
}

func (c *Client) Browse(ctx context.Context, params map[string]string) (string, error) {
	u := fmt.Sprintf("%s/browse.php", c.baseURL)
	q := url.Values{}
	for k, v := range params {
		q.Set(k, v)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u+"?"+q.Encode(), nil)
	if err != nil {
		return "", err
	}
	req.AddCookie(&http.Cookie{Name: "uid", Value: c.uuid})
	req.AddCookie(&http.Cookie{Name: "pass", Value: c.passID})
	req.AddCookie(&http.Cookie{Name: "PHPSESSID", Value: c.sessID})
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		c.log.Warn("filelist browse non-200",
			zap.Int("status", resp.StatusCode),
			zap.Any("params", params),
			zap.String("body", string(body)))
		return "", fmt.Errorf("filelist browse error (%d): %s", resp.StatusCode, string(body))
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func (c *Client) DownloadTorrent(ctx context.Context, downloadURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	if err != nil {
		return "", err
	}
	req.AddCookie(&http.Cookie{Name: "uid", Value: c.uuid})
	req.AddCookie(&http.Cookie{Name: "pass", Value: c.passID})
	req.AddCookie(&http.Cookie{Name: "PHPSESSID", Value: c.sessID})
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(data), nil
}
