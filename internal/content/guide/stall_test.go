package guide

import (
	"context"
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

// A download that stalls half way through the archive is cut and retried,
// and the retry starts over in an emptied temp dir: the files the first
// attempt extracted would otherwise collide (writeFile is O_EXCL).
func TestSnapshotStallRetries(t *testing.T) {
	var got atomic.Int64
	Progress = func(n int64) { got.Store(n) }
	t.Cleanup(func() { Progress = nil })
	srv, hits := stallServer(t, tarball(t, good), 1)
	start := time.Now()
	if _, err := snapshot(context.Background(), fast(srv), t.TempDir(), srv.URL, testCommit); err != nil {
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
