package qbittorrent

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/Cristian0711/media-bridge/backend/shared/logger"
	"go.uber.org/zap"
)

type EventType string

const (
	EventTypeTorrentAdded   EventType = "torrent_added"
	EventTypeTorrentRemoved EventType = "torrent_removed"
	EventTypeTorrentUpdated EventType = "torrent_updated"
	EventTypeHeartbeat      EventType = "heartbeat"
	EventTypeConnected      EventType = "connected"
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

type Client struct {
	ID       string
	Messages chan string
	done     chan struct{}
	once     sync.Once
}

func NewClient(id string) *Client {
	return &Client{
		ID:       id,
		Messages: make(chan string, 100),
		done:     make(chan struct{}),
	}
}

func (c *Client) Close() {
	c.once.Do(func() {
		close(c.done)
		close(c.Messages)
	})
}

type Broker struct {
	clients    map[string]*Client
	clientsMux sync.RWMutex

	addClient    chan *Client
	removeClient chan string
	broadcast    chan string
	shutdown     chan struct{}
}

func NewBroker() *Broker {
	b := &Broker{
		clients:      make(map[string]*Client),
		addClient:    make(chan *Client),
		removeClient: make(chan string),
		broadcast:    make(chan string, 100),
		shutdown:     make(chan struct{}),
	}
	go b.run()
	return b
}

func (b *Broker) run() {
	log := logger.Named("qbittorrent.sse")
	heartbeatTicker := time.NewTicker(30 * time.Second)
	defer heartbeatTicker.Stop()

	for {
		select {
		case client := <-b.addClient:
			b.clientsMux.Lock()
			b.clients[client.ID] = client
			b.clientsMux.Unlock()
			log.Info("client connected",
				zap.String("event", "client_connected"),
				zap.String("client_id", client.ID),
				zap.Int("clients", b.GetClientCount()),
			)

		case clientID := <-b.removeClient:
			b.clientsMux.Lock()
			if client, ok := b.clients[clientID]; ok {
				client.Close()
				delete(b.clients, clientID)
				log.Info("client disconnected",
					zap.String("event", "client_disconnected"),
					zap.String("client_id", clientID),
					zap.Int("clients", len(b.clients)),
				)
			}
			b.clientsMux.Unlock()

		case message := <-b.broadcast:
			b.clientsMux.RLock()
			for _, client := range b.clients {
				select {
				case client.Messages <- message:
				default:
					log.Warn("client queue full",
						zap.String("event", "client_queue_full"),
						zap.String("client_id", client.ID),
					)
				}
			}
			b.clientsMux.RUnlock()

		case <-heartbeatTicker.C:
			heartbeat := b.formatSSEMessage(EventTypeHeartbeat, map[string]any{"timestamp": time.Now().Unix()})
			b.clientsMux.RLock()
			for _, client := range b.clients {
				select {
				case client.Messages <- heartbeat:
				default:
				}
			}
			b.clientsMux.RUnlock()

		case <-b.shutdown:
			b.clientsMux.Lock()
			for _, client := range b.clients {
				client.Close()
			}
			b.clients = make(map[string]*Client)
			b.clientsMux.Unlock()
			return
		}
	}
}

func (b *Broker) AddClient(c *Client) {
	b.addClient <- c
}

func (b *Broker) RemoveClient(id string) {
	b.removeClient <- id
}

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

func (b *Broker) BroadcastConnected(data any) {
	msg := b.formatSSEMessage(EventTypeConnected, data)
	if msg == "" {
		return
	}
	b.broadcast <- msg
}

func (b *Broker) broadcastEvent(event TorrentEvent) {
	if b.GetClientCount() == 0 {
		return
	}
	msg := b.formatSSEMessage(event.Type, event)
	if msg == "" {
		return
	}
	b.broadcast <- msg
}

func (b *Broker) formatSSEMessage(eventType EventType, data any) string {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return ""
	}
	return fmt.Sprintf("event: %s\ndata: %s\n\n", eventType, string(jsonData))
}

func (b *Broker) Shutdown() {
	close(b.shutdown)
}

func (b *Broker) GetClientCount() int {
	b.clientsMux.RLock()
	defer b.clientsMux.RUnlock()
	return len(b.clients)
}

func StartTorrentMonitor(ctx context.Context, svc Service, broker *Broker, interval time.Duration) {
	if interval <= 0 {
		interval = 2 * time.Second
	}

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
