package usaco

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"
)

const userAgent = "katharsis/0.1 (terminal competitive-programming trainer; " +
	"fetches each problem once and caches it)"

// errNotFound is a 404: the resource does not exist, so it is cached as
// absent instead of retried (interactive problems have no test zip).
var errNotFound = errors.New("not found")

// politeClient serializes requests per host and spaces them out. usaco.org
// is a volunteer-run site; prefetching doors must never look like a crawl.
// It is process-wide (see client) so two Catalogs cannot double the rate.
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
	http:    &http.Client{Timeout: 5 * time.Minute},
	gap:     500 * time.Millisecond,
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

// get fetches rawURL and hands the body to read while the host slot is
// still held, so a large zip download counts as one in-flight request. 5xx,
// 429 and transport errors are retried with exponential backoff (honoring
// Retry-After); a 404 returns errNotFound at once.
func (c *politeClient) get(ctx context.Context, rawURL string, read func(io.Reader) error) error {
	return c.do(ctx, http.MethodGet, rawURL, read)
}

// head checks that rawURL exists without downloading it (errNotFound if
// not); facts.json records which problems have test zips this way.
func (c *politeClient) head(ctx context.Context, rawURL string) error {
	return c.do(ctx, http.MethodHead, rawURL, func(io.Reader) error { return nil })
}

func (c *politeClient) do(ctx context.Context, method, rawURL string, read func(io.Reader) error) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return err
	}
	s := c.slot(u.Host)
	delay := c.backoff
	for attempt := 0; ; attempt++ {
		retryAfter, err := c.once(ctx, s, method, rawURL, read)
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

// permanentError wraps failures a retry cannot fix: a 4xx other than 429,
// or a body that read rejected (too big, malformed).
type permanentError struct{ error }

func (p permanentError) Unwrap() error { return p.error }

func (c *politeClient) once(ctx context.Context, s *hostSlot, method, rawURL string, read func(io.Reader) error) (time.Duration, error) {
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

	req, err := http.NewRequestWithContext(ctx, method, rawURL, nil)
	if err != nil {
		return 0, permanentError{err}
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := c.http.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode == http.StatusOK:
	case resp.StatusCode == http.StatusNotFound:
		return 0, fmt.Errorf("%s %s: %w", method, rawURL, errNotFound)
	case resp.StatusCode >= 500 || resp.StatusCode == http.StatusTooManyRequests:
		return retryAfter(resp.Header.Get("Retry-After")), fmt.Errorf("%s %s: %s", method, rawURL, resp.Status)
	default:
		return 0, permanentError{fmt.Errorf("%s %s: %s", method, rawURL, resp.Status)}
	}
	if err := read(resp.Body); err != nil {
		// A read cut short by the network is worth retrying; a body the
		// reader refused is not.
		var perm permanentError
		if errors.As(err, &perm) {
			return 0, err
		}
		return 0, fmt.Errorf("%s %s: %w", method, rawURL, err)
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
func (c *politeClient) getBytes(ctx context.Context, rawURL string, limit int64) ([]byte, error) {
	var out []byte
	err := c.get(ctx, rawURL, func(r io.Reader) error {
		b, err := io.ReadAll(io.LimitReader(r, limit+1))
		if err != nil {
			return err
		}
		if int64(len(b)) > limit {
			return permanentError{fmt.Errorf("%s: body over %d bytes", rawURL, limit)}
		}
		out = b
		return nil
	})
	return out, err
}
