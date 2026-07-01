package qbittorrent

import (
	"context"
	"time"

	"github.com/Cristian0711/media-bridge/backend/shared/logger"
	"github.com/Cristian0711/media-bridge/backend/shared/ssehub"
	"github.com/Cristian0711/media-bridge/backend/shared/telemetry"
)

type EventType string

const (
	EventTypeTorrentAdded   EventType = "torrent_added"
	EventTypeTorrentRemoved EventType = "torrent_removed"
	EventTypeTorrentUpdated EventType = "torrent_updated"
	EventTypeHeartbeat      EventType = "heartbeat"
)

type TorrentEvent struct {
	Type       EventType `json:"type"`
	Hash       string    `json:"hash"`
	Name       string    `json:"name,omitempty"`
	Size       int64     `json:"size,omitempty"`
	State      string    `json:"state,omitempty"`
	Progress   float64   `json:"progress,omitempty"`
	Seeders    int       `json:"seeders,omitempty"`
	Leechers   int       `json:"leechers,omitempty"`
	Downloaded int64     `json:"downloaded,omitempty"`
	Uploaded   int64     `json:"uploaded,omitempty"`
	DlSpeed    int64     `json:"dlspeed,omitempty"`
	UpSpeed    int64     `json:"upspeed,omitempty"`
	ETA        int64     `json:"eta,omitempty"`
}

// Client is one subscriber to the torrent event stream.
type Client = ssehub.Client

// NewClient allocates a buffered channel for outbound SSE frames.
func NewClient(id string) *Client { return ssehub.NewClient(id) }

// Broker fans out torrent events to connected clients. It is a thin wrapper
// over the shared ssehub.Hub.
type Broker struct {
	hub *ssehub.Hub
}

// NewBroker starts the broker goroutine. Call Shutdown on process exit.
func NewBroker() *Broker {
	return &Broker{hub: ssehub.New("qbittorrent.sse")}
}

// AddClient registers a subscriber.
func (b *Broker) AddClient(c *Client) { b.hub.AddClient(c) }

// RemoveClient disconnects a subscriber by ID.
func (b *Broker) RemoveClient(id string) { b.hub.RemoveClient(id) }

func (b *Broker) BroadcastTorrentAdded(t Torrent) {
	b.broadcastEvent(TorrentEvent{
		Type:       EventTypeTorrentAdded,
		Hash:       t.Hash,
		Name:       t.Name,
		Size:       t.Size,
		State:      t.State,
		Progress:   t.Progress,
		Seeders:    t.Seeders,
		Leechers:   t.Leechers,
		Downloaded: t.Downloaded,
		Uploaded:   t.Uploaded,
		DlSpeed:    t.DlSpeed,
		UpSpeed:    t.UpSpeed,
		ETA:        t.ETA,
	})
}

func (b *Broker) BroadcastTorrentRemoved(hash string) {
	b.broadcastEvent(TorrentEvent{
		Type: EventTypeTorrentRemoved,
		Hash: hash,
	})
}

func (b *Broker) BroadcastTorrentUpdated(t Torrent) {
	b.broadcastEvent(TorrentEvent{
		Type:       EventTypeTorrentUpdated,
		Hash:       t.Hash,
		Name:       t.Name,
		State:      t.State,
		Progress:   t.Progress,
		Seeders:    t.Seeders,
		Leechers:   t.Leechers,
		Downloaded: t.Downloaded,
		Uploaded:   t.Uploaded,
		DlSpeed:    t.DlSpeed,
		UpSpeed:    t.UpSpeed,
		ETA:        t.ETA,
	})
}

func (b *Broker) broadcastEvent(event TorrentEvent) {
	b.hub.Publish(ssehub.Format(string(event.Type), event))
}

// Shutdown closes all clients and stops the broker loop.
func (b *Broker) Shutdown() { b.hub.Shutdown() }

// GetClientCount returns the number of active SSE connections.
func (b *Broker) GetClientCount() int { return b.hub.ClientCount() }

func StartTorrentMonitor(ctx context.Context, svc Service, broker *Broker, interval time.Duration) {
	if interval <= 0 {
		interval = 2 * time.Second
	}
	ctx = telemetry.WithoutTracing(logger.WithSystem(ctx, "qbittorrent.torrent_monitor"))

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		var previous map[string]Torrent
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				torrents, err := svc.ListTorrents(ctx)
				if err != nil {
					// Transient poll failure (qBittorrent briefly unreachable):
					// expected, so log at debug rather than dropping it silently.
					logger.Debug(ctx, "torrent monitor: list failed", logger.Err(err))
					continue
				}

				current := make(map[string]Torrent, len(torrents))
				for _, t := range torrents {
					current[t.Hash] = t
				}

				if previous == nil {
					previous = current
					continue
				}

				for hash, cur := range current {
					prev, exists := previous[hash]
					if !exists {
						broker.BroadcastTorrentAdded(cur)
						continue
					}
					if torrentChanged(prev, cur) {
						broker.BroadcastTorrentUpdated(cur)
					}
				}
				for hash := range previous {
					if _, exists := current[hash]; !exists {
						broker.BroadcastTorrentRemoved(hash)
					}
				}
				previous = current
			}
		}
	}()
}

func torrentChanged(old Torrent, current Torrent) bool {
	return old.State != current.State ||
		old.Progress != current.Progress ||
		old.DlSpeed != current.DlSpeed ||
		old.UpSpeed != current.UpSpeed ||
		old.Seeders != current.Seeders ||
		old.Leechers != current.Leechers ||
		old.Downloaded != current.Downloaded ||
		old.Uploaded != current.Uploaded
}
