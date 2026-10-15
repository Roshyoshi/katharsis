package web

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func fast(srv *httptest.Server, retries int) *Client {
	return &Client{HTTP: srv.Client(), Gap: time.Millisecond, Retries: retries, Backoff: time.Millisecond}
}

// The client must never have two requests in flight to one host, must
// leave the gap between them, and must say who it is and nothing else.
func TestClientSerializes(t *testing.T) {
	t.Parallel()
	var inFlight, peak atomic.Int32
	var mu sync.Mutex
	var starts, ends []time.Time
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.UserAgent() != UserAgent {
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
	c := &Client{HTTP: srv.Client(), Gap: gap, Retries: 1, Backoff: time.Millisecond}

	var wg sync.WaitGroup
	for range 6 {
		wg.Go(func() {
			if _, _, err := c.Bytes(context.Background(), srv.URL+"/x", 10); err != nil {
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

func TestClientRetries(t *testing.T) {
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
			b, _, err := fast(srv, tc.retries).Bytes(context.Background(), srv.URL, 100)
			if got := int(hits.Load()); got != tc.wantHits {
				t.Errorf("hits = %d, want %d", got, tc.wantHits)
			}
			if tc.wantErr == "" {
				if err != nil || string(b) != "body" {
					t.Errorf("Bytes = %q, %v", b, err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("err = %v, want %q", err, tc.wantErr)
			}
			if errors.Is(err, ErrNotFound) != tc.notFound {
				t.Errorf("errors.Is(err, ErrNotFound) = %v", !tc.notFound)
			}
		})
	}
}

// A body over the cap is refused, not retried.
func TestClientBodyCap(t *testing.T) {
	t.Parallel()
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		_, _ = w.Write([]byte(strings.Repeat("x", 101)))
	}))
	defer srv.Close()
	if _, _, err := fast(srv, 3).Bytes(context.Background(), srv.URL, 100); err == nil {
		t.Error("oversized body accepted")
	}
	if n := hits.Load(); n != 1 {
		t.Errorf("an oversized body was retried: %d hits", n)
	}
}

func TestClientRanges(t *testing.T) {
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
	c := fast(srv, 2)
	ctx := context.Background()

	if b, err := c.Range(ctx, srv.URL, 3, 4); err != nil || string(b) != "3456" {
		t.Errorf("Range = %q, %v", b, err)
	}
	if b, size, err := c.Tail(ctx, srv.URL, 5); err != nil || string(b) != "fghij" || size != 20 {
		t.Errorf("Tail = %q, %d, %v", b, size, err)
	}
	// A tail longer than the file is the whole file.
	if b, size, err := c.Tail(ctx, srv.URL, 100); err != nil || !bytes.Equal(b, body) || size != 20 {
		t.Errorf("long Tail = %q, %d, %v", b, size, err)
	}
	if _, err := c.Range(ctx, srv.URL, 30, 4); err == nil {
		t.Error("range past the end succeeded")
	}
	f, err := os.Create(filepath.Join(t.TempDir(), "spool"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	if _, err := f.WriteString("stale bytes from an earlier attempt, longer than the body"); err != nil {
		t.Fatal(err)
	}
	if n, err := c.Spool(ctx, srv.URL, f, 20); err != nil || n != 20 {
		t.Errorf("Spool = %d, %v", n, err)
	}
	if got, _ := os.ReadFile(f.Name()); !bytes.Equal(got, body) {
		t.Errorf("spooled %q", got)
	}
	if _, err := c.Spool(ctx, srv.URL, f, 19); err == nil {
		t.Error("Spool over the cap succeeded")
	}

	// A server that ignores Range would send the whole file; that must
	// fail at once, not be read or retried.
	ignoreRange.Store(true)
	if _, err := c.Range(ctx, srv.URL, 3, 4); err == nil || !isPermanent(err) {
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

// stallServer serves body, except that the first stalls requests send half
// of it and then hold the connection open with nothing more, as the
// stalled download in qa/chaos r07 did. headers sets a stall before the
// response headers instead.
func stallServer(t *testing.T, body []byte, stalls int, headers bool) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var hits atomic.Int32
	quit := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if int(hits.Add(1)) > stalls {
			_, _ = w.Write(body)
			return
		}
		if !headers {
			_, _ = w.Write(body[:len(body)/2])
			w.(http.Flusher).Flush()
		}
		select {
		case <-r.Context().Done():
		case <-quit:
		}
	}))
	t.Cleanup(func() { close(quit); srv.Close() })
	return srv, &hits
}

// An attempt that receives nothing for Idle is cut and retried, whether
// it stalls before the headers or in the body; it used to hang until the
// caller's own timeout. Progress hears the bytes, and starts over.
func TestClientIdleCut(t *testing.T) {
	t.Parallel()
	body := bytes.Repeat([]byte("k"), 1<<16)
	for _, headers := range []bool{false, true} {
		srv, hits := stallServer(t, body, 1, headers)
		c := fast(srv, 2)
		c.Idle = 100 * time.Millisecond
		var mu sync.Mutex
		var seen []int64
		start := time.Now()
		var got []byte
		err := c.Do(context.Background(), Request{
			URL: srv.URL,
			Read: func(resp *http.Response) error {
				var buf bytes.Buffer
				_, err := buf.ReadFrom(resp.Body)
				got = buf.Bytes()
				return err
			},
			Progress: func(n int64) { mu.Lock(); seen = append(seen, n); mu.Unlock() },
		})
		if err != nil || !bytes.Equal(got, body) {
			t.Fatalf("headers=%v: Do = %v, %d bytes", headers, err, len(got))
		}
		if d := time.Since(start); d > 5*time.Second {
			t.Errorf("headers=%v: took %v", headers, d)
		}
		if n := hits.Load(); n != 2 {
			t.Errorf("headers=%v: requests = %d, want 2 (one stall, one retry)", headers, n)
		}
		if len(seen) == 0 || seen[len(seen)-1] != int64(len(body)) {
			t.Errorf("headers=%v: progress ended at %v", headers, seen)
		}
	}
}

// A download that stalls on every try fails, saying so, once the retries
// are spent.
func TestClientIdleGivesUp(t *testing.T) {
	t.Parallel()
	srv, hits := stallServer(t, []byte("data"), 100, false)
	c := fast(srv, 2)
	c.Idle = 50 * time.Millisecond
	_, _, err := c.Bytes(context.Background(), srv.URL, 100)
	if !errors.Is(err, ErrStalled) {
		t.Fatalf("Bytes = %v, want ErrStalled", err)
	}
	if n := hits.Load(); n != 3 {
		t.Errorf("requests = %d, want 3", n)
	}
}

// A slow reader is not a stall: the timer only counts time the network
// sends nothing, and a Read that finished is never undone by it.
func TestClientIdleAfterRead(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()
	c := fast(srv, 0)
	c.Idle = 20 * time.Millisecond
	err := c.Do(context.Background(), Request{URL: srv.URL, Read: func(resp *http.Response) error {
		var buf bytes.Buffer
		_, err := buf.ReadFrom(resp.Body)
		time.Sleep(80 * time.Millisecond) // the timer fires here
		return err
	}})
	if err != nil {
		t.Errorf("Do = %v", err)
	}
}

func TestWriteFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	p := filepath.Join(dir, "sub", "index.json")
	for _, s := range []string{"first", "second"} {
		if err := WriteFile(p, []byte(s)); err != nil {
			t.Fatal(err)
		}
		if got, _ := os.ReadFile(p); string(got) != s {
			t.Errorf("read %q, want %q", got, s)
		}
	}
	if left, _ := filepath.Glob(filepath.Join(dir, "sub", TempPrefix+"*")); len(left) > 0 {
		t.Errorf("temp files left: %v", left)
	}
	// A write that cannot rename leaves the old file and no temp file.
	if err := os.Mkdir(filepath.Join(dir, "sub", "dir"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "sub", "dir", "x"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := WriteFile(filepath.Join(dir, "sub", "dir"), []byte("x")); err == nil {
		t.Error("rename over a non-empty dir succeeded")
	}
	if left, _ := filepath.Glob(filepath.Join(dir, "sub", TempPrefix+"*")); len(left) > 0 {
		t.Errorf("temp files left after a failed write: %v", left)
	}
}

func TestCleanDebris(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	old := time.Now().Add(-DebrisAge - time.Hour)
	for _, name := range []string{".dl-1", ".tmp-1899C-2", ".tmp-text.bin-3", "4A", ".dl-new", ".tmp-new", ".other-1"} {
		p := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Join(p, "tests"), 0o755); err != nil {
			t.Fatal(err)
		}
		if !strings.HasSuffix(name, "-new") {
			if err := os.Chtimes(p, old, old); err != nil {
				t.Fatal(err)
			}
		}
	}
	CleanDebris(root, TempPrefix, ".dl-")
	CleanDebris(filepath.Join(root, "missing")) // no root, nothing to do
	des, _ := os.ReadDir(root)
	var left []string
	for _, d := range des {
		left = append(left, d.Name())
	}
	if want := []string{".dl-new", ".other-1", ".tmp-new", "4A"}; !slices.Equal(left, want) {
		t.Errorf("left %v, want %v", left, want)
	}
}
