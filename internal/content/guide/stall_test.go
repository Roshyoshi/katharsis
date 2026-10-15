package guide

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

// stallServer serves body, except that the first stalls requests send
// half of it and then hold the connection open with nothing more, as the
// stalled download in qa/chaos r07 did.
func stallServer(t *testing.T, body []byte, stalls int) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var hits atomic.Int32
	quit := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if int(hits.Add(1)) > stalls {
			_, _ = w.Write(body)
			return
		}
		_, _ = w.Write(body[:len(body)/2])
		w.(http.Flusher).Flush()
		select {
		case <-r.Context().Done():
		case <-quit:
		}
	}))
	t.Cleanup(func() { close(quit); srv.Close() })
	return srv, &hits
}

func shortTimers(t *testing.T) {
	idle, delay := idleTimeout, retryDelay
	idleTimeout, retryDelay = 100*time.Millisecond, time.Millisecond
	t.Cleanup(func() { idleTimeout, retryDelay = idle, delay })
}

// A download that stalls is cut after idleTimeout without bytes and
// retried; it used to hang until the caller's 10-minute load timeout.
func TestSnapshotStallRetries(t *testing.T) {
	shortTimers(t)
	var got atomic.Int64
	Progress = func(n int64) { got.Store(n) }
	t.Cleanup(func() { Progress = nil })
	srv, hits := stallServer(t, tarball(t, good), 1)
	start := time.Now()
	if _, err := snapshot(context.Background(), t.TempDir(), srv.URL, testCommit); err != nil {
		t.Fatal(err)
	}
	if d := time.Since(start); d > 5*time.Second {
		t.Errorf("took %v", d)
	}
	if n := hits.Load(); n != 2 {
		t.Errorf("requests = %d, want 2 (one stall, one retry)", n)
	}
	if got.Load() == 0 {
		t.Error("Progress never heard of a byte")
	}
}

// A download that stalls on every try fails, saying so, once the retries
// are spent.
func TestSnapshotStallGivesUp(t *testing.T) {
	shortTimers(t)
	srv, hits := stallServer(t, tarball(t, good), 100)
	_, err := snapshot(context.Background(), t.TempDir(), srv.URL, testCommit)
	if !errors.Is(err, errStalled) {
		t.Fatalf("snapshot = %v, want errStalled", err)
	}
	if n := hits.Load(); n != 3 {
		t.Errorf("requests = %d, want 3", n)
	}
}
