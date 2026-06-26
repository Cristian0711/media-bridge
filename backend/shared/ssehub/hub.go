// Package ssehub is a small, transport-agnostic Server-Sent Events fan-out hub.
//
// It owns the connected-client registry, a single broadcast goroutine, periodic
// heartbeats, and graceful shutdown. It deals only in pre-formatted SSE frames
// (strings), so domain packages keep their own event types and payload shapes
// and simply hand finished frames to Publish. Both internal/sse (app events) and
// internal/qbittorrent (torrent events) wrap one Hub.
//
// Delivery is non-blocking: a frame is dropped for any client whose outbound
// buffer is full rather than stalling the whole fan-out (drop-on-full).
package ssehub

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/Cristian0711/media-bridge/backend/shared/logger"
	"go.uber.org/zap"
)

const (
	// clientBuffer is the per-client outbound queue depth. Frames beyond this
	// are dropped for that client (a slow/stalled consumer never blocks others).
	clientBuffer = 128
	// broadcastBuffer is the hub-wide pending-frame queue depth.
	broadcastBuffer   = 256
	heartbeatInterval = 30 * time.Second
)

// Client is one subscriber (typically a browser tab). The HTTP handler reads
// frames from Messages until it is closed.
type Client struct {
	ID       string
	Messages chan string
	done     chan struct{}
	once     sync.Once
}

// NewClient allocates a buffered channel for outbound SSE frames.
func NewClient(id string) *Client {
	return &Client{
		ID:       id,
		Messages: make(chan string, clientBuffer),
		done:     make(chan struct{}),
	}
}

// Close shuts down the client; safe to call more than once.
func (c *Client) Close() {
	c.once.Do(func() {
		close(c.done)
		close(c.Messages)
	})
}

// Done is closed when the client is removed or the hub shuts down.
func (c *Client) Done() <-chan struct{} { return c.done }

// Hub fans out encoded SSE frames to all connected clients from one goroutine.
type Hub struct {
	name string

	clients    map[string]*Client
	clientsMux sync.RWMutex

	addClient    chan *Client
	removeClient chan string
	broadcast    chan string
	shutdown     chan struct{}
	shutdownOnce sync.Once
}

// New starts the hub goroutine. name is used as the logger scope. Call Shutdown
// on process exit.
func New(name string) *Hub {
	h := &Hub{
		name:         name,
		clients:      make(map[string]*Client),
		addClient:    make(chan *Client),
		removeClient: make(chan string),
		broadcast:    make(chan string, broadcastBuffer),
		shutdown:     make(chan struct{}),
	}
	go h.run()
	return h
}

func (h *Hub) run() {
	log := logger.Named(h.name)
	heartbeat := time.NewTicker(heartbeatInterval)
	defer heartbeat.Stop()

	for {
		select {
		case client := <-h.addClient:
			h.clientsMux.Lock()
			h.clients[client.ID] = client
			n := len(h.clients)
			h.clientsMux.Unlock()
			log.Info("sse client connected", zap.String("client_id", client.ID), zap.Int("clients", n))

		case clientID := <-h.removeClient:
			h.clientsMux.Lock()
			client, ok := h.clients[clientID]
			if ok {
				client.Close()
				delete(h.clients, clientID)
			}
			n := len(h.clients)
			h.clientsMux.Unlock()
			if ok {
				log.Info("sse client disconnected", zap.String("client_id", clientID), zap.Int("clients", n))
			}

		case message := <-h.broadcast:
			h.fanOut(log, message, true)

		case <-heartbeat.C:
			h.fanOut(log, heartbeatFrame(), false)

		case <-h.shutdown:
			h.clientsMux.Lock()
			for _, client := range h.clients {
				client.Close()
			}
			h.clients = make(map[string]*Client)
			h.clientsMux.Unlock()
			return
		}
	}
}

// fanOut delivers message to every client without blocking. warnOnFull logs a
// dropped frame (suppressed for heartbeats, which are intentionally lossy).
func (h *Hub) fanOut(log *zap.Logger, message string, warnOnFull bool) {
	if message == "" {
		return
	}
	h.clientsMux.RLock()
	defer h.clientsMux.RUnlock()
	for _, client := range h.clients {
		select {
		case client.Messages <- message:
		default:
			if warnOnFull {
				log.Warn("sse client queue full, dropping frame", zap.String("client_id", client.ID))
			}
		}
	}
}

// AddClient registers a subscriber. If the hub has already shut down it closes
// the client instead of blocking forever on the unbuffered channel (the run
// loop is gone and would never receive).
func (h *Hub) AddClient(c *Client) {
	select {
	case h.addClient <- c:
	case <-h.shutdown:
		c.Close()
	}
}

// RemoveClient disconnects a subscriber by ID. It no-ops once the hub has shut
// down (the run loop already closed every client), rather than blocking the
// caller's HTTP goroutine forever on the unbuffered channel.
func (h *Hub) RemoveClient(id string) {
	select {
	case h.removeClient <- id:
	case <-h.shutdown:
	}
}

// ClientCount returns the number of active subscribers.
func (h *Hub) ClientCount() int {
	h.clientsMux.RLock()
	defer h.clientsMux.RUnlock()
	return len(h.clients)
}

// Publish hands a pre-formatted SSE frame to the fan-out goroutine. It is
// non-blocking: frames are dropped when there are no subscribers or the hub
// queue is full.
func (h *Hub) Publish(frame string) {
	if frame == "" || h.ClientCount() == 0 {
		return
	}
	select {
	case h.broadcast <- frame:
	default:
		logger.Named(h.name).Warn("broadcast channel full, dropping frame")
	}
}

// Shutdown closes all clients and stops the hub loop. Safe to call more than once.
func (h *Hub) Shutdown() {
	h.shutdownOnce.Do(func() {
		close(h.shutdown)
	})
}

func heartbeatFrame() string {
	return Format("heartbeat", map[string]any{"timestamp": time.Now().UTC().Unix()})
}

// Format builds an SSE frame ("event: <event>\ndata: <json>\n\n"). It returns
// "" when the payload cannot be marshaled.
func Format(event string, data any) string {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return ""
	}
	return fmt.Sprintf("event: %s\ndata: %s\n\n", event, string(jsonData))
}
