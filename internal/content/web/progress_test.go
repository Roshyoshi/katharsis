package web

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

// A context with WithProgress hears every body byte of every request made
// under it, whichever helper makes them.
func TestWithProgress(t *testing.T) {
	t.Parallel()
	body := bytes.Repeat([]byte("k"), 3<<14)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(body) }))
	t.Cleanup(srv.Close)
	c := fast(srv, 0)
	var got atomic.Int64
	ctx := WithProgress(context.Background(), func(n int64) { got.Add(n) })
	for range 2 {
		if _, _, err := c.Bytes(ctx, srv.URL, int64(len(body))); err != nil {
			t.Fatal(err)
		}
	}
	if want := int64(2 * len(body)); got.Load() != want {
		t.Errorf("progress heard %d bytes, want %d", got.Load(), want)
	}
	if _, _, err := c.Bytes(context.Background(), srv.URL, int64(len(body))); err != nil || got.Load() != int64(2*len(body)) {
		t.Errorf("a request without the context was heard too: %d, %v", got.Load(), err)
	}
}
