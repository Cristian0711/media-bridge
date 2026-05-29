package sse

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/Cristian0711/media-bridge/backend/shared/logger"
	"go.uber.org/zap"
)

// Client is one browser tab (or consumer) subscribed to the app event stream.
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
		Messages: make(chan string, 128),
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

// Broker fans out encoded SSE messages to all connected clients.
// Pattern matches qbittorrent/sse.go but carries app domain events only.
type Broker struct {
	clients    map[string]*Client
	clientsMux sync.RWMutex

	addClient    chan *Client
	removeClient chan string
	broadcast    chan string
	shutdown     chan struct{}
}

// NewBroker starts the broker goroutine. Call Shutdown on process exit if needed.
func NewBroker() *Broker {
	b := &Broker{
		clients:      make(map[string]*Client),
		addClient:    make(chan *Client),
		removeClient: make(chan string),
		broadcast:    make(chan string, 256),
		shutdown:     make(chan struct{}),
	}
	go b.run()
	return b
}

func (b *Broker) run() {
	log := logger.Named("sse.broker")
	heartbeat := time.NewTicker(30 * time.Second)
	defer heartbeat.Stop()

	for {
		select {
		case client := <-b.addClient:
			b.clientsMux.Lock()
			b.clients[client.ID] = client
			b.clientsMux.Unlock()
			log.Info("sse client connected",
				zap.String("client_id", client.ID),
				zap.Int("clients", len(b.clients)),
			)

		case clientID := <-b.removeClient:
			b.clientsMux.Lock()
			if client, ok := b.clients[clientID]; ok {
				client.Close()
				delete(b.clients, clientID)
				log.Info("sse client disconnected",
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
					log.Warn("sse client queue full, dropping frame",
						zap.String("client_id", client.ID),
					)
				}
			}
			b.clientsMux.RUnlock()

		case <-heartbeat.C:
			msg := formatMessage(EventHeartbeat, map[string]any{"timestamp": unixNow()})
			if msg == "" {
				continue
			}
			b.clientsMux.RLock()
			for _, client := range b.clients {
				select {
				case client.Messages <- msg:
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

// AddClient registers a subscriber (non-blocking).
func (b *Broker) AddClient(c *Client) {
	b.addClient <- c
}

// RemoveClient disconnects a subscriber by ID.
func (b *Broker) RemoveClient(id string) {
	b.removeClient <- id
}

// ClientCount returns the number of active SSE connections.
func (b *Broker) ClientCount() int {
	b.clientsMux.RLock()
	defer b.clientsMux.RUnlock()
	return len(b.clients)
}

// Shutdown closes all clients and stops the broker loop.
func (b *Broker) Shutdown() {
	close(b.shutdown)
}

func (b *Broker) publish(frame string) {
	if frame == "" || b.ClientCount() == 0 {
		return
	}
	select {
	case b.broadcast <- frame:
	default:
		logger.Named("sse.broker").Warn("broadcast channel full, dropping frame")
	}
}

func formatMessage(eventType EventType, data any) string {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return ""
	}
	return fmt.Sprintf("event: %s\ndata: %s\n\n", eventType, string(jsonData))
}
