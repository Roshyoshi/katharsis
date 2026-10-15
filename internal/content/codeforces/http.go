package codeforces

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

const userAgent = "katharsis/0.1 (terminal competitive-programming trainer; " +
	"fetches each problem once and caches it)"

// errNotFound is a 404: the file does not exist at the pinned revision, so
// it is reported as absent instead of retried.
var errNotFound = errors.New("not found")

// politeClient serializes requests per host and spaces them out. Hugging
// Face rate-limits unauthenticated resolves (3000 per 5 minutes at the time
// of writing) and a door prefetch must never look like a crawl. It is
// process-wide (see client) so two Catalogs cannot double the rate. It is
// the USACO source's client plus byte ranges; both could move to a shared
// package once a third source needs one.
type politeClient struct {
	http    *http.Client
	gap     time.Duration // between the end of one request and the next
	retries int
	backoff time.Duration // first retry delay; doubles each time

	mu    sync.Mutex
	hosts map[string]*hostSlot
}

// hostSlot is a one-token semaphore plus the time the host was last used.
type hostSlot struct {
	sem  chan struct{}
	last time.Time
}

var client = &politeClient{
	http:    &http.Client{Timeout: 15 * time.Minute},
	gap:     100 * time.Millisecond,
	retries: 4,
	backoff: time.Second,
}

func (c *politeClient) slot(host string) *hostSlot {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.hosts == nil {
		c.hosts = map[string]*hostSlot{}
	}
	s := c.hosts[host]
	if s == nil {
		s = &hostSlot{sem: make(chan struct{}, 1)}
		c.hosts[host] = s
	}
	return s
}

// permanentError wraps failures a retry cannot fix: a 4xx other than 429,
// a server that ignored a Range header, or a body that read rejected.
type permanentError struct{ error }

func (p permanentError) Unwrap() error { return p.error }

// request is one GET. rng is a Range header value ("" for the whole body);
// a ranged request must come back 206 or it fails permanently, since a 200
// would be the whole file (hundreds of MB) when a few KB were wanted.
type request struct {
	url  string
	rng  string
	read func(*http.Response) error
}

// do runs r with the host slot held for the whole body, so a large
// download counts as one in-flight request. 5xx, 429 and transport errors
// are retried with exponential backoff, honoring Retry-After; a 404
// returns errNotFound at once.
func (c *politeClient) do(ctx context.Context, r request) error {
	u, err := url.Parse(r.url)
	if err != nil {
		return err
	}
	s := c.slot(u.Host)
	delay := c.backoff
	for attempt := 0; ; attempt++ {
		retryAfter, err := c.once(ctx, s, r)
		if err == nil || errors.Is(err, errNotFound) || ctx.Err() != nil {
			return err
		}
		var perm permanentError
		if errors.As(err, &perm) || attempt >= c.retries {
			return err
		}
		wait := delay + rand.N(delay/2+1)
		if retryAfter > wait {
			wait = min(retryAfter, 2*time.Minute)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(wait):
		}
		delay *= 2
	}
}

