// Package codeforces serves Codeforces problems from the Hugging Face
// dataset open-r1/codeforces as a catalog.Catalog: about 10k problems with
// Codeforces ratings, statements, editorials (60%) and test data, judged
// locally like every other source.
//
// Build downloads the problem table once, only the columns it needs (17 MB
// of the 2.7 GB; see remote.go), and caches an index plus the statement
// text, so Cached, All and Get never touch the network and a Fetch needs
// it only for tests:
//
//	<cache>/katharsis/codeforces/
//	  index.json, text.bin                problem table; statement text
//	  <contest><index>/statement.md       e.g. 1899C/
//	  <contest><index>/editorial.md       when the dataset has one
//	  <contest><index>/samples/N.in, N.out
//	  <contest><index>/tests/N.in, N.out  official tests, then generated
//	  <contest><index>/meta.json          written last; marks the dir done
//
// Tests come from two places. A problem whose official tests are all in
// the dataset (19% of the judgeable ones) gets them from its
// verifiable_tests file, a few ranged reads. Every other judgeable
// problem has generated tests, stored one file per contest (median 32 MB,
// up to 1.75 GB); Fetch downloads the contest's file, keeps this
// problem's rows and deletes the rest, and adds the official tests the
// dataset does have (inputs under 400 characters, Codeforces' truncation
// limit) in front of them.
package codeforces

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"katharsis/internal/catalog"
	"katharsis/internal/content/web"
)

type Catalog struct {
	root   string // <cache>/katharsis/codeforces
	base   string // site; a test server in tests
	lay    layout
	client *web.Client

	mu      sync.RWMutex // guards entries (a fetch can mark one unjudgeable)
	saveMu  sync.Mutex
	idx     index // Key and text.bin's size and hash, for persist
	entries map[catalog.ID]*entry
	order   []catalog.ID // contest, then problem index

	flights web.Flights[catalog.ID, catalog.Assets]
}

var _ catalog.Catalog = (*Catalog)(nil)

const site = "https://huggingface.co"

// client is the process's one client for Hugging Face, which rate-limits
// anonymous resolves (3000 per 5 minutes when written).
var client = &web.Client{
	HTTP:    &http.Client{Timeout: 15 * time.Minute},
	Gap:     100 * time.Millisecond,
	Retries: 4,
	Backoff: time.Second,
}

// ErrNoIndex means the cache holds no index this build can read: none was
// built yet, or another revision, schema or layout wrote it. Build makes
// one.
var ErrNoIndex = errors.New("no Codeforces index cached for this build")

// Cached opens the catalog from the cached index alone: about 0.1 s and no
// network, or ErrNoIndex. The game starts from it, so a cold or stale
// cache never holds up the start.
func Cached() (*Catalog, error) {
	root, err := cacheRoot()
	if err != nil {
		return nil, err
	}
	return cached(root, site, pinned, client)
}

// Build downloads the problem table, caches the index and opens it: about
// 17 MB in 38 requests, 20 s when measured. progress, when set, hears how
// many of the build's steps are done as each finishes. Afterwards All and
// Get need no network.
//
// A failed build tries once more after RetryWait, so a game that started
// offline picks Codeforces up later in the session rather than at the next
// start. offline, when set, hears the first failure as the wait begins.
func Build(ctx context.Context, progress func(done, total int), offline func(error)) (*Catalog, error) {
	root, err := cacheRoot()
	if err != nil {
		return nil, err
	}
	return retryOnce(ctx, RetryWait, offline, func() (*Catalog, error) {
		return build(ctx, root, site, pinned, client, progress)
	})
}

// RetryWait is how long Build waits to try again: long enough for a
// network to come back, and only once, so a site that is down is not
// asked all session.
const RetryWait = 10 * time.Minute

// retryOnce runs try, and if it fails for any reason but ctx ending, tells
// offline and runs it once more after wait.
func retryOnce(ctx context.Context, wait time.Duration, offline func(error), try func() (*Catalog, error)) (*Catalog, error) {
	c, err := try()
	if err == nil || ctx.Err() != nil {
		return c, err
	}
	if offline != nil {
		offline(err)
	}
	t := time.NewTimer(wait)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-t.C:
	}
	return try()
}

// Open is Cached, else Build, for a caller that would rather wait than do
// without (the content QA, the network test).
func Open(ctx context.Context) (*Catalog, error) {
	root, err := cacheRoot()
	if err != nil {
		return nil, err
	}
	return open(ctx, root, site, pinned, client)
}

func cacheRoot() (string, error) {
	c, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(c, "katharsis", "codeforces"), nil
}

func open(ctx context.Context, root, base string, l layout, pc *web.Client) (*Catalog, error) {
	c, err := cached(root, base, l, pc)
	if errors.Is(err, ErrNoIndex) {
		return build(ctx, root, base, l, pc, nil)
	}
	return c, err
}

func cached(root, base string, l layout, pc *web.Client) (*Catalog, error) {
	c := newCatalog(root, base, l, pc)
	idx, ok := loadIndex(root, l)
	if !ok {
		return nil, ErrNoIndex
	}
	c.load(idx)
	return c, nil
}

