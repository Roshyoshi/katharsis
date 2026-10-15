package codeforces

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// The client must never have two requests in flight to one host, must
// leave the gap between them, and must say who it is (and nothing else).
func TestPoliteClientSerializes(t *testing.T) {
	t.Parallel()
	var inFlight, peak atomic.Int32
	var mu sync.Mutex
	var starts, ends []time.Time
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.UserAgent() != userAgent {
			t.Errorf("User-Agent = %q", r.UserAgent())
		}
		for k := range r.Header {
			if k != "User-Agent" && k != "Accept-Encoding" && k != "Range" {
				t.Errorf("unexpected header %s", k)
			}
		}
		n := inFlight.Add(1)
		defer inFlight.Add(-1)
		for {
			p := peak.Load()
			if n <= p || peak.CompareAndSwap(p, n) {
				break
			}
		}
		mu.Lock()
		starts = append(starts, time.Now())
		mu.Unlock()
		time.Sleep(5 * time.Millisecond)
		_, _ = w.Write([]byte("ok"))
		mu.Lock()
		ends = append(ends, time.Now())
		mu.Unlock()
	}))
	defer srv.Close()
	gap := 30 * time.Millisecond
	c := &politeClient{http: srv.Client(), gap: gap, retries: 1, backoff: time.Millisecond}
	var wg sync.WaitGroup
	for range 6 {
		wg.Go(func() {
			if _, _, err := c.getBytes(context.Background(), srv.URL+"/x", 10); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	if p := peak.Load(); p != 1 {
		t.Errorf("peak in-flight = %d, want 1", p)
	}
	slices.SortFunc(starts, func(a, b time.Time) int { return a.Compare(b) })
	slices.SortFunc(ends, func(a, b time.Time) int { return a.Compare(b) })
	for i := 1; i < len(starts); i++ {
		if d := starts[i].Sub(ends[i-1]); d < gap-5*time.Millisecond {
			t.Errorf("request %d started %v after the previous ended, want >= %v", i, d, gap)
		}
	}
}

func TestPoliteClientRetries(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		statuses []int // per attempt; the last repeats
		retries  int
		wantHits int
		wantErr  string // "" for success
		notFound bool
	}{
		{"5xx then ok", []int{502, 503, 200}, 4, 3, "", false},
		{"429 then ok", []int{429, 200}, 4, 2, "", false},
		{"gives up", []int{500}, 2, 3, "500", false},
		{"404 is final", []int{404}, 4, 1, "not found", true},
		{"403 is final", []int{403}, 4, 1, "403", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var hits atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				i := int(hits.Add(1)) - 1
				st := tc.statuses[min(i, len(tc.statuses)-1)]
				if st == 429 {
					w.Header().Set("Retry-After", "0")
				}
				w.WriteHeader(st)
				_, _ = w.Write([]byte("body"))
			}))
			defer srv.Close()
			c := &politeClient{http: srv.Client(), gap: time.Millisecond, retries: tc.retries, backoff: time.Millisecond}
			b, _, err := c.getBytes(context.Background(), srv.URL, 100)
			if got := int(hits.Load()); got != tc.wantHits {
				t.Errorf("hits = %d, want %d", got, tc.wantHits)
			}
			if tc.wantErr == "" {
				if err != nil || string(b) != "body" {
					t.Errorf("getBytes = %q, %v", b, err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("err = %v, want %q", err, tc.wantErr)
			}
			if errors.Is(err, errNotFound) != tc.notFound {
				t.Errorf("errors.Is(err, errNotFound) = %v", !tc.notFound)
			}
		})
	}
}

func TestPoliteClientRanges(t *testing.T) {
	t.Parallel()
	body := []byte("0123456789abcdefghij")
	var ignoreRange atomic.Bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if ignoreRange.Load() {
			_, _ = w.Write(body) // 200 with everything
			return
		}
		http.ServeContent(w, r, "f", time.Time{}, bytes.NewReader(body))
	}))
	defer srv.Close()
	c := &politeClient{http: srv.Client(), gap: time.Millisecond, retries: 2, backoff: time.Millisecond}
	ctx := context.Background()

	if b, err := c.getRange(ctx, srv.URL, 3, 4); err != nil || string(b) != "3456" {
		t.Errorf("getRange = %q, %v", b, err)
	}
	if b, size, err := c.getTail(ctx, srv.URL, 5); err != nil || string(b) != "fghij" || size != 20 {
		t.Errorf("getTail = %q, %d, %v", b, size, err)
	}
	// A tail longer than the file is the whole file.
	if b, size, err := c.getTail(ctx, srv.URL, 100); err != nil || !bytes.Equal(b, body) || size != 20 {
		t.Errorf("long getTail = %q, %d, %v", b, size, err)
	}
	if _, err := c.getRange(ctx, srv.URL, 30, 4); err == nil {
		t.Error("range past the end succeeded")
	}
	var buf bytes.Buffer
	reset := func() (io.Writer, error) { buf.Reset(); return &buf, nil }
	if err := c.download(ctx, srv.URL, 20, reset); err != nil || !bytes.Equal(buf.Bytes(), body) {
		t.Errorf("download = %q, %v", buf.Bytes(), err)
	}
	if err := c.download(ctx, srv.URL, 19, reset); err == nil {
		t.Error("download over the cap succeeded")
	}

	// A server that ignores Range would send the whole file; that must
	// fail at once, not be read or retried.
	ignoreRange.Store(true)
	if _, err := c.getRange(ctx, srv.URL, 3, 4); err == nil || !errors.As(err, new(permanentError)) {
		t.Errorf("200 to a range request: %v", err)
	}
}

func TestContentRange(t *testing.T) {
	t.Parallel()
	tests := []struct {
		in                string
		start, end, total int64
		err               bool
	}{
		{"bytes 0-9/100", 0, 9, 100, false},
		{"bytes 90-99/100", 90, 99, 100, false},
		{"bytes 5-9/*", 5, 9, -1, false},
		{"bytes */100", 0, 0, 0, true},
		{"items 0-9/100", 0, 0, 0, true},
		{"", 0, 0, 0, true},
	}
	for _, tc := range tests {
		s, e, n, err := contentRange(tc.in)
		if (err != nil) != tc.err || !tc.err && (s != tc.start || e != tc.end || n != tc.total) {
			t.Errorf("contentRange(%q) = %d %d %d %v", tc.in, s, e, n, err)
		}
	}
}

func TestNextLink(t *testing.T) {
	t.Parallel()
	h := http.Header{}
	h.Add("Link", `<https://huggingface.co/api/datasets/x/tree/main?cursor=abc>; rel="next"`)
	if got := nextLink(h); got != "https://huggingface.co/api/datasets/x/tree/main?cursor=abc" {
		t.Errorf("nextLink = %q", got)
	}
	if got := nextLink(http.Header{}); got != "" {
		t.Errorf("nextLink of nothing = %q", got)
	}
}
