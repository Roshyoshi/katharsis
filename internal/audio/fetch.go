package audio

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// userAgent names the game and nothing about who runs it.
const userAgent = "katharsis/0.1 (terminal game; downloads each music file once and caches it)"

// FetchOptions controls Fetch.
type FetchOptions struct {
	Out    io.Writer    // progress, one line per step; nil is quiet
	Client *http.Client // nil is a client with a 10 min overall timeout
	// Only limits the fetch to tracks whose ID or pools are listed.
	Only []string
	// Retries is how many times a failed download is retried.
	Retries int
	// Gap is the pause between requests, so a host serving several files
	// sees them one at a time and spaced out.
	Gap time.Duration
}

// FetchResult counts what Fetch did.
type FetchResult struct {
	Fetched, Had, Failed int
	Bytes                int64
	Errors               []error
}

// Fetch downloads the library's manifest tracks that are missing into
// its cache. Each file streams into a temp file beside its final name,
// hashed as it is written; only a file of the listed size and SHA-256 is
// renamed into place, so the cache never holds a partial or altered file.
// It then measures each new file's loudness. One track failing does not
// stop the rest.
func Fetch(ctx context.Context, l *Library, o FetchOptions) FetchResult {
	if o.Client == nil {
		o.Client = &http.Client{Timeout: 10 * time.Minute}
	}
	if o.Out == nil {
		o.Out = io.Discard
	}
	var todo []Track
	for _, t := range l.Tracks {
		if len(o.Only) == 0 || slices.Contains(o.Only, t.ID) || slices.ContainsFunc(t.Pools, func(p string) bool { return slices.Contains(o.Only, p) }) {
			todo = append(todo, t)
		}
	}
	var r FetchResult
	if err := os.MkdirAll(l.CacheDir, 0o755); err != nil {
		r.Failed, r.Errors = len(todo), []error{err}
		return r
	}
	cleanParts(l.CacheDir)
	for i, t := range todo {
		t.Path = l.Path(t)
		tag := fmt.Sprintf("[%d/%d] %s", i+1, len(todo), t.ID)
		if l.Have(t) {
			r.Had++
			if _, ok := l.Loudness(t); !ok {
				measure(l, t, o.Out, tag)
			}
			fmt.Fprintf(o.Out, "%s: already here\n", tag)
			continue
		}
		if ctx.Err() != nil {
			r.Failed++
			r.Errors = append(r.Errors, fmt.Errorf("%s: %w", t.ID, ctx.Err()))
			continue
		}
		var err error
		for attempt := 0; attempt <= o.Retries; attempt++ {
			if attempt > 0 {
				fmt.Fprintf(o.Out, "%s: retrying (%v)\n", tag, err)
				if !sleep(ctx, time.Duration(attempt)*2*time.Second) {
					break
				}
			}
			if err = download(ctx, o, t, tag); err == nil || errors.Is(err, errPermanent) {
				break
			}
		}
		if i < len(todo)-1 {
			sleep(ctx, o.Gap)
		}
		if err != nil {
			r.Failed++
			r.Errors = append(r.Errors, fmt.Errorf("%s: %w", t.ID, err))
			fmt.Fprintf(o.Out, "%s: failed: %v\n", tag, err)
			continue
		}
		r.Fetched++
		r.Bytes += t.Size
		measure(l, t, o.Out, tag)
	}
	return r
}

func measure(l *Library, t Track, out io.Writer, tag string) {
	m, err := l.Measure(t)
	if err != nil {
		fmt.Fprintf(out, "%s: loudness not measured: %v\n", tag, err)
		return
	}
	fmt.Fprintf(out, "%s: %.1f LUFS, peak %.1f dBFS\n", tag, m.LUFS, DB(m.Peak))
}

func sleep(ctx context.Context, d time.Duration) bool {
	if d <= 0 {
		return ctx.Err() == nil
	}
	select {
	case <-ctx.Done():
		return false
	case <-time.After(d):
		return true
	}
}

// errPermanent marks a failure a retry will not fix: a 4xx, or a file
// that is not the one listed.
var errPermanent = errors.New("permanent")

type permanent struct{ error }

func (p permanent) Unwrap() []error { return []error{p.error, errPermanent} }

// partPrefix marks downloads in progress; cleanParts removes ones an
// interrupted fetch left behind.
const partPrefix = ".part-"

func cleanParts(dir string) {
	es, _ := os.ReadDir(dir)
	for _, e := range es {
		if strings.HasPrefix(e.Name(), partPrefix) {
			os.Remove(filepath.Join(dir, e.Name()))
		}
	}
}

func download(ctx context.Context, o FetchOptions, t Track, tag string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, t.URL, nil)
	if err != nil {
		return permanent{err}
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := o.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if final := resp.Request.URL; final.Scheme != "https" {
		return permanent{fmt.Errorf("redirected to %s, not https", redact(final))}
	}
	switch {
	case resp.StatusCode == http.StatusOK:
	case resp.StatusCode >= 500 || resp.StatusCode == http.StatusTooManyRequests:
		return fmt.Errorf("GET %s: %s", t.URL, resp.Status)
	default:
		return permanent{fmt.Errorf("GET %s: %s", t.URL, resp.Status)}
	}
	if resp.ContentLength >= 0 && resp.ContentLength != t.Size {
		return permanent{fmt.Errorf("the server offers %d bytes; the manifest lists %d", resp.ContentLength, t.Size)}
	}
	f, err := os.CreateTemp(filepath.Dir(t.Path), partPrefix+t.ID+"-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name()) // a no-op once renamed
	h := sha256.New()
	p := &progress{out: o.Out, tag: tag, total: t.Size, every: 500 * time.Millisecond}
	n, err := io.Copy(io.MultiWriter(f, h, p), io.LimitReader(resp.Body, t.Size+1))
	if err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if n != t.Size {
		return permanent{fmt.Errorf("got %d bytes, want %d", n, t.Size)}
	}
	if sum := hex.EncodeToString(h.Sum(nil)); sum != t.SHA256 {
		return permanent{fmt.Errorf("sha256 %s, want %s: the file changed at its source", sum, t.SHA256)}
	}
	if err := os.Rename(f.Name(), t.Path); err != nil {
		return err
	}
	p.print() // the last step, if the interval skipped it
	return nil
}

func redact(u *url.URL) string {
	c := *u
	c.User, c.RawQuery = nil, ""
	return c.String()
}

// progress prints "tag 1.2/4.5 MiB 27%" at most every interval, and once
// at the end.
type progress struct {
	out        io.Writer
	tag        string
	total, n   int64
	every      time.Duration
	last       time.Time
	lastPrintN int64
}

func (p *progress) Write(b []byte) (int, error) {
	p.n += int64(len(b))
	if now := time.Now(); now.Sub(p.last) >= p.every {
		p.last = now
		p.print()
	}
	return len(b), nil
}

func (p *progress) print() {
	if p.n == p.lastPrintN {
		return
	}
	p.lastPrintN = p.n
	fmt.Fprintf(p.out, "%s: %s/%s %3.0f%%\n", p.tag, mib(p.n), mib(p.total), 100*float64(p.n)/float64(max(p.total, 1)))
}

func mib(n int64) string { return fmt.Sprintf("%.1f MiB", float64(n)/(1<<20)) }