func build(ctx context.Context, root, base string, l layout, pc *web.Client, progress func(done, total int)) (*Catalog, error) {
	if progress == nil {
		progress = func(int, int) {}
	}
	c := newCatalog(root, base, l, pc)
	idx, err := c.build(ctx, progress)
	if err != nil {
		return nil, err
	}
	c.load(idx)
	return c, nil
}

func newCatalog(root, base string, l layout, pc *web.Client) *Catalog {
	// A contest download (.dl-) can be hundreds of MB.
	web.CleanDebris(root, web.TempPrefix, ".dl-")
	return &Catalog{
		root: root, base: base, lay: l, client: pc,
		entries: map[catalog.ID]*entry{},
	}
}

func (c *Catalog) load(idx *index) {
	c.idx = *idx
	c.idx.Entries = nil
	for i := range idx.Entries {
		e := &idx.Entries[i]
		c.entries[e.ID] = e
		c.order = append(c.order, e.ID)
	}
}

// build downloads the generated-tests listing and the table shards and
// writes the cache. Each of those is a step for progress, and so is the
// write.
func (c *Catalog) build(ctx context.Context, progress func(done, total int)) (*index, error) {
	total := len(c.lay.Shards) + 2
	progress(0, total)
	gen, err := c.genSizes(ctx)
	if err != nil {
		return nil, err
	}
	progress(1, total)
	// A split's rows in shard order are its table order, which the
	// verifiable_tests positions are counted in.
	var splits []split
	for n, s := range c.lay.Shards {
		rows, err := c.readShard(ctx, s)
		if err != nil {
			return nil, err
		}
		i := slices.IndexFunc(splits, func(sp split) bool { return sp.name == s.Split })
		if i < 0 {
			i = len(splits)
			splits = append(splits, split{name: s.Split})
		}
		splits[i].rows = append(splits[i].rows, rows...)
		progress(n+2, total)
	}
	idx, blob, err := buildIndex(c.lay, splits, gen)
	if err != nil {
		return nil, err
	}
	if err := saveIndex(c.root, idx, blob); err != nil {
		return nil, err
	}
	progress(total, total)
	return idx, nil
}

// All lists every problem in contest order, judgeable or not.
func (c *Catalog) All() []catalog.Problem {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([]catalog.Problem, 0, len(c.order))
	for _, id := range c.order {
		out = append(out, c.entries[id].problem())
	}
	return out
}

func (c *Catalog) Get(id catalog.ID) (catalog.Problem, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	e, ok := c.entries[id]
	if !ok {
		return catalog.Problem{}, false
	}
	return e.problem(), true
}

// problem copies the entry so callers cannot alias the shared Tags slice.
func (e *entry) problem() catalog.Problem {
	p := e.Problem
	p.Tags = append([]string(nil), p.Tags...)
	return p
}

func (c *Catalog) entry(id catalog.ID) (entry, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	e, ok := c.entries[id]
	if !ok {
		return entry{}, false
	}
	return *e, true
}

// markMissing takes id out of door play after a fetch found its tests
// absent from the files the index pointed at, and persists that, so the
// engine stops offering it.
func (c *Catalog) markMissing(id catalog.ID) error {
	c.mu.Lock()
	e, ok := c.entries[id]
	if !ok || e.Skip == skipTestsMissing {
		c.mu.Unlock()
		return nil
	}
	e.Skip, e.Judgeable = skipTestsMissing, false
	c.mu.Unlock()
	return c.persist()
}

// persist rewrites index.json from memory; text.bin does not change.
// saveMu orders the writers and each snapshots under it, so the last
// rename carries every update so far.
func (c *Catalog) persist() error {
	c.saveMu.Lock()
	defer c.saveMu.Unlock()
	c.mu.RLock()
	idx := c.idx
	idx.Entries = make([]entry, 0, len(c.order))
	for _, id := range c.order {
		idx.Entries = append(idx.Entries, *c.entries[id])
	}
	c.mu.RUnlock()
	b, err := json.Marshal(&idx)
	if err != nil {
		return err
	}
	return web.WriteFile(filepath.Join(c.root, "index.json"), b)
}

// Fetch materializes id under the cache and returns its asset paths. A
// finished problem dir is reused without the network. Concurrent calls for
// one id share a single download; calls for different ids proceed in
// parallel but still go through the per-host limiter. Tests are fetched
// only for judgeable problems; the others get statement, samples and
// editorial.
func (c *Catalog) Fetch(ctx context.Context, id catalog.ID) (catalog.Assets, error) {
	e, ok := c.entry(id)
	if !ok {
		return catalog.Assets{}, errors.New(string(id) + ": not a Codeforces problem in the catalog")
	}
	dir := filepath.Join(c.root, dirName(e))
	// The cache is checked inside the flight, so a download of the same
	// problem that finishes a moment earlier cannot be missed and started
	// again.
	return c.flights.Do(ctx, id, func() (catalog.Assets, error) {
		if a, ok := loadAssets(dir); usable(a, ok, e) {
			return a, nil
		}
		return c.materialize(ctx, e, dir)
	})
}

// dirName is "1899C": the id without "cf:". newEntry admits only digits
// and an index of capital letters and digits, so it is a safe path.
func dirName(e entry) string { return e.ID.Native() }
