package qbittorrent_test

import (
	"context"
	"strings"
	"testing"
	"time"

	qbit "github.com/Cristian0711/media-bridge/backend/internal/qbittorrent"
)

func TestBrokerBroadcastAndRemoveClient(t *testing.T) {
	t.Parallel()

	b := qbit.NewBroker()
	client := qbit.NewClient("c1")
	b.AddClient(client)
	defer b.Shutdown()

	b.BroadcastTorrentAdded(qbit.Torrent{Hash: "h1", Name: "name"})

	select {
	case msg := <-client.Messages:
		if !strings.Contains(msg, "event: torrent_added") {
			t.Fatalf("expected torrent_added event, got %q", msg)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("expected broadcast message")
	}

	b.RemoveClient("c1")
}

func TestStartTorrentMonitorBroadcastsUpdated(t *testing.T) {
	t.Parallel()

	b := qbit.NewBroker()
	client := qbit.NewClient("c1")
	b.AddClient(client)
	defer b.Shutdown()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	call := 0
	svc := &qbitServiceStub{
		listFn: func(context.Context) ([]qbit.Torrent, error) {
			call++
			if call < 2 {
				return []qbit.Torrent{{Hash: "h1", State: "downloading", Progress: 0.2, DlSpeed: 10}}, nil
			}
			return []qbit.Torrent{{Hash: "h1", State: "downloading", Progress: 0.3, DlSpeed: 20}}, nil
		},
	}

	qbit.StartTorrentMonitor(ctx, svc, b, 10*time.Millisecond)

	select {
	case msg := <-client.Messages:
		if !strings.Contains(msg, "event: torrent_updated") {
			t.Fatalf("expected torrent_updated event, got %q", msg)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("expected update broadcast from monitor")
	}
}