func (c *politeClient) once(ctx context.Context, s *hostSlot, r request) (time.Duration, error) {
	select {
	case s.sem <- struct{}{}:
	case <-ctx.Done():
		return 0, ctx.Err()
	}
	defer func() {
		s.last = time.Now()
		<-s.sem
	}()
	if wait := time.Until(s.last.Add(c.gap)); wait > 0 {
		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		case <-time.After(wait):
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, r.url, nil)
	if err != nil {
		return 0, permanentError{err}
	}
	req.Header.Set("User-Agent", userAgent)
	if r.rng != "" {
		req.Header.Set("Range", r.rng)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	want := http.StatusOK
	if r.rng != "" {
		want = http.StatusPartialContent
	}
	switch {
	case resp.StatusCode == want:
	case resp.StatusCode == http.StatusNotFound:
		return 0, fmt.Errorf("GET %s: %w", r.url, errNotFound)
	case resp.StatusCode >= 500 || resp.StatusCode == http.StatusTooManyRequests:
		return retryAfter(resp.Header.Get("Retry-After")), fmt.Errorf("GET %s: %s", r.url, resp.Status)
	default:
		return 0, permanentError{fmt.Errorf("GET %s (range %q): %s", r.url, r.rng, resp.Status)}
	}
	if err := r.read(resp); err != nil {
		// A read cut short by the network is worth retrying; a body the
		// reader refused is not.
		var perm permanentError
		if errors.As(err, &perm) {
			return 0, err
		}
		return 0, fmt.Errorf("GET %s: %w", r.url, err)
	}
	return 0, nil
}

func retryAfter(v string) time.Duration {
	if v == "" {
		return 0
	}
	if secs, err := strconv.Atoi(v); err == nil {
		return time.Duration(secs) * time.Second
	}
	if t, err := http.ParseTime(v); err == nil {
		return time.Until(t)
	}
	return 0
}

// getBytes reads a whole body of at most limit bytes.
func (c *politeClient) getBytes(ctx context.Context, rawURL string, limit int64) ([]byte, http.Header, error) {
	var out []byte
	var hdr http.Header
	err := c.do(ctx, request{url: rawURL, read: func(resp *http.Response) error {
		b, err := readCapped(resp.Body, limit, rawURL)
		out, hdr = b, resp.Header
		return err
	}})
	return out, hdr, err
}

// getRange reads bytes [off, off+n) of a remote file.
func (c *politeClient) getRange(ctx context.Context, rawURL string, off, n int64) ([]byte, error) {
	var out []byte
	rng := fmt.Sprintf("bytes=%d-%d", off, off+n-1)
	err := c.do(ctx, request{url: rawURL, rng: rng, read: func(resp *http.Response) error {
		start, _, _, err := contentRange(resp.Header.Get("Content-Range"))
		if err != nil || start != off {
			return permanentError{fmt.Errorf("%s: asked for %s, got Content-Range %q", rawURL, rng, resp.Header.Get("Content-Range"))}
		}
		b, err := readCapped(resp.Body, n, rawURL)
		if err != nil {
			return err
		}
		if int64(len(b)) != n {
			return fmt.Errorf("%s: short range read, %d of %d bytes", rawURL, len(b), n)
		}
		out = b
		return nil
	}})
	return out, err
}

// getTail reads the last n bytes of a remote file (fewer if it is smaller)
// and reports the file's total size, which a parquet footer read needs and
// which saves asking for it separately.
func (c *politeClient) getTail(ctx context.Context, rawURL string, n int64) ([]byte, int64, error) {
	var out []byte
	var size int64
	rng := fmt.Sprintf("bytes=-%d", n)
	err := c.do(ctx, request{url: rawURL, rng: rng, read: func(resp *http.Response) error {
		start, end, total, err := contentRange(resp.Header.Get("Content-Range"))
		if err != nil || total <= 0 || end != total-1 {
			return permanentError{fmt.Errorf("%s: asked for %s, got Content-Range %q", rawURL, rng, resp.Header.Get("Content-Range"))}
		}
		want := end - start + 1
		b, err := readCapped(resp.Body, want, rawURL)
		if err != nil {
			return err
		}
		if int64(len(b)) != want {
			return fmt.Errorf("%s: short range read, %d of %d bytes", rawURL, len(b), want)
		}
		out, size = b, total
		return nil
	}})
	return out, size, err
}

// download streams a whole body into w, refusing more than limit bytes. On
// a retry reset is called first so a cut connection starts the copy over.
func (c *politeClient) download(ctx context.Context, rawURL string, limit int64, reset func() (io.Writer, error)) error {
	return c.do(ctx, request{url: rawURL, read: func(resp *http.Response) error {
		w, err := reset()
		if err != nil {
			return permanentError{err}
		}
		if resp.ContentLength > limit {
			return permanentError{fmt.Errorf("%s is %d bytes, over the %d cap", rawURL, resp.ContentLength, limit)}
		}
		n, err := io.Copy(w, io.LimitReader(resp.Body, limit+1))
		if err != nil {
			return err
		}
		if n > limit {
			return permanentError{fmt.Errorf("%s: body over %d bytes", rawURL, limit)}
		}
		return nil
	}})
}

func readCapped(r io.Reader, limit int64, what string) ([]byte, error) {
	b, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(b)) > limit {
		return nil, permanentError{fmt.Errorf("%s: body over %d bytes", what, limit)}
	}
	return b, nil
}

// contentRange parses "bytes 100-199/1000".
func contentRange(v string) (start, end, total int64, err error) {
	rest, ok := strings.CutPrefix(v, "bytes ")
	if !ok {
		return 0, 0, 0, fmt.Errorf("Content-Range %q", v)
	}
	span, tot, ok := strings.Cut(rest, "/")
	a, b, ok2 := strings.Cut(span, "-")
	if !ok || !ok2 {
		return 0, 0, 0, fmt.Errorf("Content-Range %q", v)
	}
	if start, err = strconv.ParseInt(a, 10, 64); err != nil {
		return
	}
	if end, err = strconv.ParseInt(b, 10, 64); err != nil {
		return
	}
	if tot == "*" {
		return start, end, -1, nil
	}
	total, err = strconv.ParseInt(tot, 10, 64)
	return
}
