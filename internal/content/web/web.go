// Package web is how the content sources reach the network and write their
// caches. Client is the one HTTP client they share: usaco.org is run by
// volunteers and Hugging Face rate-limits anonymous reads, so prefetching
// doors must never look like a crawl, and a connection that stays open and
// sends nothing must fail instead of holding up the game. Flights shares
// one download among the callers that want it. WriteFile, SyncDir and
// CleanDebris keep a cache readable when the process is killed or the
// power cut.
package web

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

// UserAgent names the program and its version, and nothing about the
// player.
const UserAgent = "katharsis/0.1"

var (
	// ErrNotFound is a 404: the resource does not exist, so it is not
	// retried. Sources cache it as absent (a USACO problem with no test
	// zip) or take the problem out of play.
	ErrNotFound = errors.New("not found")
	// ErrStalled is an attempt cut by the idle rule (Client.Idle).
	ErrStalled = errors.New("download stalled")
)

// DefaultIdle is the idle cut when Client.Idle is zero: long enough for a
// slow server to start answering, short against the minutes a caller waits.
const DefaultIdle = 30 * time.Second

// maxRetryAfter caps how long a server's Retry-After can hold a retry.
const maxRetryAfter = 2 * time.Minute

// Client sends one request at a time per host, leaves Gap between them,
// and retries 5xx, 429, transport errors and stalls with exponential
// backoff, honoring Retry-After. A 404 or another 4xx fails at once, as
// does a body Request.Read refuses with Permanent. Sources keep one Client
// for the whole process, so two catalogs cannot double the rate.
type Client struct {
	// HTTP sends the requests; nil is http.DefaultClient. Its Timeout
	// bounds one attempt, the body included.
	HTTP *http.Client
	// Gap runs from the end of one request to a host to the start of the
	// next.
	Gap time.Duration
	// Retries is the attempts after the first. The first retry waits
	// Backoff plus up to half again, and each later one twice the last.
	Retries int
	Backoff time.Duration
	// Idle cuts an attempt that receives no bytes for this long, headers
	// or body, and retries it; zero is DefaultIdle.
	Idle time.Duration

	mu    sync.Mutex
	hosts map[string]*hostSlot
}

// hostSlot is a one-token semaphore plus the time the host was last used.
type hostSlot struct {
	sem  chan struct{}
	last time.Time
}

// Request is one HTTP request.
type Request struct {
	Method string // "" is GET
	URL    string
	// Range is a Range header value, "" for the whole body. A ranged
	// request must come back 206: a 200 would be the whole file, which
	// can be gigabytes where a few KB were wanted.
	Range string
	// Read consumes the response once per attempt, with the host slot
	// held, so a large download counts as one request in flight. An error
	// it returns is retried unless wrapped with Permanent. Nil discards
	// the body.
	Read func(*http.Response) error
	// Progress, when set, is told the bytes of body received so far. A
	// retry starts the count over.
	Progress func(received int64)
}

type progressKey struct{}

// WithProgress returns a context under which every request tells fn the
// body bytes it receives, as they arrive: a caller that cannot reach the
// Requests a source makes (a door's Fetch) can still show a download
// moving. fn adds up what it is told, retries included; it may be called
// from several goroutines at once.
func WithProgress(ctx context.Context, fn func(n int64)) context.Context {
	return context.WithValue(ctx, progressKey{}, fn)
}

func progressFrom(ctx context.Context) func(int64) {
	fn, _ := ctx.Value(progressKey{}).(func(int64))
	return fn
}

// Permanent marks an error a retry cannot fix, such as a body that is too
// big or malformed, so the Client gives up at once.
func Permanent(err error) error { return permanentError{err} }

type permanentError struct{ error }

func (p permanentError) Unwrap() error { return p.error }

func isPermanent(err error) bool { return errors.As(err, new(permanentError)) }

// Do runs r, retrying as Client says.
func (c *Client) Do(ctx context.Context, r Request) error {
	u, err := url.Parse(r.URL)
	if err != nil {
		return err
	}
	s := c.slot(u.Host)
	delay := c.Backoff
	for attempt := 0; ; attempt++ {
		retryAfter, err := c.once(ctx, s, r)
		if err == nil || errors.Is(err, ErrNotFound) || ctx.Err() != nil {
			return err
		}
		if isPermanent(err) || attempt >= c.Retries {
			return err
		}
		wait := delay + rand.N(delay/2+1)
		if retryAfter > wait {
			wait = min(retryAfter, maxRetryAfter)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(wait):
		}
		delay *= 2
	}
}

