package sse

import "context"

// Publisher emits real-time events to connected clients.
// All methods are safe to call from queue workers; publishing is non-blocking.
type Publisher interface {
	PublishMediaAdded(ctx context.Context, payload *MediaPayload)
	PublishMediaRemoved(ctx context.Context, payload *MediaPayload)
	PublishRequestCreated(ctx context.Context, payload *RequestPayload)
	PublishRequestStatusChanged(ctx context.Context, payload *RequestPayload)
}

// brokerPublisher encodes domain events and hands them to the broker.
type brokerPublisher struct {
	broker *Broker
}

// NewPublisher returns a Publisher backed by the shared SSE broker.
func NewPublisher(broker *Broker) Publisher {
	if broker == nil {
		return NoopPublisher{}
	}
	return brokerPublisher{broker: broker}
}

// NoopPublisher discards events (tests, disabled SSE).
type NoopPublisher struct{}

func (NoopPublisher) PublishMediaAdded(context.Context, *MediaPayload)       {}
func (NoopPublisher) PublishMediaRemoved(context.Context, *MediaPayload)     {}
func (NoopPublisher) PublishRequestCreated(context.Context, *RequestPayload) {}
func (NoopPublisher) PublishRequestStatusChanged(context.Context, *RequestPayload) {
}

func (p brokerPublisher) PublishMediaAdded(_ context.Context, payload *MediaPayload) {
	if payload == nil {
		return
	}
	p.broker.publish(formatMessage(EventMediaAdded, Envelope{
		Type:      EventMediaAdded,
		Timestamp: unixNow(),
		Media:     payload,
	}))
}

func (p brokerPublisher) PublishMediaRemoved(_ context.Context, payload *MediaPayload) {
	if payload == nil {
		return
	}
	p.broker.publish(formatMessage(EventMediaRemoved, Envelope{
		Type:      EventMediaRemoved,
		Timestamp: unixNow(),
		Media:     payload,
	}))
}

func (p brokerPublisher) PublishRequestCreated(_ context.Context, payload *RequestPayload) {
	if payload == nil {
		return
	}
	p.broker.publish(formatMessage(EventRequestCreated, Envelope{
		Type:      EventRequestCreated,
		Timestamp: unixNow(),
		Request:   payload,
	}))
}

func (p brokerPublisher) PublishRequestStatusChanged(_ context.Context, payload *RequestPayload) {
	if payload == nil {
		return
	}
	p.broker.publish(formatMessage(EventRequestStatusChanged, Envelope{
		Type:      EventRequestStatusChanged,
		Timestamp: unixNow(),
		Request:   payload,
	}))
}

// FormatConnected returns the initial SSE frame for a new subscriber.
func FormatConnected(clientID string) string {
	return formatMessage(EventConnected, ConnectedPayload{
		Type:      EventConnected,
		ClientID:  clientID,
		Timestamp: unixNow(),
	})
}
