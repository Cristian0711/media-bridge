package queueutil_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/Cristian0711/media-bridge/backend/shared/queueutil"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestMarkRequestNoopWhenMarkNil(t *testing.T) {
	t.Parallel()
	// Must not panic when mark is nil.
	queueutil.MarkRequest(context.Background(), discardLogger(), 1, "download", nil)
}

func TestMarkRequestNoopWhenIDZero(t *testing.T) {
	t.Parallel()
	called := false
	mark := func(context.Context, uint) (bool, error) {
		called = true
		return true, nil
	}
	queueutil.MarkRequest(context.Background(), discardLogger(), 0, "download", mark)
	if called {
		t.Fatal("mark must not be called when requestEntryID is 0")
	}
}

func TestMarkRequestInvokesMark(t *testing.T) {
	t.Parallel()
	var gotID uint
	mark := func(_ context.Context, id uint) (bool, error) {
		gotID = id
		return true, nil
	}
	queueutil.MarkRequest(context.Background(), discardLogger(), 42, "download request done", mark)
	if gotID != 42 {
		t.Fatalf("mark received id %d, want 42", gotID)
	}
}

func TestMarkRequestSwallowsError(t *testing.T) {
	t.Parallel()
	mark := func(context.Context, uint) (bool, error) {
		return false, errors.New("db down")
	}
	// MarkRequest logs and returns; it must not panic or propagate the error.
	queueutil.MarkRequest(context.Background(), discardLogger(), 5, "download failed", mark)
}
