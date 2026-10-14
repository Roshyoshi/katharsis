package usaco

import (
	"context"
	"errors"
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
// leave the gap between them, and must say who it is.
func TestPoliteClientSerializes(t *testing.T) {
	t.Parallel()
	var inFlight, peak atomic.Int32
	var mu sync.Mutex
	var starts, ends []time.Time
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.UserAgent(), "katharsis/") {
			t.Errorf("User-Agent = %q", r.UserAgent())
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
		w.Write([]byte("ok"))
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
			if _, err := c.getBytes(context.Background(), srv.URL+"/x", 10); err != nil {
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
		// The gap runs from the end of one request to the start of the
		// next; allow a little for the server's clock reads.
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
				w.Write([]byte("body"))
			}))
			defer srv.Close()
			c := &politeClient{http: srv.Client(), gap: time.Millisecond, retries: tc.retries, backoff: time.Millisecond}
			b, err := c.getBytes(context.Background(), srv.URL, 100)
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

func TestPoliteClientBodyCap(t *testing.T) {
	t.Parallel()
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Write([]byte(strings.Repeat("x", 101)))
	}))
	defer srv.Close()
	c := &politeClient{http: srv.Client(), gap: time.Millisecond, retries: 3, backoff: time.Millisecond}
	if _, err := c.getBytes(context.Background(), srv.URL, 100); err == nil {
		t.Error("oversized body accepted")
	}
	if n := hits.Load(); n != 1 {
		t.Errorf("an oversized body was retried: %d hits", n)
	}
}