func (c *Client) slot(host string) *hostSlot {
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

// once is one attempt. It returns the server's Retry-After for a 5xx or 429.
func (c *Client) once(ctx context.Context, s *hostSlot, r Request) (retryAfter time.Duration, err error) {
	select {
	case s.sem <- struct{}{}:
	case <-ctx.Done():
		return 0, ctx.Err()
	}
	defer func() {
		s.last = time.Now()
		<-s.sem
	}()
	if wait := time.Until(s.last.Add(c.Gap)); wait > 0 {
		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		case <-time.After(wait):
		}
	}

	method := r.Method
	if method == "" {
		method = http.MethodGet
	}
	what := method + " " + r.URL // for errors
	if r.Range != "" {
		what += " (range " + r.Range + ")"
	}
	// The idle timer runs from the request on and restarts with every read
	// that brings bytes, so it bounds a stall before the headers too. A
	// cut only counts when the attempt failed: a timer that fires after
	// Read is done changes nothing.
	idle := c.Idle
	if idle <= 0 {
		idle = DefaultIdle
	}
	actx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	timer := time.AfterFunc(idle, func() { cancel(ErrStalled) })
	defer timer.Stop()
	defer func() {
		if err != nil && ctx.Err() == nil && errors.Is(context.Cause(actx), ErrStalled) {
			retryAfter, err = 0, fmt.Errorf("%s: %w: no data for %v", what, ErrStalled, idle)
		}
	}()

	req, err := http.NewRequestWithContext(actx, method, r.URL, nil)
	if err != nil {
		return 0, Permanent(err)
	}
	req.Header.Set("User-Agent", UserAgent)
	if r.Range != "" {
		req.Header.Set("Range", r.Range)
	}
	hc := c.HTTP
	if hc == nil {
		hc = http.DefaultClient
	}
	resp, err := hc.Do(req)
	if err != nil {
		return 0, err
	}
	defer func() { _ = resp.Body.Close() }()
	want := http.StatusOK
	if r.Range != "" {
		want = http.StatusPartialContent
	}
	switch {
	case resp.StatusCode == want:
	case resp.StatusCode == http.StatusNotFound:
		return 0, fmt.Errorf("%s: %w", what, ErrNotFound)
	case resp.StatusCode >= 500 || resp.StatusCode == http.StatusTooManyRequests:
		return parseRetryAfter(resp.Header.Get("Retry-After")), fmt.Errorf("%s: %s", what, resp.Status)
	default:
		return 0, Permanent(fmt.Errorf("%s: %s", what, resp.Status))
	}
	if r.Read == nil {
		return 0, nil
	}
	resp.Body = &idleBody{ReadCloser: resp.Body, timer: timer, idle: idle, progress: r.Progress, sink: progressFrom(ctx)}
	if err := r.Read(resp); err != nil {
		// A read cut short by the network is worth retrying; a body the
		// reader refused is not.
		if isPermanent(err) {
			return 0, err
		}
		return 0, fmt.Errorf("%s: %w", what, err)
	}
	return 0, nil
}

// idleBody restarts the idle timer on every read that brings bytes and
// reports the running total to progress.
type idleBody struct {
	io.ReadCloser
	timer    *time.Timer
	idle     time.Duration
	progress func(int64)
	sink     func(int64) // the context's (WithProgress): each read's bytes
	n        int64
}

func (b *idleBody) Read(p []byte) (int, error) {
	n, err := b.ReadCloser.Read(p)
	if n > 0 {
		b.timer.Reset(b.idle)
		b.n += int64(n)
		if b.progress != nil {
			b.progress(b.n)
		}
		if b.sink != nil {
			b.sink(int64(n))
		}
	}
	return n, err
}

func parseRetryAfter(v string) time.Duration {
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

// ---- reads -------------------------------------------------------------------

// Bytes reads a whole body of at most limit bytes, and its headers.
func (c *Client) Bytes(ctx context.Context, rawURL string, limit int64) ([]byte, http.Header, error) {
	var out []byte
	var hdr http.Header
	err := c.Do(ctx, Request{URL: rawURL, Read: func(resp *http.Response) error {
		b, err := readCapped(resp.Body, limit, rawURL)
		out, hdr = b, resp.Header
		return err
	}})
	return out, hdr, err
}

// Range reads bytes [off, off+n) of a remote file.
func (c *Client) Range(ctx context.Context, rawURL string, off, n int64) ([]byte, error) {
	var out []byte
	rng := fmt.Sprintf("bytes=%d-%d", off, off+n-1)
	err := c.Do(ctx, Request{URL: rawURL, Range: rng, Read: func(resp *http.Response) error {
		start, _, _, err := contentRange(resp.Header.Get("Content-Range"))
		if err != nil || start != off {
			return Permanent(fmt.Errorf("%s: asked for %s, got Content-Range %q", rawURL, rng, resp.Header.Get("Content-Range")))
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

// Tail reads the last n bytes of a remote file (fewer if it is smaller) and
// reports the file's total size, which saves asking for it separately.
func (c *Client) Tail(ctx context.Context, rawURL string, n int64) ([]byte, int64, error) {
	var out []byte
	var size int64
	rng := fmt.Sprintf("bytes=-%d", n)
	err := c.Do(ctx, Request{URL: rawURL, Range: rng, Read: func(resp *http.Response) error {
		start, end, total, err := contentRange(resp.Header.Get("Content-Range"))
		if err != nil || total <= 0 || end != total-1 {
			return Permanent(fmt.Errorf("%s: asked for %s, got Content-Range %q", rawURL, rng, resp.Header.Get("Content-Range")))
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

// Spool downloads a whole body of at most limit bytes into f and returns
// its size. Every attempt truncates f first, so a retry after a cut
// connection starts the copy over.
func (c *Client) Spool(ctx context.Context, rawURL string, f *os.File, limit int64) (int64, error) {
	var size int64
	err := c.Do(ctx, Request{URL: rawURL, Read: func(resp *http.Response) error {
		if resp.ContentLength > limit {
			return Permanent(fmt.Errorf("%s is %d bytes, over the %d cap", rawURL, resp.ContentLength, limit))
		}
		if _, err := f.Seek(0, io.SeekStart); err != nil {
			return Permanent(err)
		}
		if err := f.Truncate(0); err != nil {
			return Permanent(err)
		}
		n, err := io.Copy(f, io.LimitReader(resp.Body, limit+1))
		if err != nil {
			return err
		}
		if n > limit {
			return Permanent(fmt.Errorf("%s: body over %d bytes", rawURL, limit))
		}
		size = n
		return nil
	}})
	return size, err
}

func readCapped(r io.Reader, limit int64, what string) ([]byte, error) {
	b, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(b)) > limit {
		return nil, Permanent(fmt.Errorf("%s: body over %d bytes", what, limit))
	}
	return b, nil
}

// contentRange parses "bytes 100-199/1000"; total is -1 for "/*".
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
