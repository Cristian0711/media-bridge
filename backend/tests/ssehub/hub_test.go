package ssehub_test

import (
	"strings"
	"testing"
	"time"

	"github.com/Cristian0711/media-bridge/backend/shared/ssehub"
)

// waitForClients blocks until the hub reports want subscribers (registration is
// processed asynchronously by the hub goroutine).
func waitForClients(t *testing.T, hub *ssehub.Hub, want int) {
	t.Helper()
	deadline := time.After(time.Second)
	for {
		if hub.ClientCount() == want {
			return
		}
		select {
		case <-deadline:
			t.Fatalf("hub never reached %d clients (have %d)", want, hub.ClientCount())
		case <-time.After(time.Millisecond):
		}
	}
}

func recvWithin(t *testing.T, c *ssehub.Client, d time.Duration) (string, bool) {
	t.Helper()
	select {
	case msg, ok := <-c.Messages:
		return msg, ok
	case <-time.After(d):
		t.Fatal("timed out waiting for frame")
		return "", false
	}
}

func TestFormatBuildsSSEFrame(t *testing.T) {
	t.Parallel()
	got := ssehub.Format("media.added", map[string]any{"id": 7})
	if !strings.HasPrefix(got, "event: media.added\n") {
		t.Fatalf("missing event line: %q", got)
	}
	if !strings.HasSuffix(got, "\n\n") {
		t.Fatalf("frame must end with a blank line: %q", got)
	}
	if !strings.Contains(got, `data: {"id":7}`) {
		t.Fatalf("missing data line: %q", got)
	}
}

func TestFormatUnmarshalableReturnsEmpty(t *testing.T) {
	t.Parallel()
	if got := ssehub.Format("x", make(chan int)); got != "" {
		t.Fatalf("expected empty frame for unmarshalable payload, got %q", got)
	}
}

func TestHubFanOutDeliversToAllClients(t *testing.T) {
	t.Parallel()
	hub := ssehub.New("test.fanout")
	defer hub.Shutdown()

	a := ssehub.NewClient("a")
	b := ssehub.NewClient("b")
	hub.AddClient(a)
	hub.AddClient(b)
	waitForClients(t, hub, 2)

	hub.Publish(ssehub.Format("ping", map[string]any{"n": 1}))

	for _, c := range []*ssehub.Client{a, b} {
		msg, ok := recvWithin(t, c, 500*time.Millisecond)
		if !ok || !strings.Contains(msg, "event: ping") {
			t.Fatalf("client %s got %q (ok=%v)", c.ID, msg, ok)
		}
	}
}

func TestPublishWithoutSubscribersIsNoop(t *testing.T) {
	t.Parallel()
	hub := ssehub.New("test.empty")
	defer hub.Shutdown()
	// Must not block or panic with zero subscribers.
	hub.Publish(ssehub.Format("ping", map[string]any{"n": 1}))
}

func TestHubDropsOnFullClientBufferAndStaysResponsive(t *testing.T) {
	t.Parallel()
	hub := ssehub.New("test.drop")
	defer hub.Shutdown()

	c := ssehub.NewClient("slow")
	hub.AddClient(c)
	waitForClients(t, hub, 1)

	// Flood far beyond the per-client buffer with no reader. Every Publish must
	// return promptly (drop-on-full), never blocking the caller.
	done := make(chan struct{})
	go func() {
		for i := 0; i < 5000; i++ {
			hub.Publish(ssehub.Format("e", map[string]any{"i": i}))
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Publish blocked under a full client buffer (drop-on-full broken)")
	}

	// Hub stays responsive to control ops.
	if hub.ClientCount() != 1 {
		t.Fatalf("client count = %d, want 1", hub.ClientCount())
	}

	// Some frames were delivered, but far fewer than published (the rest dropped).
	delivered := 0
	for {
		select {
		case _, ok := <-c.Messages:
			if !ok {
				goto counted
			}
			delivered++
		default:
			goto counted
		}
	}
counted:
	if delivered == 0 {
		t.Fatal("expected at least one delivered frame")
	}
	if delivered >= 5000 {
		t.Fatalf("expected frames to be dropped, delivered all %d", delivered)
	}
}

func TestShutdownClosesClients(t *testing.T) {
	t.Parallel()
	hub := ssehub.New("test.shutdown")
	c := ssehub.NewClient("c")
	hub.AddClient(c)
	waitForClients(t, hub, 1)

	hub.Shutdown()
	hub.Shutdown() // idempotent

	select {
	case _, ok := <-c.Messages:
		if ok {
			t.Fatal("expected Messages channel closed after shutdown")
		}
	case <-time.After(time.Second):
		t.Fatal("Messages channel not closed after shutdown")
	}

	select {
	case <-c.Done():
	case <-time.After(time.Second):
		t.Fatal("Done channel not closed after shutdown")
	}
}

func TestRemoveClientStopsDelivery(t *testing.T) {
	t.Parallel()
	hub := ssehub.New("test.remove")
	defer hub.Shutdown()

	c := ssehub.NewClient("c")
	hub.AddClient(c)
	waitForClients(t, hub, 1)

	hub.RemoveClient("c")
	waitForClients(t, hub, 0)

	// Channel is closed on removal.
	select {
	case _, ok := <-c.Messages:
		if ok {
			t.Fatal("expected closed channel after RemoveClient")
		}
	case <-time.After(time.Second):
		t.Fatal("channel not closed after RemoveClient")
	}
}
