package sse

import "github.com/Cristian0711/media-bridge/backend/shared/ssehub"

// Client is one browser tab (or consumer) subscribed to the app event stream.
type Client = ssehub.Client

// NewClient allocates a buffered channel for outbound SSE frames.
func NewClient(id string) *Client { return ssehub.NewClient(id) }

// Broker fans out encoded app domain events (media + requests) to all connected
// clients. It is a thin wrapper over the shared ssehub.Hub.
type Broker struct {
	hub *ssehub.Hub
}

// NewBroker starts the broker goroutine. Call Shutdown on process exit.
func NewBroker() *Broker {
	return &Broker{hub: ssehub.New("sse.broker")}
}

// AddClient registers a subscriber.
func (b *Broker) AddClient(c *Client) { b.hub.AddClient(c) }

// RemoveClient disconnects a subscriber by ID.
func (b *Broker) RemoveClient(id string) { b.hub.RemoveClient(id) }

// ClientCount returns the number of active SSE connections.
func (b *Broker) ClientCount() int { return b.hub.ClientCount() }

// Shutdown closes all clients and stops the broker loop.
func (b *Broker) Shutdown() { b.hub.Shutdown() }

func (b *Broker) publish(frame string) { b.hub.Publish(frame) }

func formatMessage(eventType EventType, data any) string {
	return ssehub.Format(string(eventType), data)
}
