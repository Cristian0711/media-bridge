package hardlink

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Cristian0711/media-bridge/backend/internal/media"
)

// stubService is a controllable inner Service for exercising the progress cache.
type stubService struct {
	progressCalls int
	hardlinkCalls int
	progress      *Progress
	err           error
}

func (s *stubService) Hardlink(context.Context, uint) error {
	s.hardlinkCalls++
	return s.err
}

func (s *stubService) IsComplete(ctx context.Context, mediaID uint) (bool, error) {
	p, err := s.Progress(ctx, mediaID)
	if err != nil {
		return false, err
	}
	return p.Complete, nil
}

func (s *stubService) Progress(context.Context, uint) (*Progress, error) {
	s.progressCalls++
	if s.err != nil {
		return nil, s.err
	}
	return s.progress, nil
}

func (s *stubService) ProgressForMedia(context.Context, *media.Media) (*Progress, error) {
	s.progressCalls++
	if s.err != nil {
		return nil, s.err
	}
	return s.progress, nil
}

func TestProgressCacheServesFromCacheWithinTTL(t *testing.T) {
	inner := &stubService{progress: &Progress{Total: 2, Linked: 2, Complete: true}}
	c := withProgressCache(inner, time.Minute)

	for i := 0; i < 3; i++ {
		if _, err := c.Progress(context.Background(), 1); err != nil {
			t.Fatalf("Progress() error = %v", err)
		}
	}
	if inner.progressCalls != 1 {
		t.Fatalf("inner Progress called %d times, want 1 (rest served from cache)", inner.progressCalls)
	}
}

func TestProgressCacheReturnsIndependentClones(t *testing.T) {
	inner := &stubService{progress: &Progress{Total: 1, Done: []Artifact{{Name: "a"}}}}
	c := withProgressCache(inner, time.Minute)

	first, err := c.Progress(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	first.Done = append(first.Done, Artifact{Name: "mutation"})

	second, err := c.Progress(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(second.Done) != 1 {
		t.Fatalf("cache handed out a shared slice; second.Done len = %d, want 1", len(second.Done))
	}
}

func TestProgressCacheExpiresAfterTTL(t *testing.T) {
	inner := &stubService{progress: &Progress{Total: 1}}
	c := withProgressCache(inner, time.Nanosecond) // effectively already expired

	_, _ = c.Progress(context.Background(), 1)
	time.Sleep(time.Millisecond)
	_, _ = c.Progress(context.Background(), 1)

	if inner.progressCalls != 2 {
		t.Fatalf("inner Progress called %d times, want 2 (cache should have expired)", inner.progressCalls)
	}
}

func TestProgressCacheInvalidatedByHardlink(t *testing.T) {
	inner := &stubService{progress: &Progress{Total: 1}}
	c := withProgressCache(inner, time.Minute)

	if _, err := c.Progress(context.Background(), 1); err != nil {
		t.Fatal(err)
	}
	if err := c.Hardlink(context.Background(), 1); err != nil {
		t.Fatal(err)
	}
	// After Hardlink invalidates the entry, the next Progress must hit inner again.
	if _, err := c.Progress(context.Background(), 1); err != nil {
		t.Fatal(err)
	}
	if inner.progressCalls != 2 {
		t.Fatalf("inner Progress called %d times, want 2 (Hardlink should invalidate)", inner.progressCalls)
	}
}

func TestProgressCacheDoesNotCacheErrors(t *testing.T) {
	inner := &stubService{err: errors.New("boom")}
	c := withProgressCache(inner, time.Minute)

	if _, err := c.Progress(context.Background(), 1); err == nil {
		t.Fatal("expected error from inner")
	}
	if _, err := c.Progress(context.Background(), 1); err == nil {
		t.Fatal("expected error from inner on second call too")
	}
	if inner.progressCalls != 2 {
		t.Fatalf("errors must not be cached; inner called %d times, want 2", inner.progressCalls)
	}
}

func TestProgressForMediaNilRowReturnsEmpty(t *testing.T) {
	inner := &stubService{progress: &Progress{Total: 5}}
	c := withProgressCache(inner, time.Minute)

	p, err := c.ProgressForMedia(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if p.Total != 0 {
		t.Fatalf("nil row should yield empty Progress, got Total=%d", p.Total)
	}
	if inner.progressCalls != 0 {
		t.Fatalf("nil row must short-circuit before inner; inner called %d times", inner.progressCalls)
	}
}
