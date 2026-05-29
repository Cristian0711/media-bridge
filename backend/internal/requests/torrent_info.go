package requests

import (
	"context"
	"errors"
	"fmt"

	"github.com/Cristian0711/media-bridge/backend/internal/hardlink"
	"github.com/Cristian0711/media-bridge/backend/internal/media"
	"github.com/Cristian0711/media-bridge/backend/internal/qbittorrent"
)

// RequestTorrentInfo is returned by GET /requests/:id/torrent for live status polling.
type RequestTorrentInfo struct {
	RequestStatus    string               `json:"request_status"`
	TorrentName      string               `json:"torrent_name"`
	Indexer          string               `json:"indexer"`
	Quality          string               `json:"quality"`
	TorrentHash      string               `json:"torrent_hash,omitempty"`
	Torrent          *qbittorrent.Torrent `json:"torrent,omitempty"`
	Hardlink         *hardlink.Progress     `json:"hardlink,omitempty"`
	Message          string               `json:"message,omitempty"`
}

type torrentInfoDeps struct {
	mediaSvc    media.Service
	qbitSvc     qbittorrent.Service
	hardlinkSvc hardlink.Service
	cache       *torrentInfoCache
}

func (d torrentInfoDeps) build(ctx context.Context, req *Request) (*RequestTorrentInfo, error) {
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

	row, err := d.mediaSvc.GetMediaByID(ctx, req.MediaID)
	if err != nil {
		if errors.Is(err, media.ErrMediaNotFound) {
			out.Message = "media not found"
			return out, nil
		}
		return nil, err
	}

	hardlinkProgress, err := d.hardlinkSvc.ProgressForMedia(ctx, row)
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

	torrent, err := d.qbitSvc.GetTorrent(ctx, hash)
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

func isDownloadType(t string) bool {
	return t == "movie_download" || t == "show_download"
}
