package requests

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/Cristian0711/media-bridge/backend/internal/hardlink"
	"github.com/Cristian0711/media-bridge/backend/internal/media"
	"github.com/Cristian0711/media-bridge/backend/internal/qbittorrent"
)

const defaultTorrentInfoCacheTTL = 3 * time.Second

// RequestTorrentInfo is returned by GET /requests/:id/torrent for live status polling.
type RequestTorrentInfo struct {
	RequestStatus string               `json:"request_status"`
	TorrentName   string               `json:"torrent_name"`
	Indexer       string               `json:"indexer"`
	Quality       string               `json:"quality"`
	TorrentHash   string               `json:"torrent_hash,omitempty"`
	Torrent       *qbittorrent.Torrent `json:"torrent,omitempty"`
	Hardlink      *hardlink.Progress   `json:"hardlink,omitempty"`
	Message       string               `json:"message,omitempty"`
}

// TorrentInfoInvalidator drops cached torrent-modal payloads when request state
// changes. The repository calls it on every status mutation.
type TorrentInfoInvalidator interface {
	InvalidateTorrentInfo(requestID uint)
}

// torrentInfoProvider builds the torrent-details payload for the request modal
// and caches it for a few seconds to absorb polling bursts. It implements
// TorrentInfoInvalidator so the repository can evict on status changes.
type torrentInfoProvider struct {
	mediaSvc    media.Service
	qbitSvc     qbittorrent.Service
	hardlinkSvc hardlink.Service
	cache       *torrentInfoCache
}

func newTorrentInfoProvider(
	mediaSvc media.Service,
	qbitSvc qbittorrent.Service,
	hardlinkSvc hardlink.Service,
	ttl time.Duration,
) *torrentInfoProvider {
	return &torrentInfoProvider{
		mediaSvc:    mediaSvc,
		qbitSvc:     qbitSvc,
		hardlinkSvc: hardlinkSvc,
		cache:       newTorrentInfoCache(ttl),
	}
}

func (p *torrentInfoProvider) cached(requestID uint) (*RequestTorrentInfo, bool) {
	return p.cache.get(requestID)
}

func (p *torrentInfoProvider) buildAndCache(ctx context.Context, req *Request) (*RequestTorrentInfo, error) {
	info, err := p.build(ctx, req)
	if err != nil {
		return nil, err
	}
	p.cache.set(req.ID, info)
	return info, nil
}

func (p *torrentInfoProvider) InvalidateTorrentInfo(requestID uint) {
	p.cache.invalidate(requestID)
}

func (p *torrentInfoProvider) build(ctx context.Context, req *Request) (*RequestTorrentInfo, error) {
	out := &RequestTorrentInfo{
		RequestStatus: req.Status,
		TorrentName:   req.TorrentName,
		Indexer:       req.Indexer,
		Quality:       req.Quality,
	}

	if !isDownloadType(req.Type) {
		out.Message = "not a download request"
		return out, nil
	}
	if req.MediaID == 0 {
		out.Message = "torrent not added yet"
		return out, nil
	}

	row, err := p.mediaSvc.GetMediaByID(ctx, req.MediaID)
	if err != nil {
		if errors.Is(err, media.ErrMediaNotFound) {
			out.Message = "media not found"
			return out, nil
		}
		return nil, err
	}

	hardlinkProgress, err := p.hardlinkSvc.ProgressForMedia(ctx, row)
	if err != nil {
		return nil, fmt.Errorf("hardlink status: %w", err)
	}
	out.Hardlink = hardlinkProgress

	hash := media.TorrentHashFromRow(row)
	if hash == "" {
		hash = hardlinkProgress.TorrentHash
	}
	if hash == "" {
		out.Message = "torrent hash not available yet"
		return out, nil
	}
	out.TorrentHash = hash

	torrent, err := p.qbitSvc.GetTorrent(ctx, hash)
	if err != nil {
		if errors.Is(err, qbittorrent.ErrTorrentNotFound) {
			out.Message = "torrent not found in qBittorrent"
			return out, nil
		}
		return nil, err
	}
	out.Torrent = torrent
	return out, nil
}

type torrentInfoCache struct {
	ttl   time.Duration
	mu    sync.Mutex
	items map[uint]cachedTorrentInfo
}

type cachedTorrentInfo struct {
	info      *RequestTorrentInfo
	expiresAt time.Time
}

func newTorrentInfoCache(ttl time.Duration) *torrentInfoCache {
	if ttl <= 0 {
		ttl = defaultTorrentInfoCacheTTL
	}
	return &torrentInfoCache{
		ttl:   ttl,
		items: make(map[uint]cachedTorrentInfo),
	}
}

func (c *torrentInfoCache) get(requestID uint) (*RequestTorrentInfo, bool) {
	now := time.Now()
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.items[requestID]
	if !ok || now.After(entry.expiresAt) {
		delete(c.items, requestID)
		return nil, false
	}
	return entry.info, true
}

func (c *torrentInfoCache) set(requestID uint, info *RequestTorrentInfo) {
	c.mu.Lock()
	c.items[requestID] = cachedTorrentInfo{
		info:      info,
		expiresAt: time.Now().Add(c.ttl),
	}
	c.mu.Unlock()
}

func (c *torrentInfoCache) invalidate(requestID uint) {
	c.mu.Lock()
	delete(c.items, requestID)
	c.mu.Unlock()
}
