package sse_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/Cristian0711/media-bridge/backend/internal/sse"
)

func TestNewPublisherNilBrokerReturnsNoop(t *testing.T) {
	t.Parallel()
	if _, ok := sse.NewPublisher(nil).(sse.NoopPublisher); !ok {
		t.Fatal("NewPublisher(nil) should return a NoopPublisher")
	}
	// NoopPublisher methods must be safe no-ops (nil payloads included).
	p := sse.NoopPublisher{}
	p.PublishMediaAdded(context.Background(), nil)
	p.PublishMediaRemoved(context.Background(), nil)
	p.PublishRequestCreated(context.Background(), nil)
	p.PublishRequestStatusChanged(context.Background(), nil)
}

func TestFormatConnected(t *testing.T) {
	t.Parallel()
	frame := sse.FormatConnected("client-42")
	if !strings.HasPrefix(frame, "event: connected\n") {
		t.Fatalf("missing connected event line: %q", frame)
	}

	// The data line carries a JSON ConnectedPayload with the client id.
	_, data, ok := strings.Cut(frame, "data: ")
	if !ok {
		t.Fatalf("no data line: %q", frame)
	}
	var payload sse.ConnectedPayload
	if err := json.Unmarshal([]byte(strings.TrimSpace(data)), &payload); err != nil {
		t.Fatalf("data not valid JSON: %v (%q)", err, data)
	}
	if payload.ClientID != "client-42" {
		t.Errorf("ClientID = %q, want client-42", payload.ClientID)
	}
	if payload.Type != sse.EventConnected {
		t.Errorf("Type = %q, want %q", payload.Type, sse.EventConnected)
	}
}

func waitForClients(t *testing.T, b *sse.Broker, want int) {
	t.Helper()
	deadline := time.After(time.Second)
	for {
		if b.ClientCount() == want {
			return
		}
		select {
		case <-deadline:
			t.Fatalf("broker never reached %d clients (have %d)", want, b.ClientCount())
		case <-time.After(time.Millisecond):
		}
	}
}

func TestBrokerPublishFansOutEncodedEvent(t *testing.T) {
	t.Parallel()
	broker := sse.NewBroker()
	defer broker.Shutdown()

	client := sse.NewClient("c1")
	broker.AddClient(client)
	waitForClients(t, broker, 1)

	pub := sse.NewPublisher(broker)
	pub.PublishMediaAdded(context.Background(), &sse.MediaPayload{ID: 7, Name: "Movie", Type: "movie"})

	select {
	case frame, ok := <-client.Messages:
		if !ok {
			t.Fatal("client channel closed unexpectedly")
		}
		if !strings.Contains(frame, "event: media.added") {
			t.Errorf("frame missing event type: %q", frame)
		}
		var env sse.Envelope
		_, data, _ := strings.Cut(frame, "data: ")
		if err := json.Unmarshal([]byte(strings.TrimSpace(data)), &env); err != nil {
			t.Fatalf("envelope not valid JSON: %v", err)
		}
		if env.Media == nil || env.Media.ID != 7 {
			t.Errorf("envelope media = %+v, want ID 7", env.Media)
		}
	case <-time.After(time.Second):
		t.Fatal("no frame delivered to subscriber")
	}
}

func TestBrokerPublishNilPayloadIsNoop(t *testing.T) {
	t.Parallel()
	broker := sse.NewBroker()
	defer broker.Shutdown()

	client := sse.NewClient("c1")
	broker.AddClient(client)
	waitForClients(t, broker, 1)

	pub := sse.NewPublisher(broker)
	pub.PublishMediaAdded(context.Background(), nil) // nil → must not publish

	select {
	case frame := <-client.Messages:
		t.Fatalf("nil payload should not publish, got frame %q", frame)
	case <-time.After(100 * time.Millisecond):
		// expected: nothing delivered
	}
}
